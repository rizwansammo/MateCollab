// Go task runner used by the root package scripts.
//   fmt | fmt-check | lint | test | integration | build | vuln
//
// `integration` starts a throwaway PostgreSQL container on a random loopback port (roles are
// cluster-wide, so tests never touch the development database) and, when the local stack is
// running, points the Valkey, MinIO and Mailpit tests at it. Variables already present in the
// environment (as in CI) take precedence.

import { randomBytes } from 'node:crypto';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import {
  COMPOSE_FILES,
  ROOT,
  SERVICES,
  ScriptError,
  capture,
  goEnv,
  log,
  main,
  readEnvFile,
  run,
} from './lib.mjs';

// Pinned tool versions (bump deliberately).
const GOVULNCHECK = 'golang.org/x/vuln/cmd/govulncheck@v1.8.0';
// staticcheck v0.8.1 cannot read Go 1.27 export data yet; add it to `lint` once a compatible release ships.

const go = (args, env = goEnv()) => run('go', args, { cwd: SERVICES, env });

function gofmtPath() {
  const goroot = capture('go', ['env', 'GOROOT'], { cwd: SERVICES, env: goEnv() });
  if (!goroot) throw new ScriptError('go env GOROOT failed');
  return path.join(goroot, 'bin', process.platform === 'win32' ? 'gofmt.exe' : 'gofmt');
}

async function fmtCheck() {
  const out = capture(gofmtPath(), ['-l', '.'], { cwd: SERVICES });
  if (out === null) throw new ScriptError('gofmt failed');
  if (out) throw new ScriptError(`gofmt needed for:\n${out}\nRun \`pnpm format\`.`);
  log.ok('Go formatting clean');
}

function postgresImage() {
  const compose = readFileSync(path.join(ROOT, 'docker-compose.yml'), 'utf8');
  const m = /image:\s*(postgres:[^\s]+)/.exec(compose);
  if (!m) throw new ScriptError('postgres image not found in docker-compose.yml');
  return m[1];
}

function runningServices() {
  const out = capture('docker', ['compose', ...COMPOSE_FILES, 'ps', '--status', 'running', '--services']);
  return new Set(out ? out.split(/\r?\n/) : []);
}

async function waitFor(check, what, timeoutMs = 60_000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (check()) return;
    await new Promise((r) => setTimeout(r, 500));
  }
  throw new ScriptError(`timed out waiting for ${what}`);
}

async function integration() {
  const env = goEnv();
  let container;
  try {
    if (!env.MC_TEST_DATABASE_BOOTSTRAP_URL) {
      const password = randomBytes(24).toString('base64url');
      container = `mc-itest-pg-${randomBytes(4).toString('hex')}`;
      log.step(`Starting throwaway PostgreSQL (${container})`);
      await run(
        'docker',
        [
          'run',
          '-d',
          '--rm',
          '--name',
          container,
          '-e',
          `POSTGRES_PASSWORD=${password}`,
          '--tmpfs',
          '/var/lib/postgresql',
          '-p',
          '127.0.0.1::5432',
          postgresImage(),
        ],
        { stdio: ['ignore', 'ignore', 'inherit'] },
      );
      await waitFor(
        () =>
          capture('docker', ['exec', container, 'pg_isready', '-U', 'postgres', '-h', '127.0.0.1']) !== null,
        'PostgreSQL',
      );
      const port = capture('docker', ['port', container, '5432/tcp']).split(/\r?\n/)[0].split(':').pop();
      env.MC_TEST_DATABASE_BOOTSTRAP_URL = `postgres://postgres:${password}@127.0.0.1:${port}/postgres?sslmode=disable`;
      env.MC_TEST_ROLE_PASSWORD ||= randomBytes(24).toString('base64url');
    }

    const dev = readEnvFile();
    const running = runningServices();
    if (dev && running.has('valkey')) env.MC_TEST_REDIS_URL ||= dev.MC_REDIS_URL.replace(/\/\d+$/, '/15');
    if (dev && running.has('mailpit')) {
      env.MC_TEST_MAILPIT_API ||= 'http://localhost:8025';
      env.MC_TEST_SMTP_HOST ||= 'localhost';
      env.MC_TEST_SMTP_PORT ||= '1025';
    }
    if (dev && running.has('minio')) {
      env.MC_TEST_S3_ENDPOINT ||= dev.MC_S3_ENDPOINT;
      env.MC_TEST_S3_ACCESS_KEY_ID ||= dev.MC_S3_ACCESS_KEY_ID;
      env.MC_TEST_S3_SECRET_ACCESS_KEY ||= dev.MC_S3_SECRET_ACCESS_KEY;
    }
    const missing = ['MC_TEST_REDIS_URL', 'MC_TEST_MAILPIT_API', 'MC_TEST_S3_ENDPOINT'].filter(
      (k) => !env[k],
    );
    if (missing.length)
      log.warn(`Not set, related tests will skip: ${missing.join(', ')} (start \`pnpm infra:up\`)`);

    // -p 1: packages bootstrap the same cluster-wide roles, so they must not run concurrently.
    await go(['test', '-tags', 'integration', '-count=1', '-p', '1', ...extra, './...'], env);
  } finally {
    if (container) capture('docker', ['rm', '-f', container]);
  }
}

const [command, ...extra] = process.argv.slice(2);

main(async () => {
  switch (command) {
    case 'fmt':
      await go(['fmt', './...']);
      break;
    case 'fmt-check':
      await fmtCheck();
      break;
    case 'lint':
      await go(['vet', './...']);
      await go(['vet', '-tags', 'integration', './...']);
      log.ok('go vet clean (default and integration builds)');
      break;
    case 'test':
      await go(['test', '-count=1', ...extra, './...']);
      break;
    case 'integration':
      await integration();
      break;
    case 'build':
      await go(['build', './...']);
      break;
    case 'vuln':
      await go(['run', GOVULNCHECK, './...']);
      break;
    default:
      throw new ScriptError('Usage: node scripts/go.mjs fmt|fmt-check|lint|test|integration|build|vuln');
  }
});
