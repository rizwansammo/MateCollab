// Generates .env from .env.example. Every __TOKEN__ receives a random value (the same token
// always gets the same value). Re-running keeps existing values and only appends keys that
// were added to the template since, reusing token values already present in .env.

import { randomBytes } from 'node:crypto';
import { existsSync, readFileSync, writeFileSync, appendFileSync } from 'node:fs';
import path from 'node:path';
import { ENV_FILE, ROOT, parseEnv } from './lib.mjs';

const TEMPLATE = path.join(ROOT, '.env.example');
const TOKEN = /__([A-Z0-9_]+)__/g;

function generate(token) {
  // Keyring keys are exactly 32 bytes of standard base64; everything else is URL-safe so it
  // can sit inside connection URLs unescaped.
  if (token.endsWith('ENCRYPTION_KEY')) return randomBytes(32).toString('base64');
  return randomBytes(32).toString('base64url');
}

function templateEntries(text) {
  const entries = [];
  for (const line of text.split(/\r?\n/)) {
    const m = /^([A-Z][A-Z0-9_]*)=(.*)$/.exec(line.trim());
    if (m) entries.push({ key: m[1], raw: m[2], line });
  }
  return entries;
}

// learnTokens recovers token values from an existing .env by matching each templated value.
function learnTokens(entries, current) {
  const known = new Map();
  for (const { key, raw } of entries) {
    const names = [...raw.matchAll(TOKEN)].map((m) => m[1]);
    if (!names.length || current[key] === undefined) continue;
    const unquoted = raw.replace(/^"(.*)"$/, '$1');
    const pattern = unquoted
      .split(TOKEN)
      .map((part, i) => (i % 2 ? '(.+?)' : part.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
      .join('');
    const m = new RegExp(`^${pattern}$`).exec(current[key]);
    if (m) names.forEach((name, i) => known.set(name, m[i + 1]));
  }
  return known;
}

function fill(text, tokens) {
  return text.replace(TOKEN, (_, name) => {
    if (!tokens.has(name)) tokens.set(name, generate(name));
    return tokens.get(name);
  });
}

// ensureEnvFile creates or tops up .env and reports what happened.
export function ensureEnvFile() {
  const template = readFileSync(TEMPLATE, 'utf8');
  const entries = templateEntries(template);
  if (!existsSync(ENV_FILE)) {
    writeFileSync(ENV_FILE, fill(template, new Map()), { encoding: 'utf8', mode: 0o600 });
    return { created: true, added: [] };
  }
  const current = parseEnv(readFileSync(ENV_FILE, 'utf8'));
  const missing = entries.filter((e) => current[e.key] === undefined);
  if (missing.length) {
    const tokens = learnTokens(entries, current);
    const block = missing.map((e) => fill(e.line.trim(), tokens)).join('\n');
    appendFileSync(ENV_FILE, `\n# Added by pnpm setup from .env.example\n${block}\n`, 'utf8');
  }
  return { created: false, added: missing.map((e) => e.key) };
}
