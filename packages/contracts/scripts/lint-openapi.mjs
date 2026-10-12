// Runs `redocly lint` with its network update check and telemetry disabled (offline and
// deterministic in CI; the update check's late network handle can also crash Node on exit on
// Windows).

import { spawnSync } from 'node:child_process';
import { createRequire } from 'node:module';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const PKG = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const cli = path.join(path.dirname(require.resolve('@redocly/cli/package.json')), 'bin', 'cli.js');
const res = spawnSync(process.execPath, [cli, 'lint', 'openapi/openapi.yaml', '--format', 'stylish'], {
  cwd: PKG,
  stdio: 'inherit',
  env: { ...process.env, REDOCLY_TELEMETRY: 'off', REDOCLY_SUPPRESS_UPDATE_NOTICE: 'true', NO_COLOR: '1' },
});
process.exit(res.status ?? 1);
