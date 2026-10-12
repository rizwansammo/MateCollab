// `pnpm infra:<command>` — manage the local Docker stack.
//   up [--scan] [--observability]   start infrastructure (+ ClamAV / Grafana LGTM)
//   down                            stop and remove all MateCollab containers (data kept)
//   reset --yes                     as down, and delete the PostgreSQL and MinIO volumes
//   logs [service…]                 follow logs
//   ps                              list containers
//   psql                            open psql in the database as the superuser

import { c, compose, log, main, requireEnv, ScriptError, startInfra } from './lib.mjs';

const [command, ...rest] = process.argv.slice(2);
const ALL_PROFILES = ['--profile', 'app', '--profile', 'scan', '--profile', 'observability'];

main(async () => {
  const env = requireEnv();
  switch (command) {
    case 'up': {
      const profiles = [];
      if (rest.includes('--scan')) profiles.push('scan');
      if (rest.includes('--observability')) profiles.push('observability');
      await startInfra(profiles);
      log.ok('Infrastructure is up');
      if (profiles.includes('observability')) {
        console.log(
          `  Grafana ${c.cyan('http://localhost:3001')} — set MC_OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318`,
        );
      }
      break;
    }
    case 'down':
      await compose([...ALL_PROFILES, 'down', '--remove-orphans'], { env });
      break;
    case 'reset':
      if (!rest.includes('--yes')) {
        throw new ScriptError(
          'This deletes the local PostgreSQL and MinIO data. Re-run as `pnpm infra:reset --yes`.',
        );
      }
      await compose([...ALL_PROFILES, 'down', '--remove-orphans', '--volumes'], { env });
      log.ok('Local data removed. Run `pnpm dev` (or `pnpm setup`) to recreate it.');
      break;
    case 'logs':
      await compose([...ALL_PROFILES, 'logs', '-f', '--tail', '200', ...rest], { env });
      break;
    case 'ps':
      await compose([...ALL_PROFILES, 'ps'], { env });
      break;
    case 'psql':
      await compose(['exec', 'postgres', 'psql', '-U', 'postgres', '-d', 'matecollab', ...rest], { env });
      break;
    default:
      throw new ScriptError('Usage: node scripts/infra.mjs up|down|reset|logs|ps|psql');
  }
});
