// `pnpm mcctl <args>` — build and run the operator CLI against the local .env.

import { binPath, buildGo, main, requireEnv, run } from './lib.mjs';

main(async () => {
  const env = requireEnv();
  await buildGo(['mcctl'], env);
  await run(binPath('mcctl'), process.argv.slice(2), { env });
});
