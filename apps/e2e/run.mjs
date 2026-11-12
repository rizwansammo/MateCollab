// `pnpm test:e2e` — end-to-end tests against the running stack (`pnpm dev` or `pnpm dev:docker`).
//
// On the host: checks the stack, resets rate-limit state in the local Valkey, and seeds two
// organisations with `mcctl org create` (the only way organisations are created while self-service
// sign-up is off). The suite then runs in the pinned Playwright container used for visual tests,
// so screenshots match CI. Inside the container a small TCP forwarder makes the stack reachable at
// http://localhost:8080: browsers only accept the API's Secure __Host- cookies on localhost or HTTPS.
//   --update   rewrite the visual baselines (review the diff before committing)
//   --local    run Playwright natively instead (debugging; visual checks are skipped)
//   any other arguments are passed to `playwright test` (e.g. -g "invitation")

import { spawnSync } from 'node:child_process';
import { randomBytes } from 'node:crypto';
import { writeFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { COMPOSE_FILES, ROOT, capture, log, main, readEnvFile, run } from '../../scripts/lib.mjs';

const E2E = path.dirname(fileURLToPath(import.meta.url));
const SUITE = path.join(E2E, 'suite');
const RUNNER_IMAGE = 'matecollab/visual-runner:1.63.0';
const RUNNER_DIR = path.join(ROOT, 'packages', 'ui', 'visual', 'runner');
const BASE = 'http://localhost:8080';

const args = process.argv.slice(2);
const local = args.includes('--local');
const update = args.includes('--update');
const passthrough = args.filter((a) => a !== '--local' && a !== '--update');

async function stackReady() {
  try {
    const res = await fetch(`${BASE}/api/v1/config`);
    return res.ok;
  } catch {
    return false;
  }
}

// Rate limits are per client address; every request from the host arrives from the same Docker
// gateway address, so repeated local runs would otherwise hit the sign-in and recovery limits.
function resetRateLimits(env) {
  const script =
    "for _,p in ipairs({'mc:rl:*','mc:auth:fail:*','mc:auth:wait:*'}) do for _,k in ipairs(redis.call('KEYS',p)) do redis.call('DEL',k) end end return 1";
  const out = capture('docker', [
    'compose',
    ...COMPOSE_FILES,
    'exec',
    '-T',
    'valkey',
    'valkey-cli',
    '--no-auth-warning',
    '-a',
    env.MC_REDIS_PASSWORD,
    'EVAL',
    script,
    '0',
  ]);
  if (out === null) log.warn('Could not reset rate limits in Valkey; repeated runs may be rate limited.');
}

function createOrganisation(name, slug, ownerEmail, ownerName) {
  const create = [
    'org',
    'create',
    '--name',
    name,
    '--slug',
    slug,
    '--owner-email',
    ownerEmail,
    '--owner-name',
    ownerName,
  ];
  // E2E_SEED=docker uses the built mcctl image (CI); otherwise a local Go toolchain if present.
  const hasGo = process.env.E2E_SEED !== 'docker' && capture('go', ['version']) !== null;
  const out = hasGo
    ? capture(process.execPath, [path.join(ROOT, 'scripts', 'mcctl.mjs'), ...create], { cwd: ROOT })
    : capture(
        'docker',
        ['compose', ...COMPOSE_FILES, '--profile', 'app', 'run', '--rm', '--no-deps', 'migrate', ...create],
        {
          cwd: ROOT,
        },
      );
  const id = out?.match(/id:\s+([0-9a-f-]{36})/)?.[1];
  if (!id) throw new Error(`mcctl org create failed for ${slug}:\n${out ?? '(no output)'}`);
  return { id, slug, name, ownerEmail, ownerName };
}

main(async () => {
  const env = readEnvFile();
  if (!env) throw new Error('No .env found; run `pnpm run setup` and start the stack first.');
  if (!(await stackReady())) {
    throw new Error(
      `The stack is not answering at ${BASE}. Start it with \`pnpm dev\` or \`pnpm dev:docker\`.`,
    );
  }

  log.step('Seeding organisations');
  resetRateLimits(env);
  const runId = randomBytes(3).toString('hex');
  const seed = {
    run: runId,
    orgA: createOrganisation('Acme Corporation', `acme-${runId}`, `owner-a-${runId}@e2e.test`, 'Ada Owner'),
    orgB: createOrganisation('Globex', `globex-${runId}`, `owner-b-${runId}@e2e.test`, 'Grace Owner'),
  };
  writeFileSync(path.join(SUITE, '.seed.json'), JSON.stringify(seed, null, 2));
  log.ok(`Seeded run ${runId}`);

  if (local) {
    await run(
      process.execPath,
      [
        path.join(E2E, 'node_modules', '@playwright', 'test', 'cli.js'),
        'test',
        '-c',
        'suite/playwright.config.ts',
        ...passthrough,
      ],
      {
        cwd: E2E,
        env: { ...process.env, E2E_BASE_URL: BASE, E2E_MAILPIT: 'http://localhost:8025' },
      },
    );
    return;
  }

  if (
    spawnSync('docker', ['build', '--quiet', '-t', RUNNER_IMAGE, RUNNER_DIR], { stdio: 'inherit' }).status !==
    0
  ) {
    throw new Error('Could not build the Playwright runner image.');
  }
  const command = [
    'set -e',
    'node /runner/suite/support/forward.mjs 8080 host.docker.internal 8080 &',
    `node --input-type=module -e "for (let i = 0; i < 100; i++) { try { const r = await fetch('http://localhost:8080/api/v1/config'); if (r.ok) process.exit(0); } catch {} await new Promise((resolve) => setTimeout(resolve, 100)); } console.error('E2E proxy did not become ready'); process.exit(1);"`,
    `npx playwright test -c suite/playwright.config.ts${update ? ' --update-snapshots' : ''} ${passthrough.map((a) => `'${a.replaceAll("'", "'\\''")}'`).join(' ')}`,
  ].join('\n');
  const status = spawnSync(
    'docker',
    [
      'run',
      '--rm',
      '--ipc=host',
      '--add-host=host.docker.internal:host-gateway',
      '-e',
      'E2E_BASE_URL=http://localhost:8080',
      '-e',
      'E2E_MAILPIT=http://host.docker.internal:8025',
      '-e',
      'E2E_VISUAL=1',
      '-v',
      `${SUITE}:/runner/suite`,
      '-w',
      '/runner',
      RUNNER_IMAGE,
      'sh',
      '-c',
      command,
    ],
    { stdio: 'inherit' },
  ).status;
  if (status !== 0) {
    throw new Error(
      `End-to-end tests failed. Report: ${path.join(SUITE, '.results', 'report', 'index.html')}`,
    );
  }
  log.ok('End-to-end tests passed');
});
