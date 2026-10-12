// `pnpm check` — every CI gate that runs without Docker, in one command:
// formatting, lint, types, unit tests, contract freshness, brand assets and licences.
// Docker-backed gates run separately: `pnpm test:go:integration` and `pnpm visual:test`.

import { c, log, main, pnpmCommand, run, ScriptError } from './lib.mjs';

const node = (script, ...args) => [process.execPath, [script, ...args]];
const pnpm = (...args) => {
  const { cmd, args: argv, shell } = pnpmCommand(args);
  return [cmd, argv, { shell }];
};

const steps = [
  ['Formatting (Prettier)', ...pnpm('exec', 'prettier', '--check', '.')],
  ['Formatting (gofmt)', ...node('scripts/go.mjs', 'fmt-check')],
  ['Lint (ESLint, Redocly)', ...pnpm('-r', '--if-present', 'lint')],
  ['Lint (go vet)', ...node('scripts/go.mjs', 'lint')],
  ['Types (TypeScript)', ...pnpm('-r', '--if-present', 'typecheck')],
  ['Unit tests (web, design system, contracts)', ...pnpm('-r', '--if-present', 'test')],
  ['Unit tests (Go)', ...node('scripts/go.mjs', 'test')],
  ['Contracts up to date', ...pnpm('--filter', '@matecollab/contracts', 'check')],
  ['Brand assets', ...node('scripts/brand-sync.mjs', '--check')],
  ['Licences', ...node('scripts/check-licenses.mjs')],
];

main(async () => {
  const results = [];
  for (const [name, cmd, args, opts] of steps) {
    log.step(c.bold(name));
    const started = Date.now();
    try {
      await run(cmd, args, opts);
      results.push([name, true, Date.now() - started]);
    } catch (err) {
      results.push([name, false, Date.now() - started]);
      log.fail(err.message);
    }
  }
  console.log(`\n${c.bold('Summary')}`);
  for (const [name, ok, ms] of results) {
    console.log(`  ${ok ? c.green('✓') : c.red('✗')} ${name} ${c.dim(`${(ms / 1000).toFixed(1)}s`)}`);
  }
  const failed = results.filter(([, ok]) => !ok).length;
  if (failed) throw new ScriptError(`${failed} check(s) failed`);
  log.ok('All checks passed');
});
