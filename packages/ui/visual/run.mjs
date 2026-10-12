// `pnpm visual:test` / `pnpm visual:update` — build Storybook, then compare every story (light
// and dark) with the committed baselines inside the pinned Playwright container, so rendering is
// identical on Windows, macOS, Linux and CI. Report: packages/ui/visual/.results/report.
//   --update     rewrite baselines (review the diff before committing)
//   --no-build   reuse the existing storybook-static build
import { spawnSync } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const VISUAL = path.dirname(fileURLToPath(import.meta.url));
const UI = path.resolve(VISUAL, '..');
const IMAGE = 'matecollab/visual-runner:1.63.0';
const args = new Set(process.argv.slice(2));
const isWindows = process.platform === 'win32';

function run(cmd, cmdArgs, opts = {}) {
  const res = spawnSync(cmd, cmdArgs, { stdio: 'inherit', shell: opts.shell ?? false, cwd: opts.cwd ?? UI });
  if (res.error) throw res.error;
  return res.status ?? 1;
}

if (!args.has('--no-build')) {
  // pnpm is a shell shim on Windows; pass one command string there (Node DEP0190).
  const build = isWindows ? run('pnpm run build-storybook', [], { shell: true }) : run('pnpm', ['run', 'build-storybook']);
  if (build !== 0) process.exit(1);
}
if (run('docker', ['build', '--quiet', '-t', IMAGE, path.join(VISUAL, 'runner')]) !== 0) process.exit(1);

const status = run('docker', [
  'run',
  '--rm',
  '--ipc=host',
  '-v',
  `${path.join(UI, 'storybook-static')}:/storybook-src:ro`,
  '-v',
  `${VISUAL}:/runner/suite`,
  '-w',
  '/runner',
  IMAGE,
  // Serve from the container's own filesystem: bind mounts are slow on Docker Desktop.
  'sh',
  '-c',
  `cp -r /storybook-src /storybook && npx playwright test -c suite/playwright.config.ts${args.has('--update') ? ' --update-snapshots' : ''}`,
]);
if (status !== 0) {
  console.error(
    `\nVisual or accessibility differences found. Report: ${path.join(VISUAL, '.results', 'report', 'index.html')}`,
  );
}
process.exit(status);
