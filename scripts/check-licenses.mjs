// `pnpm licenses` — dependency licence policy (SECURITY-BASELINE.md §18).
// Permissive licences pass. Weak copyleft (MPL-2.0, LGPL) passes because those dependencies are
// used unmodified and dynamically linked. Strong copyleft or source-available licences (GPL,
// AGPL, SSPL, BUSL, Elastic, Commons Clause) and unknown licences fail and need a review.

import { readFileSync, readdirSync } from 'node:fs';
import path from 'node:path';
import { SERVICES, ScriptError, capture, goEnv, log, main, pnpmCommand } from './lib.mjs';

const ALLOWED = new Set([
  'MIT',
  'MIT-0',
  'ISC',
  '0BSD',
  'BSD-2-Clause',
  'BSD-3-Clause',
  'Apache-2.0',
  'BlueOak-1.0.0',
  'CC0-1.0',
  'CC-BY-4.0', // data only (caniuse-lite)
  'Python-2.0',
  'Unlicense',
  'OFL-1.1', // Inter typeface
  'MPL-2.0',
  'LGPL-3.0-or-later',
  'LGPL-2.1-or-later',
]);
const DENIED = /\b(A?GPL|SSPL|BUSL|Elastic|Commons[- ]Clause)\b/i;

// An SPDX expression passes when every AND-term passes and at least one OR-option passes.
function expressionAllowed(expression) {
  const clean = expression.replace(/[()]/g, ' ').trim();
  return clean.split(/\s+OR\s+/).some((option) =>
    option
      .split(/\s+AND\s+/)
      .map((id) => id.trim())
      .every((id) => ALLOWED.has(id) && !(DENIED.test(id) && !id.startsWith('LGPL'))),
  );
}

function checkJavaScript() {
  const { cmd, args, shell } = pnpmCommand(['licenses', 'list', '--json']);
  const out = capture(cmd, args, { shell });
  if (!out) throw new ScriptError('pnpm licenses list failed');
  const byLicense = JSON.parse(out);
  const problems = [];
  let count = 0;
  for (const [license, packages] of Object.entries(byLicense)) {
    count += packages.length;
    if (!expressionAllowed(license)) {
      problems.push(`${license}: ${packages.map((p) => `${p.name}@${p.versions.join('/')}`).join(', ')}`);
    }
  }
  return { count, problems };
}

function classifyGoLicense(dir) {
  let file;
  try {
    file = readdirSync(dir).find((f) => /^(LICEN[CS]E|COPYING)(\.(md|txt))?$/i.test(f));
  } catch {
    return null;
  }
  if (!file) return null;
  const text = readFileSync(path.join(dir, file), 'utf8');
  // Identify by the title at the top: licence bodies mention other licences (MPL-2.0 names the
  // GNU licences as "Secondary Licenses").
  const head = text.slice(0, 500);
  if (/Mozilla Public License/i.test(head)) return 'MPL-2.0';
  if (/Apache License/i.test(head)) return 'Apache-2.0';
  if (/GNU AFFERO GENERAL PUBLIC LICENSE/i.test(head)) return 'AGPL';
  if (/GNU LESSER GENERAL PUBLIC LICENSE/i.test(head)) return 'LGPL';
  if (/GNU GENERAL PUBLIC LICENSE/i.test(head)) return 'GPL';
  if (/Server Side Public License/i.test(head)) return 'SSPL';
  if (/Permission is hereby granted, free of charge/i.test(text)) return 'MIT';
  if (/Redistribution and use in source and binary forms/i.test(text)) return 'BSD';
  if (/Permission to use, copy, modify, and\/?or distribute/i.test(text)) return 'ISC';
  return null;
}

function checkGo() {
  // Modules actually linked into the binaries (not every module in go.sum).
  const out = capture('go', ['list', '-deps', '-f', '{{with .Module}}{{.Path}}\t{{.Dir}}{{end}}', './...'], {
    cwd: SERVICES,
    env: goEnv(),
  });
  if (out === null) throw new ScriptError('go list failed');
  const modules = new Map();
  for (const line of out.split(/\r?\n/)) {
    const [mod, dir] = line.split('\t');
    if (mod && dir && !mod.startsWith('github.com/rizwansammo/MateCollab')) modules.set(mod, dir);
  }
  const problems = [];
  for (const [mod, dir] of modules) {
    const license = classifyGoLicense(dir);
    if (!license) problems.push(`${mod}: licence not recognised`);
    else if (['AGPL', 'GPL', 'SSPL'].includes(license)) problems.push(`${mod}: ${license}`);
  }
  return { count: modules.size, problems };
}

main(async () => {
  const js = checkJavaScript();
  const go = checkGo();
  const problems = [...js.problems.map((p) => `npm  ${p}`), ...go.problems.map((p) => `go   ${p}`)];
  if (problems.length) {
    throw new ScriptError(`Licence policy violations (review before use):\n  - ${problems.join('\n  - ')}`);
  }
  log.ok(`Licences OK: ${js.count} npm packages, ${go.count} Go modules`);
});
