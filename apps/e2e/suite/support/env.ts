import { createHmac } from 'node:crypto';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, type Page } from '@playwright/test';

export interface SeededOrg {
  id: string;
  slug: string;
  name: string;
  ownerEmail: string;
  ownerName: string;
}

/** Organisations created for this run by ../run.mjs (mcctl org create). */
export const seed: { run: string; orgA: SeededOrg; orgB: SeededOrg } = JSON.parse(
  readFileSync(path.join(path.dirname(fileURLToPath(import.meta.url)), '..', '.seed.json'), 'utf8'),
);

export const visual = process.env.E2E_VISUAL === '1';
const MAILPIT = process.env.E2E_MAILPIT ?? 'http://localhost:8025';

/** A unique address for this run. */
export const address = (name: string) => `${name}-${seed.run}@e2e.test`;

interface MailSummary {
  ID: string;
  Subject: string;
  To: { Address: string }[];
  Created: string;
}

/**
 * Waits for the newest email to `to` whose subject contains `subject`, created after `since`,
 * and returns the one-time link it carries (token in the URL fragment).
 */
export async function emailLink(to: string, subject: string, since = 0): Promise<string> {
  let found: MailSummary | undefined;
  await expect
    .poll(
      async () => {
        const res = await fetch(
          `${MAILPIT}/api/v1/search?query=${encodeURIComponent(`to:"${to}"`)}&limit=20`,
        );
        const body = (await res.json()) as { messages: MailSummary[] };
        found = body.messages.find((m) => m.Subject.includes(subject) && Date.parse(m.Created) >= since);
        return Boolean(found);
      },
      { message: `email "${subject}" to ${to}`, timeout: 30_000, intervals: [500, 1000, 2000] },
    )
    .toBe(true);
  if (!found) throw new Error(`no email "${subject}" to ${to}`);
  const msg = (await (await fetch(`${MAILPIT}/api/v1/message/${found.ID}`)).json()) as { Text: string };
  const link = msg.Text.match(/https?:\/\/\S+#token=[A-Za-z0-9_-]+/)?.[0];
  if (!link) throw new Error(`no one-time link in "${found.Subject}"`);
  const url = new URL(link);
  // The email carries the public URL; the browser in the container reaches the same origin.
  return `${url.pathname}${url.hash}`;
}

/** RFC 6238 TOTP (SHA-1, 6 digits, 30 s) for a base32 secret, as an authenticator app computes it. */
export function totp(secretBase32: string, at = Date.now()): string {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  let bits = '';
  for (const ch of secretBase32.replace(/[\s=]/g, '').toUpperCase())
    bits += alphabet.indexOf(ch).toString(2).padStart(5, '0');
  const key = Buffer.from((bits.match(/.{8}/g) ?? []).map((b) => parseInt(b, 2)));
  const counter = Buffer.alloc(8);
  counter.writeBigUInt64BE(BigInt(Math.floor(at / 1000 / 30)));
  const mac = createHmac('sha1', key).update(counter).digest();
  const offset = mac.readUInt8(mac.length - 1) & 0x0f;
  const code = (mac.readUInt32BE(offset) & 0x7fffffff) % 1_000_000;
  return code.toString().padStart(6, '0');
}

/** Waits until the next 30-second TOTP step, so a code is never reused (replay protection). */
export async function nextTotpWindow(page: Page) {
  // A clear margin past the boundary: the server's clock and request latency must not put the
  // code back in the step that was already used.
  const ms = 30_000 - (Date.now() % 30_000) + 1_500;
  await page.waitForTimeout(ms);
}

/**
 * Before a screenshot: fonts loaded and nothing still loading. Callers first wait for something
 * only the hydrated client renders (API data), so screenshots never race hydration.
 */
export async function settle(page: Page) {
  await page.waitForLoadState('load');
  await page.evaluate(() => document.fonts.ready);
  await expect(page.locator('[aria-busy="true"], [role="status"] .animate-spin')).toHaveCount(0);
}

/** Signs in through the UI and waits for the organisation home. */
export async function signIn(page: Page, email: string, password: string, landing: RegExp = /\/home$/) {
  await page.goto('/login');
  await page.getByLabel('Work email').fill(email);
  await page.getByLabel('Password', { exact: true }).fill(password);
  await page.getByRole('button', { name: 'Continue' }).click();
  await expect(page).toHaveURL(landing);
}

/** Signs out through the account menu. */
export async function signOut(page: Page) {
  await page.getByRole('button', { name: /^Account:/ }).click();
  await page.getByRole('menuitem', { name: 'Sign out' }).click();
  await expect(page).toHaveURL(/\/login$/);
}

/** Calls the API from the page's browser context (cookies and CSRF as the web client sends them). */
export async function apiFromPage(page: Page, method: string, path: string, body?: unknown): Promise<number> {
  return page.evaluate(
    async ({ method, path, body }) => {
      const session = await fetch('/api/v1/auth/session');
      const csrf = session.ok ? ((await session.json()) as { csrf_token: string }).csrf_token : '';
      const res = await fetch(`/api/v1${path}`, {
        method,
        headers: { 'X-CSRF-Token': csrf, 'X-MC-Client': 'web', 'Content-Type': 'application/json' },
        body: body === undefined ? undefined : JSON.stringify(body),
      });
      return res.status;
    },
    { method, path, body },
  );
}
