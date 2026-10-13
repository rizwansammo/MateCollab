import { expect, test, type Browser, type Page } from '@playwright/test';
import {
  address,
  apiFromPage,
  emailLink,
  nextTotpWindow,
  seed,
  settle,
  signIn,
  signOut,
  totp,
  visual,
} from '../support/env';

// One serial story over two seeded organisations. Each person keeps one browser context for the
// whole run (sign-in is rate limited per client address, and every request from the test
// container shares one address).
test.describe.configure({ mode: 'serial' });

const { orgA, orgB } = seed;
const password = (who: string) => `${who}-Passw0rd-${seed.run}`;
const people = {
  ownerA: { email: orgA.ownerEmail, password: password('owner-a') },
  ownerB: { email: orgB.ownerEmail, password: password('owner-b') },
  member: { email: address('member'), password: password('member'), name: 'Mo Member' },
  guest: { email: address('guest'), password: password('guest'), name: 'Gus Guest' },
};
const home = (slug: string) => new RegExp(`/${slug}/home$`);

const pages = new Map<string, Page>();
async function as(browser: Browser, who: keyof typeof people): Promise<Page> {
  let page = pages.get(who);
  if (!page) {
    page = await (await browser.newContext()).newPage();
    pages.set(who, page);
  }
  return page;
}

test.afterAll(async () => {
  for (const page of pages.values()) await page.context().close();
});

async function setPasswordFromLink(page: Page, link: string, newPassword: string) {
  await page.goto(link);
  // The token is read from the fragment and removed from the address bar.
  await expect(page).toHaveURL(/\/reset-password$/);
  await page.getByLabel('New password', { exact: true }).fill(newPassword);
  await page.getByLabel('Confirm new password').fill(newPassword);
  await page.getByRole('button', { name: 'Save password' }).click();
  await expect(page).toHaveURL(/\/login\?reason=password-reset$/);
  await expect(page.locator('main').getByRole('status')).toContainText('Your password has been changed');
}

test('sign-in page', async ({ page }) => {
  // The client checks for an existing session once hydrated.
  const hydrated = page.waitForResponse((r) => r.url().endsWith('/api/v1/auth/session'));
  await page.goto('/login');
  await hydrated;
  await expect(page.getByRole('heading', { name: 'Welcome to MateCollab' })).toBeVisible();
  await expect(page.getByRole('img', { name: 'MateCollab' }).first()).toBeVisible();
  if (visual) {
    await settle(page);
    await expect(page).toHaveScreenshot('sign-in-light.png');
    await page.emulateMedia({ colorScheme: 'dark' });
    await expect(page).toHaveScreenshot('sign-in-dark.png');
  }
  // Signed-out visitors are sent to sign-in, keeping where they were going.
  await page.goto(`/${orgA.slug}/admin/members`);
  await expect(page).toHaveURL(new RegExp(`/login\\?next=%2F${orgA.slug}%2Fadmin%2Fmembers$`));
});

test('owners set a password from the setup email and sign in', async ({ browser }) => {
  const anon = await (await browser.newContext()).newPage();
  await setPasswordFromLink(
    anon,
    await emailLink(orgA.ownerEmail, 'Set up your MateCollab account'),
    people.ownerA.password,
  );
  await setPasswordFromLink(
    anon,
    await emailLink(orgB.ownerEmail, 'Set up your MateCollab account'),
    people.ownerB.password,
  );
  await anon.context().close();

  const ownerA = await as(browser, 'ownerA');
  await signIn(ownerA, people.ownerA.email, people.ownerA.password, home(orgA.slug));
  await expect(ownerA.getByRole('navigation', { name: 'Breadcrumb' })).toContainText(orgA.name);
  const ownerB = await as(browser, 'ownerB');
  await signIn(ownerB, people.ownerB.email, people.ownerB.password, home(orgB.slug));
});

test('wrong password is rejected without revealing which part was wrong', async ({ page }) => {
  await page.goto('/login');
  await page.getByLabel('Work email').fill(people.ownerA.email);
  await page.getByLabel('Password', { exact: true }).fill('not-the-password');
  await page.getByRole('button', { name: 'Continue' }).click();
  await expect(page.locator('main').getByRole('alert')).toHaveText('The email or password is incorrect.');
  await expect(page).toHaveURL(/\/login$/);
});

test('owner invites a member and a guest; they create accounts from the emails', async ({ browser }) => {
  const ownerA = await as(browser, 'ownerA');
  await ownerA.goto(`/${orgA.slug}/admin/invitations`);
  for (const [who, role] of [
    ['member', 'Member'],
    ['guest', 'Guest'],
  ] as const) {
    await ownerA.getByRole('button', { name: 'Invite people' }).click();
    await ownerA.getByLabel('Email address').fill(people[who].email);
    await ownerA.getByRole('combobox', { name: 'Role' }).click();
    await ownerA.getByRole('option', { name: role }).click();
    await ownerA.getByRole('button', { name: 'Send invitation' }).click();
    await expect(ownerA.getByText(`Invitation sent to ${people[who].email}`).first()).toBeVisible();
  }
  await expect(ownerA.getByRole('list', { name: 'Invitations' }).getByRole('listitem')).toHaveCount(2);

  for (const who of ['member', 'guest'] as const) {
    const page = await as(browser, who);
    await page.goto(await emailLink(people[who].email, 'invited you to Acme Corporation'));
    await expect(page).toHaveURL(/\/invite$/);
    await expect(page.getByRole('heading', { name: `Join ${orgA.name}` })).toBeVisible();
    if (visual && who === 'member') {
      await settle(page);
      await expect(page).toHaveScreenshot('invitation-create-account.png', {
        mask: [page.locator('strong')],
      });
    }
    await page.getByLabel('Your name').fill(people[who].name);
    await page.getByLabel('Password', { exact: true }).fill(people[who].password);
    await page.getByLabel('Confirm password').fill(people[who].password);
    await page.getByRole('button', { name: 'Create account and join' }).click();
    await expect(page).toHaveURL(home(orgA.slug));
  }

  // The same link cannot be used twice.
  const reuse = await (await browser.newContext()).newPage();
  await reuse.goto(await emailLink(people.member.email, 'invited you to Acme Corporation'));
  await expect(reuse.getByRole('heading', { name: 'Invitation already used' })).toBeVisible();
  await reuse.context().close();

  await ownerA.goto(`/${orgA.slug}/admin/members`);
  const members = ownerA.getByRole('list', { name: 'Members' });
  await expect(members.getByRole('listitem')).toHaveCount(3);
  await expect(members).toContainText('Mo Member');
  await expect(members).toContainText('Gus Guest');
  if (visual) {
    await settle(ownerA);
    // Member rows carry per-run addresses and dates; the footer shows the build version.
    await expect(ownerA).toHaveScreenshot('members.png', { mask: [members, ownerA.getByText(/^Connected/)] });
  }
});

test('a tampered invitation link is refused', async ({ page }) => {
  await page.goto('/invite#token=not-a-real-invitation-token');
  await expect(page.getByRole('heading', { name: 'This invitation can’t be used' })).toBeVisible();
});

test('guests are restricted', async ({ browser }) => {
  const guest = await as(browser, 'guest');
  await guest.goto(`/${orgA.slug}/admin/members`);
  await expect(guest.getByRole('heading', { name: 'You don’t have access to this page' })).toBeVisible();
  expect(await apiFromPage(guest, 'GET', `/orgs/${orgA.id}/members`)).toBe(403);
  expect(await apiFromPage(guest, 'GET', `/orgs/${orgA.id}/invitations`)).toBe(403);
  // No administration entries in the organisation menu.
  await guest.getByRole('button', { name: `Organisation: ${orgA.name}` }).click();
  await expect(guest.getByRole('menuitem', { name: 'Members and invitations' })).toHaveCount(0);
  await guest.keyboard.press('Escape');
});

test('members can read the directory but not administer', async ({ browser }) => {
  const member = await as(browser, 'member');
  await member.goto(`/${orgA.slug}/admin/members`);
  await expect(member.getByRole('list', { name: 'Members' }).getByRole('listitem')).toHaveCount(3);
  await expect(member.getByRole('button', { name: /^Manage / })).toHaveCount(0);
  await member.goto(`/${orgA.slug}/admin/security`);
  await expect(member.getByRole('heading', { name: 'You don’t have access to this page' })).toBeVisible();
  expect(await apiFromPage(member, 'PATCH', `/orgs/${orgA.id}`, { name: 'Taken over' })).toBe(403);
  expect(
    await apiFromPage(member, 'POST', `/orgs/${orgA.id}/invitations`, { email: address('x'), role: 'admin' }),
  ).toBe(403);
});

test('other organisations are invisible', async ({ browser }) => {
  const member = await as(browser, 'member');
  await member.goto(`/${orgB.slug}/home`);
  await expect(member.getByRole('heading', { name: 'Organisation not found' })).toBeVisible();
  for (const path of [`/orgs/${orgB.id}`, `/orgs/${orgB.id}/members`, `/orgs/${orgB.id}/invitations`]) {
    expect(await apiFromPage(member, 'GET', path), path).toBe(404);
  }
  const ownerB = await as(browser, 'ownerB');
  expect(await apiFromPage(ownerB, 'GET', `/orgs/${orgA.id}/members`)).toBe(404);
});

test('an existing account joins a second organisation and switches between them', async ({ browser }) => {
  const ownerB = await as(browser, 'ownerB');
  await ownerB.goto(`/${orgB.slug}/admin/invitations`);
  await ownerB.getByRole('button', { name: 'Invite people' }).click();
  await ownerB.getByLabel('Email address').fill(people.ownerA.email);
  await ownerB.getByRole('button', { name: 'Send invitation' }).click();
  await expect(ownerB.getByText(`Invitation sent to ${people.ownerA.email}`).first()).toBeVisible();

  const ownerA = await as(browser, 'ownerA');
  await ownerA.goto(await emailLink(people.ownerA.email, 'invited you to Globex'));
  await expect(ownerA.getByText('You’re signed in as')).toBeVisible();
  await ownerA.getByRole('button', { name: 'Accept invitation' }).click();
  await expect(ownerA).toHaveURL(home(orgB.slug));
  await expect(ownerA.getByRole('navigation', { name: 'Breadcrumb' })).toContainText(orgB.name);

  await ownerA.getByRole('button', { name: `Organisation: ${orgB.name}` }).click();
  await ownerA.getByRole('menuitem', { name: orgA.name }).click();
  await expect(ownerA).toHaveURL(home(orgA.slug));
  await expect(ownerA.getByRole('navigation', { name: 'Breadcrumb' })).toContainText(orgA.name);

  await ownerA.goto('/select-organisation');
  await expect(ownerA.getByRole('list', { name: 'Your organisations' }).getByRole('listitem')).toHaveCount(2);
  if (visual) {
    await settle(ownerA);
    await expect(ownerA).toHaveScreenshot('select-organisation.png', { mask: [ownerA.locator('strong')] });
  }
});

test('deactivated members lose access immediately', async ({ browser }) => {
  const ownerA = await as(browser, 'ownerA');
  await ownerA.goto(`/${orgA.slug}/admin/members`);
  await ownerA.getByRole('button', { name: `Manage ${people.guest.name}` }).click();
  await ownerA.getByRole('menuitem', { name: 'Deactivate' }).click();
  await ownerA.getByRole('alertdialog').getByRole('button', { name: 'Deactivate' }).click();
  await expect(ownerA.getByText(`${people.guest.name} is deactivated`).first()).toBeVisible();

  const guest = await as(browser, 'guest');
  expect(await apiFromPage(guest, 'GET', `/orgs/${orgA.id}`)).toBe(404);
  await guest.reload();
  await expect(guest.getByRole('heading', { name: 'Organisation not found' })).toBeVisible();
});

test('forgot password resets the password and signs out other sessions', async ({ browser }) => {
  const member = await as(browser, 'member');
  const anon = await (await browser.newContext()).newPage();
  const since = Date.now();
  await anon.goto('/forgot-password');
  await anon.getByLabel('Work email').fill(people.member.email);
  await anon.getByRole('button', { name: 'Send reset link' }).click();
  await expect(anon.getByRole('heading', { name: 'Check your email' })).toBeVisible();
  const link = await emailLink(people.member.email, 'Reset your MateCollab password', since);
  people.member.password = password('member-reset');
  await setPasswordFromLink(anon, link, people.member.password);

  // The old session was revoked: the member is sent back to sign in.
  await member.goto(`/${orgA.slug}/home`);
  await expect(member).toHaveURL(/\/login/);
  await signIn(member, people.member.email, people.member.password, home(orgA.slug));

  // The link works once.
  await anon.goto(link);
  await anon.getByLabel('New password', { exact: true }).fill(password('again'));
  await anon.getByLabel('Confirm new password').fill(password('again'));
  await anon.getByRole('button', { name: 'Save password' }).click();
  await expect(anon.getByRole('heading', { name: 'This link can’t be used' })).toBeVisible();
  await anon.context().close();
});

test('two-step verification: enrol, sign in with a code, then with a recovery code', async ({ browser }) => {
  const ownerA = await as(browser, 'ownerA');
  await ownerA.goto(`/${orgA.slug}/settings/security`);
  await ownerA.getByRole('button', { name: 'Turn on' }).click();
  const dialog = ownerA.getByRole('dialog', { name: 'Set up two-step verification' });
  const secret = (await dialog.locator('code').innerText()).replace(/\s+/g, '');
  await dialog.getByLabel('Authentication code').fill(totp(secret));
  await dialog.getByRole('button', { name: 'Turn on' }).click();
  const codesDialog = ownerA.getByRole('dialog', { name: 'Save your recovery codes' });
  const codeItems = codesDialog.getByRole('list', { name: 'Recovery codes' }).getByRole('listitem');
  await expect(codeItems).toHaveCount(10);
  const codes = await codeItems.allInnerTexts();
  await codesDialog.getByRole('button', { name: 'I’ve saved them' }).click();
  await expect(ownerA.getByText('On', { exact: true })).toBeVisible();

  await signOut(ownerA);
  await ownerA.getByLabel('Work email').fill(people.ownerA.email);
  await ownerA.getByLabel('Password', { exact: true }).fill(people.ownerA.password);
  await ownerA.getByRole('button', { name: 'Continue' }).click();
  await expect(ownerA).toHaveURL(/\/login\/mfa$/);
  await ownerA.getByLabel('Authentication code').fill('000000');
  await ownerA.getByRole('button', { name: 'Verify and sign in' }).click();
  await expect(ownerA.getByText('That code didn’t work')).toBeVisible();
  await nextTotpWindow(ownerA); // the enrolment code's time step cannot be reused
  await ownerA.getByLabel('Authentication code').fill(totp(secret));
  await ownerA.getByRole('button', { name: 'Verify and sign in' }).click();
  await expect(ownerA).toHaveURL(/\/select-organisation$|\/home$/);
  await expect(ownerA.getByRole('list', { name: 'Your organisations' })).toBeVisible();

  await ownerA.goto(`/${orgA.slug}/home`);
  await signOut(ownerA);
  await ownerA.getByLabel('Work email').fill(people.ownerA.email);
  await ownerA.getByLabel('Password', { exact: true }).fill(people.ownerA.password);
  await ownerA.getByRole('button', { name: 'Continue' }).click();
  await ownerA.getByRole('link', { name: 'Use a recovery code instead' }).click();
  await ownerA.getByLabel('Recovery code').fill(codes[0] ?? '');
  await ownerA.getByRole('button', { name: 'Verify and sign in' }).click();
  await expect(ownerA.getByRole('list', { name: 'Your organisations' })).toBeVisible();
});

test('signing out ends the session', async ({ browser }) => {
  const member = await as(browser, 'member');
  await member.goto(`/${orgA.slug}/home`);
  await signOut(member);
  await member.goto(`/${orgA.slug}/home`);
  await expect(member).toHaveURL(/\/login\?next=/);
  expect(await apiFromPage(member, 'GET', '/auth/session')).toBe(401);
});

test('a client cannot choose its rate-limit address with X-Forwarded-For', async ({ request, baseURL }) => {
  // Password recovery allows 5 requests per hour per client address. Each request claims a
  // different forwarded address and a different email (so only the address limit applies):
  // the edge must ignore the claims, so the budget runs out.
  const statuses: number[] = [];
  for (let i = 1; i <= 7; i++) {
    const res = await request.post('/api/v1/auth/password/forgot', {
      headers: { Origin: baseURL ?? '', 'X-MC-Client': 'web', 'X-Forwarded-For': `203.0.113.${i}` },
      data: { email: address(`spoof-${i}`) },
    });
    statuses.push(res.status());
  }
  expect(statuses).toContain(429);
});
