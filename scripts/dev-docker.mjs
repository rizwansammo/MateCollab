// `pnpm dev:docker` — the whole stack in containers (production-like images, no hot reload).
// Nginx is pointed at the api and web containers instead of the host.
//   pnpm dev:docker            build, start and wait until healthy
//   pnpm dev:docker --logs     …then follow application logs (Ctrl+C stops following only)

import { c, compose, log, main, requireEnv } from './lib.mjs';

main(async () => {
  const env = {
    ...requireEnv(),
    MC_NGINX_WEB_UPSTREAM: 'web:3000',
    MC_NGINX_API_UPSTREAM: 'api:8081',
  };
  log.step('Building images and starting the full stack (first build takes several minutes)');
  await compose(['--profile', 'app', 'up', '--build', '-d', '--wait', '--remove-orphans'], { env });
  log.ok('All services are healthy');
  console.log(`
  App       ${c.cyan('http://localhost:8080')}
  Mailpit   http://localhost:8025
  Logs      pnpm infra:logs api worker web
  Stop      pnpm infra:down
`);
  if (process.argv.includes('--logs')) {
    await compose(['--profile', 'app', 'logs', '-f', 'api', 'worker', 'web', 'nginx'], { env }).catch(
      () => {},
    );
  }
});
