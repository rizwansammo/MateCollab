// `pnpm brand:sync` — copy the curated brand assets listed in scripts/brand-assets.json from
// MateCollab-Logo-Library into the web app, verifying SHA-256 hashes.
//   (default)   copy when missing or different; refuse if a Library file no longer matches its
//               approved hash
//   --check     write nothing; fail if any copy is missing or differs, if a Library file changed,
//               or if a HemiHead font file exists anywhere in the product
//   --accept    record the current Library hashes (after the brand owner approved a change)
// The Library is only ever read.

import { createHash } from 'node:crypto';
import { copyFileSync, existsSync, mkdirSync, readFileSync, readdirSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { ROOT, ScriptError, log, main } from './lib.mjs';

const MANIFEST = path.join(ROOT, 'scripts', 'brand-assets.json');
const args = new Set(process.argv.slice(2));
const CHECK = args.has('--check');
const ACCEPT = args.has('--accept');

const sha256 = (file) => createHash('sha256').update(readFileSync(file)).digest('hex');

// HemiHead is a restricted typeface: the wordmark exists only as outlined SVG artwork.
function findRestrictedFonts() {
  const hits = [];
  const skip = new Set(['node_modules', '.git', '.next', 'storybook-static', 'MateCollab-Logo-Library']);
  const walk = (dir) => {
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      if (skip.has(entry.name)) continue;
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) walk(full);
      else if (/hemihead/i.test(entry.name)) hits.push(path.relative(ROOT, full));
    }
  };
  for (const dir of ['apps', 'packages']) if (existsSync(path.join(ROOT, dir))) walk(path.join(ROOT, dir));
  return hits;
}

main(async () => {
  const manifest = JSON.parse(readFileSync(MANIFEST, 'utf8'));
  const library = path.join(ROOT, manifest.library);
  const haveLibrary = existsSync(library);
  if (!haveLibrary && !CHECK) throw new ScriptError(`Logo Library not found at ${manifest.library}`);

  const problems = [];
  let copied = 0;
  let accepted = 0;
  for (const asset of manifest.assets) {
    const source = path.join(library, asset.source);
    const target = path.join(ROOT, asset.target);

    if (haveLibrary) {
      if (!existsSync(source)) {
        problems.push(`missing in Library: ${asset.source}`);
        continue;
      }
      const sourceHash = sha256(source);
      if (sourceHash !== asset.sha256) {
        if (ACCEPT) {
          asset.sha256 = sourceHash;
          accepted++;
        } else {
          problems.push(
            `Library file changed: ${asset.source} (approved ${asset.sha256.slice(0, 12)}…, now ${sourceHash.slice(0, 12)}…)`,
          );
          continue;
        }
      }
    }

    const targetHash = existsSync(target) ? sha256(target) : null;
    if (targetHash === asset.sha256) continue;
    if (CHECK) {
      problems.push(targetHash ? `modified copy: ${asset.target}` : `missing copy: ${asset.target}`);
      continue;
    }
    mkdirSync(path.dirname(target), { recursive: true });
    copyFileSync(source, target);
    if (sha256(target) !== asset.sha256) problems.push(`copy verification failed: ${asset.target}`);
    else copied++;
  }

  for (const font of findRestrictedFonts())
    problems.push(`restricted HemiHead font file in the product: ${font}`);

  if (accepted) writeFileSync(MANIFEST, JSON.stringify(manifest, null, 2) + '\n', 'utf8');
  if (problems.length) {
    const hint = CHECK
      ? 'Run `pnpm brand:sync`.'
      : 'If the brand owner approved a Library change, run `pnpm brand:sync --accept`.';
    throw new ScriptError(`Brand assets are not in sync:\n  - ${problems.join('\n  - ')}\n${hint}`);
  }
  if (CHECK) {
    log.ok(
      `${manifest.assets.length} brand assets match the approved Library hashes${haveLibrary ? '' : ' (Library not present; copies checked)'}`,
    );
  } else {
    log.ok(
      `Brand assets in sync (${copied} copied, ${manifest.assets.length - copied} unchanged${accepted ? `, ${accepted} accepted` : ''})`,
    );
  }
});
