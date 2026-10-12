// Shared helpers for the cross-platform root scripts (Windows, macOS, Linux). No dependencies
// beyond Node itself so they work before `pnpm install` has run.

import { spawn, spawnSync } from 'node:child_process';
import { existsSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

export const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
export const SERVICES = path.join(ROOT, 'services');
export const ENV_FILE = path.join(ROOT, '.env');
export const IS_WINDOWS = process.platform === 'win32';
export const EXE = IS_WINDOWS ? '.exe' : '';

export const COMPOSE_FILES = ['-f', 'docker-compose.yml', '-f', 'docker-compose.dev.yml'];
export const INFRA_SERVICES = ['postgres', 'valkey', 'minio', 'livekit', 'mailpit', 'nginx'];

const useColor = process.stdout.isTTY && !process.env.NO_COLOR;
const paint = (code) => (s) => (useColor ? `\x1b[${code}m${s}\x1b[0m` : String(s));
export const c = {
  bold: paint('1'),
  dim: paint('2'),
  red: paint('31'),
  green: paint('32'),
  yellow: paint('33'),
  blue: paint('34'),
  magenta: paint('35'),
  cyan: paint('36'),
};

export const log = {
  step: (msg) => console.log(`${c.cyan('›')} ${msg}`),
  ok: (msg) => console.log(`${c.green('✓')} ${msg}`),
  warn: (msg) => console.warn(`${c.yellow('!')} ${msg}`),
  fail: (msg) => console.error(`${c.red('✗')} ${msg}`),
};

export class ScriptError extends Error {}

// main runs an async entry point and turns failures into a clean message and exit code.
export function main(fn) {
  fn().catch((err) => {
    log.fail(err instanceof ScriptError ? err.message : (err?.stack ?? String(err)));
    process.exit(1);
  });
}

// parseEnv reads KEY=VALUE lines; values may be wrapped in single or double quotes.
export function parseEnv(text) {
  const out = {};
  for (const raw of text.split(/\r?\n/)) {
    const line = raw.trim();
    if (!line || line.startsWith('#')) continue;
    const eq = line.indexOf('=');
    if (eq <= 0) continue;
    const key = line.slice(0, eq).trim();
    let value = line.slice(eq + 1).trim();
    if (value.length >= 2 && (value[0] === '"' || value[0] === "'") && value.at(-1) === value[0]) {
      value = value.slice(1, -1);
    }
    out[key] = value;
  }
  return out;
}

export function readEnvFile(file = ENV_FILE) {
  return existsSync(file) ? parseEnv(readFileSync(file, 'utf8')) : null;
}

// requireEnv returns .env merged under the current process environment (real environment
// variables win), failing with setup guidance when .env does not exist yet.
export function requireEnv() {
  const fileEnv = readEnvFile();
  if (!fileEnv) throw new ScriptError('.env not found. Run `pnpm setup` first.');
  return { ...fileEnv, ...process.env };
}

// goEnv lets the go.mod toolchain directive fetch Go 1.27 when an older Go is installed.
export function goEnv(env = process.env) {
  return { ...env, GOTOOLCHAIN: env.GOTOOLCHAIN || 'auto' };
}

// run executes a command, streaming its output, and resolves when it exits successfully.
export function run(cmd, args, opts = {}) {
  return new Promise((resolve, reject) => {
    const child = spawn(cmd, args, {
      cwd: opts.cwd ?? ROOT,
      env: opts.env ?? process.env,
      stdio: opts.stdio ?? 'inherit',
      shell: opts.shell ?? false,
    });
    child.on('error', (err) =>
      reject(new ScriptError(`${cmd} could not be started: ${err.message}${hintFor(cmd)}`)),
    );
    child.on('exit', (code, signal) => {
      if (code === 0) resolve();
      else reject(new ScriptError(`${[cmd, ...args].join(' ')} failed (${signal ?? `exit ${code}`})`));
    });
  });
}

// capture runs a command synchronously and returns its trimmed output, or null on failure.
export function capture(cmd, args, opts = {}) {
  const res = spawnSync(cmd, args, {
    cwd: opts.cwd ?? ROOT,
    env: opts.env ?? process.env,
    encoding: 'utf8',
    shell: opts.shell ?? false,
  });
  if (res.error || res.status !== 0) return null;
  return res.stdout.trim();
}

function hintFor(cmd) {
  if (cmd === 'docker') return ' — install and start Docker Desktop (or Docker Engine).';
  if (cmd === 'go') return ' — install Go (https://go.dev/dl); go.mod selects Go 1.27 automatically.';
  return '';
}

// pnpm invokes pnpm through the shell on Windows, where it is a .cmd/.ps1 shim.
export function pnpm(args, opts = {}) {
  return run('pnpm', args, { ...opts, shell: IS_WINDOWS });
}

export function compose(args, opts = {}) {
  return run('docker', ['compose', ...COMPOSE_FILES, ...args], opts);
}

export function dockerAvailable() {
  return capture('docker', ['info', '--format', '{{.ServerVersion}}']);
}

export function binPath(name) {
  return path.join(SERVICES, 'bin', name + EXE);
}

// buildGo compiles one or more service binaries into services/bin.
export async function buildGo(names, env = process.env) {
  for (const name of names) {
    await run('go', ['build', '-o', binPath(name), `./${name}`], { cwd: SERVICES, env: goEnv(env) });
  }
}

// startInfra starts the infrastructure containers and waits until they report healthy.
export async function startInfra(extraProfiles = []) {
  if (!dockerAvailable()) throw new ScriptError('Docker is not running. Start Docker Desktop and retry.');
  const profiles = extraProfiles.flatMap((p) => ['--profile', p]);
  const extra = extraProfiles.map((p) => ({ scan: 'clamav', observability: 'otel-lgtm' })[p]).filter(Boolean);
  await compose([...profiles, 'up', '-d', '--wait', ...INFRA_SERVICES, ...extra]);
}

// prepareBackend makes the database and bucket match the code: roles (idempotent), schema
// migrations and bucket policy. Safe to run on every start.
export async function prepareBackend(env) {
  await buildGo(['mcctl'], env);
  await run(binPath('mcctl'), ['prepare'], { env });
}

// killTree stops a child process and everything it started.
export function killTree(child) {
  if (!child || child.exitCode !== null || child.pid === undefined) return;
  if (IS_WINDOWS) {
    spawnSync('taskkill', ['/pid', String(child.pid), '/T', '/F'], { stdio: 'ignore' });
  } else {
    try {
      process.kill(-child.pid, 'SIGTERM');
    } catch {
      child.kill('SIGTERM');
    }
  }
}
