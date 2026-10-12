// `pnpm dev` — hybrid local development.
//
// Infrastructure and Nginx run in Docker; the API, worker and Next.js run natively so edits
// apply immediately. Go sources and migrations are watched: on change the binaries are rebuilt
// (and migrations applied) and the API and worker restart. A failed build keeps the previous
// processes running. Open http://localhost:8080 — Nginx serves web and API on one origin.
//   --scan            also start ClamAV and scan uploads with it (MC_SCANNER=clamav)
//   --observability   also start Grafana LGTM and export traces to it

import { spawn } from 'node:child_process';
import { watch } from 'node:fs';
import path from 'node:path';
import readline from 'node:readline';
import {
  IS_WINDOWS,
  ROOT,
  SERVICES,
  binPath,
  buildGo,
  c,
  compose,
  killTree,
  log,
  main,
  pnpmCommand,
  prepareBackend,
  requireEnv,
  startInfra,
} from './lib.mjs';

const children = new Map();
let shuttingDown = false;

function prefixLines(stream, label) {
  readline.createInterface({ input: stream }).on('line', (line) => console.log(`${label} ${line}`));
}

function start(name, label, cmd, args, opts) {
  const child = spawn(cmd, args, {
    cwd: opts.cwd ?? ROOT,
    env: opts.env,
    shell: opts.shell ?? false,
    detached: !IS_WINDOWS,
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  prefixLines(child.stdout, label);
  prefixLines(child.stderr, label);
  child.on('exit', (code, signal) => {
    if (children.get(name) === child) children.delete(name);
    if (!shuttingDown && !child.restarting) {
      log.warn(`${name} exited (${signal ?? `code ${code}`}); it restarts on the next source change`);
    }
  });
  children.set(name, child);
  return child;
}

function stop(name) {
  return new Promise((resolve) => {
    const child = children.get(name);
    if (!child || child.exitCode !== null) return resolve();
    child.restarting = true;
    child.once('exit', () => resolve());
    killTree(child);
  });
}

const flags = new Set(process.argv.slice(2));

main(async () => {
  const env = requireEnv();
  const profiles = [];
  if (flags.has('--scan')) {
    profiles.push('scan');
    env.MC_SCANNER = 'clamav';
  }
  if (flags.has('--observability')) {
    profiles.push('observability');
    env.MC_OTEL_EXPORTER_OTLP_ENDPOINT = 'http://localhost:4318';
  }
  const apiEnv = { ...env, MC_HTTP_ADDR: '127.0.0.1:8081', MC_ADMIN_ADDR: '127.0.0.1:8091' };
  const workerEnv = { ...env, MC_ADMIN_ADDR: '127.0.0.1:8092' };

  // Containers from `pnpm dev:docker` would compete with the native processes (two workers
  // on one queue), so remove them; data volumes are untouched.
  await compose(['--profile', 'app', 'rm', '--stop', '--force', 'migrate', 'api', 'worker', 'web'], {
    env,
    stdio: 'ignore',
  }).catch(() => {});

  log.step(`Starting infrastructure${profiles.length ? ` (+ ${profiles.join(', ')})` : ''}`);
  await startInfra(profiles);

  log.step('Building Go services and applying migrations');
  await prepareBackend(env);
  await buildGo(['api', 'worker'], env);

  const startGo = () => {
    start('api', c.blue('[api]   '), binPath('api'), [], { env: apiEnv });
    start('worker', c.magenta('[worker]'), binPath('worker'), [], { env: workerEnv });
  };
  startGo();
  const web = pnpmCommand(['--filter', '@matecollab/web', 'dev']);
  start('web', c.green('[web]   '), web.cmd, web.args, {
    // Only the public, non-secret settings the web server needs (CSP origins).
    env: {
      ...process.env,
      NEXT_TELEMETRY_DISABLED: '1',
      MC_PUBLIC_URL: env.MC_PUBLIC_URL,
      MC_LIVEKIT_PUBLIC_URL: env.MC_LIVEKIT_PUBLIC_URL,
      MC_S3_PUBLIC_ENDPOINT: env.MC_S3_PUBLIC_ENDPOINT,
    },
    shell: web.shell,
  });

  // Rebuild on Go or migration changes, debounced so a save-all triggers one rebuild.
  let timer;
  let building = false;
  let pending = false;
  let migrationsChanged = false;
  const rebuild = async () => {
    if (building) return void (pending = true);
    building = true;
    const migrate = migrationsChanged;
    migrationsChanged = false;
    try {
      log.step(`Change detected — rebuilding${migrate ? ' and migrating' : ''}`);
      if (migrate) await prepareBackend(env);
      await buildGo(['api', 'worker'], env);
    } catch (err) {
      log.fail(`${err.message}; keeping the running processes`);
      building = false;
      return;
    }
    await Promise.all([stop('api'), stop('worker')]);
    startGo();
    log.ok('API and worker restarted');
    building = false;
    if (pending) {
      pending = false;
      void rebuild();
    }
  };
  const schedule = (isMigration) => {
    if (isMigration) migrationsChanged = true;
    clearTimeout(timer);
    timer = setTimeout(rebuild, 300);
  };
  watch(SERVICES, { recursive: true }, (_e, file) => {
    if (!file || file.startsWith(`bin${path.sep}`)) return;
    if (file.endsWith('.go') || file === 'go.mod' || file === 'go.sum') schedule(false);
  });
  watch(path.join(ROOT, 'db', 'migrations'), (_e, file) => {
    if (file?.endsWith('.sql')) schedule(true);
  });

  console.log(`
${c.bold('MateCollab is starting')}  →  ${c.cyan('http://localhost:8080')}
  API (direct)      http://127.0.0.1:8081/api/v1/config
  Admin             http://127.0.0.1:8091/readyz  ·  worker http://127.0.0.1:8092/readyz
  Mailpit           http://localhost:8025${profiles.includes('observability') ? '\n  Grafana           http://localhost:3001' : ''}
  Press Ctrl+C to stop the app processes (containers keep running; \`pnpm infra:down\` stops them).
`);

  const shutdown = async () => {
    if (shuttingDown) return;
    shuttingDown = true;
    clearTimeout(timer);
    console.log('\nStopping…');
    await Promise.all([...children.keys()].map(stop));
    process.exit(0);
  };
  process.on('SIGINT', shutdown);
  process.on('SIGTERM', shutdown);
  await new Promise(() => {});
});
