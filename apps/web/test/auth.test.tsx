import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { moduleFor } from '@/components/shell/navigation';
import { safeNext } from '@/features/auth/session';
import { SignInForm } from '@/features/auth/sign-in-form';
import { orgKey, orgPath } from '@/features/org/org-context';
import { isAdministrator, roleLabel } from '@/features/org/roles';
import { ApiError, errorMessage, setCsrfToken, unwrap } from '@/lib/api';
import { formatRelative } from '@/lib/time';

describe('post-sign-in destinations', () => {
  it('accepts only same-site paths', () => {
    expect(safeNext('/acme/home')).toBe('/acme/home');
    expect(safeNext('/acme/admin/members?role=guest')).toBe('/acme/admin/members?role=guest');
    for (const bad of [
      'https://evil.example',
      '//evil.example',
      '/\\evil.example',
      'javascript:alert(1)',
      '/login',
      '',
      null,
    ]) {
      expect(safeNext(bad)).toBeNull();
    }
  });
});

describe('organisation routing', () => {
  it('builds organisation paths and scoped query keys', () => {
    expect(orgPath({ slug: 'acme' }, '/home')).toBe('/acme/home');
    expect(orgPath({ slug: 'acme' }, 'admin/members')).toBe('/acme/admin/members');
    expect(orgKey({ id: 'o1' }, 'members', { q: 'a' })).toEqual(['org', 'o1', 'members', { q: 'a' }]);
  });

  it('maps organisation-relative paths to modules', () => {
    expect(moduleFor('/admin/members')).toBe('admin');
    expect(moduleFor('/settings/security')).toBe('settings');
  });

  it('labels roles and recognises administrators', () => {
    expect(roleLabel('admin', { article: true })).toBe('an Admin');
    expect(roleLabel('member', { article: true })).toBe('a Member');
    expect(isAdministrator('owner')).toBe(true);
    expect(isAdministrator('admin')).toBe(true);
    expect(isAdministrator('member')).toBe(false);
    expect(isAdministrator('guest')).toBe(false);
  });
});

describe('API errors', () => {
  const response = (status: number, headers: Record<string, string> = {}) =>
    new Response(null, { status, headers });

  it('turns problem details into ApiError', async () => {
    const call = Promise.resolve({
      error: {
        code: 'validation_failed',
        detail: 'One or more fields are invalid.',
        errors: [{ field: 'email', code: 'invalid', message: 'Enter a valid email address.' }],
      },
      response: response(422),
    });
    const err = await unwrap(call).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).field('email')).toBe('Enter a valid email address.');
  });

  it('explains rate limits with the wait time', async () => {
    const err = await unwrap(
      Promise.resolve({
        error: { code: 'rate_limited', detail: 'Too many attempts.' },
        response: response(429, { 'Retry-After': '90' }),
      }),
    ).catch((e: unknown) => e);
    expect(errorMessage(err)).toBe('Too many attempts. Try again in 2 minutes.');
  });

  it('reports network failures without technical detail', async () => {
    const err = await unwrap(Promise.reject(new TypeError('Failed to fetch'))).catch((e: unknown) => e);
    expect((err as ApiError).code).toBe('network_error');
    expect(errorMessage(err)).not.toContain('fetch');
  });
});

describe('relative time', () => {
  it('formats past and future times', () => {
    const now = new Date('2026-10-12T12:00:00Z');
    expect(formatRelative('2026-10-12T11:59:40Z', now)).toBe('just now');
    expect(formatRelative('2026-10-12T11:55:00Z', now)).toBe('5 minutes ago');
    expect(formatRelative('2026-10-15T12:00:00Z', now)).toBe('in 3 days');
  });
});

describe('sign-in form', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    setCsrfToken(null);
  });

  it('looks up the CSRF token before the first unsafe request of a signed-in page', async () => {
    const { api, resetCsrfForTests } = await import('@/lib/api');
    resetCsrfForTests();
    const fetchMock = vi
      .spyOn(globalThis, 'fetch')
      .mockImplementation(async (input) =>
        String(input instanceof Request ? input.url : input).endsWith('/auth/session')
          ? new Response(JSON.stringify({ csrf_token: 'from-session' }), { status: 200 })
          : new Response(null, { status: 204 }),
      );
    await api.POST('/auth/logout');
    const post = fetchMock.mock.calls
      .map((c) => c[0])
      .find((r): r is Request => r instanceof Request && r.method === 'POST');
    expect(post?.headers.get('X-CSRF-Token')).toBe('from-session');
  });

  function renderForm() {
    const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
    const onSignedIn = vi.fn();
    const onMFARequired = vi.fn();
    render(
      <QueryClientProvider client={client}>
        <SignInForm onSignedIn={onSignedIn} onMFARequired={onMFARequired} />
      </QueryClientProvider>,
    );
    return { onSignedIn, onMFARequired };
  }

  it('validates before calling the API', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch');
    renderForm();
    fireEvent.click(screen.getByRole('button', { name: 'Continue' }));
    expect(await screen.findByText('Enter your work email.')).toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('sends the pre-session CSRF header and shows the server’s message', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(
        JSON.stringify({
          type: 'urn:matecollab:problem:invalid_credentials',
          title: 'Unauthorized',
          status: 401,
          code: 'invalid_credentials',
          detail: 'The email or password is incorrect.',
        }),
        { status: 401, headers: { 'Content-Type': 'application/problem+json' } },
      ),
    );
    const { onSignedIn } = renderForm();
    fireEvent.change(screen.getByLabelText('Work email'), { target: { value: 'ada@example.com' } });
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'not-the-password' } });
    fireEvent.click(screen.getByRole('button', { name: 'Continue' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('The email or password is incorrect.');
    // Signed out: the session lookup answers 401, so the login carries only the pre-session header.
    const request = fetchMock.mock.calls
      .map((c) => c[0])
      .find((r): r is Request => r instanceof Request && r.method === 'POST');
    expect(request?.headers.get('X-MC-Client')).toBe('web');
    expect(request?.headers.get('X-CSRF-Token')).toBeNull();
    await waitFor(() => expect(screen.getByLabelText('Password')).toHaveValue(''));
    expect(onSignedIn).not.toHaveBeenCalled();
  });

  it('asks for the second step when the account has two-step verification', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ mfa_required: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    const { onMFARequired } = renderForm();
    fireEvent.change(screen.getByLabelText('Work email'), { target: { value: 'ada@example.com' } });
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'correct horse battery' } });
    fireEvent.click(screen.getByRole('button', { name: 'Continue' }));
    await waitFor(() => expect(onMFARequired).toHaveBeenCalled());
  });

  it('attaches the session CSRF token to unsafe requests once signed in', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(null, { status: 204 }));
    setCsrfToken('csrf-123');
    const { api } = await import('@/lib/api');
    await api.POST('/auth/logout');
    await api.GET('/auth/session').catch(() => undefined);
    const [post, get] = fetchMock.mock.calls.map((c) => c[0] as Request);
    expect(post?.headers.get('X-CSRF-Token')).toBe('csrf-123');
    expect(get?.headers.get('X-CSRF-Token')).toBeNull();
  });
});
