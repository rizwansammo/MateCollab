// `pnpm setup` — one-time (and repeatable) local preparation:
// prerequisites → .env → dependencies → brand assets → contracts → infrastructure → Go
// binaries → database roles, migrations and bucket → doctor.

import { existsSync } from 'node:fs';
import path from 'node:path';
import { ensureEnvFile } from './env-file.mjs';
import {
  ROOT,
  SERVICES,
  ScriptError,
  buildGo,
  binPath,
  c,
  capture,
  dockerAvailable,
  goEnv,
  log,
  main,
  pnpm,
  prepareBackend,
  requireEnv,
  run,
  startInfra,
} from './lib.mjs';

const args = new Set(process.argv.slice(2));

main(async () => {
  console.log(c.bold('MateCollab local setup\n'));

  log.step('Checking prerequisites');
  const nodeMajor = Number(process.versions.node.split('.')[0]);
  if (nodeMajor < 24)
    throw new ScriptError(`Node ${process.versions.node} found; Node 24 or newer is required.`);
  const docker = dockerAvailable();
  if (!docker) throw new ScriptError('Docker is not running. Start Docker Desktop and retry.');
  const go = capture('go', ['version'], { cwd: SERVICES, env: goEnv() });
  if (!go) {
    throw new ScriptError(
      'Go is not available. Install Go (https://go.dev/dl); go.mod then selects Go 1.27.',
    );
  }
  log.ok(`Node ${process.versions.node}, Docker ${docker}, ${go.replace(/^go version /, '')}`);

  log.step('Preparing .env');
  const envResult = ensureEnvFile();
  if (envResult.created) log.ok('Created .env with freshly generated local secrets');
  else if (envResult.added.length) log.ok(`Added new settings to .env: ${envResult.added.join(', ')}`);
  else log.ok('.env is up to date (existing secrets kept)');
  const env = requireEnv();

  if (!args.has('--skip-install')) {
    log.step('Installing JavaScript dependencies');
    await pnpm(['install']);
  }

  if (existsSync(path.join(ROOT, 'MateCollab-Logo-Library'))) {
    log.step('Syncing official brand assets');
    await run(process.execPath, ['scripts/brand-sync.mjs']);
  } else {
    log.warn('MateCollab-Logo-Library not found; keeping the committed brand assets');
  }

  log.step('Generating contract types');
  await pnpm(['run', 'gen']);

  log.step('Starting infrastructure (first run builds the local MinIO image; this can take a few minutes)');
  await startInfra();
  log.ok('PostgreSQL, Valkey, MinIO, LiveKit, Mailpit and Nginx are up');

  log.step('Building Go services');
  await run('go', ['mod', 'download'], { cwd: SERVICES, env: goEnv(env) });
  await buildGo(['api', 'worker'], env);

  log.step('Preparing database roles, migrations and object storage');
  await prepareBackend(env);

  log.step('Checking dependencies');
  await run(binPath('mcctl'), ['doctor'], { env });

  console.log(`
${c.green(c.bold('Setup complete.'))}

  ${c.bold('pnpm dev')}          start API, worker and web with hot reload
  ${c.bold('pnpm dev:docker')}   run everything in containers instead

  App        http://localhost:8080
  Mailpit    http://localhost:8025
  MinIO      http://localhost:9001
`);
});
