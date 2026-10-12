'use client';

import {
  API_BASE_PATH,
  type FieldError,
  type paths,
  type ProblemDetails,
  type RuntimeConfig,
} from '@matecollab/contracts';
import createClient from 'openapi-fetch';
import { useCallback, useEffect, useState } from 'react';

// The CSRF token for the current session lives in memory only (never in storage). It comes from
// GET /auth/session and is attached to every unsafe request; pre-session endpoints are covered by
// the X-MC-Client header, which cross-site pages cannot send (API-DESIGN.md §3).
let csrfToken: string | null = null;
let csrfKnown = false;
let csrfLookup: Promise<string | null> | null = null;

/** Records the session's CSRF token (null when signed out). */
export function setCsrfToken(token: string | null) {
  csrfToken = token;
  csrfKnown = true;
}

/** Test hook: forget what is known about the session's CSRF token. */
export function resetCsrfForTests() {
  csrfToken = null;
  csrfKnown = false;
}

const SAFE_METHODS = new Set(['GET', 'HEAD', 'OPTIONS']);
const baseUrl = typeof window === 'undefined' ? API_BASE_PATH : `${window.location.origin}${API_BASE_PATH}`;

// A page can send its first unsafe request before anything loaded the session (for example the
// invitation page opened by a signed-in user). The token is then looked up once: a signed-in
// browser needs it, a signed-out one is covered by X-MC-Client.
function resolveCsrfToken(): Promise<string | null> {
  if (csrfKnown) return Promise.resolve(csrfToken);
  csrfLookup ??= globalThis
    .fetch(`${baseUrl}/auth/session`, { headers: { Accept: 'application/json' } })
    .then(async (res) => {
      if (res.ok) setCsrfToken(((await res.json()) as { csrf_token: string }).csrf_token);
      else if (res.status === 401) setCsrfToken(null);
      return csrfToken;
    })
    .catch(() => null)
    .finally(() => {
      csrfLookup = null;
    });
  return csrfLookup;
}

/** Typed client for the same-origin API; request and response types come from the OpenAPI contract. */
export const api = createClient<paths>({
  // Same origin; absolute so Request objects can be built outside a document base (tests).
  baseUrl,
  headers: { Accept: 'application/json' },
  // Resolved per call rather than captured at start-up.
  fetch: (request: Request) => globalThis.fetch(request),
});

api.use({
  async onRequest({ request }) {
    request.headers.set('X-MC-Client', 'web');
    if (!SAFE_METHODS.has(request.method)) {
      const token = await resolveCsrfToken();
      if (token) request.headers.set('X-CSRF-Token', token);
    }
    return request;
  },
});

/** A failed API call, carrying the problem details the server returned. */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly fields: FieldError[];
  readonly retryAfter: number | null;

  constructor(status: number, problem?: Partial<ProblemDetails>, retryAfter: number | null = null) {
    super(problem?.detail ?? 'Something went wrong. Try again.');
    this.name = 'ApiError';
    this.status = status;
    this.code = problem?.code ?? (status >= 500 ? 'internal_error' : 'unknown');
    this.fields = problem?.errors ?? [];
    this.retryAfter = retryAfter;
  }

  /** The message for one field, if the server rejected it. */
  field(name: string): string | undefined {
    return this.fields.find((f) => f.field === name)?.message;
  }
}

interface FetchResult<T> {
  data?: T;
  error?: unknown;
  response: Response;
}

/** Returns the data of a successful call or throws an ApiError (network failures included). */
export async function unwrap<T>(call: Promise<FetchResult<T>>): Promise<T> {
  let result: FetchResult<T>;
  try {
    result = await call;
  } catch {
    throw new ApiError(0, {
      code: 'network_error',
      detail: 'MateCollab can’t be reached. Check your connection and try again.',
    });
  }
  const { data, error, response } = result;
  if (response.ok) return data as T;
  const retry = Number(response.headers.get('Retry-After'));
  throw new ApiError(
    response.status,
    (error as Partial<ProblemDetails> | undefined) ?? undefined,
    Number.isFinite(retry) && retry > 0 ? retry : null,
  );
}

/** A user-facing sentence for any error thrown by an API call. */
export function errorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.code === 'rate_limited') {
      return err.retryAfter
        ? `Too many attempts. Try again in ${formatWait(err.retryAfter)}.`
        : 'Too many attempts. Wait a moment and try again.';
    }
    return err.message;
  }
  return 'Something went wrong. Try again.';
}

function formatWait(seconds: number): string {
  if (seconds < 60) return `${seconds} second${seconds === 1 ? '' : 's'}`;
  const minutes = Math.ceil(seconds / 60);
  return `${minutes} minute${minutes === 1 ? '' : 's'}`;
}

export type RuntimeConfigState =
  | { status: 'loading' }
  | { status: 'ready'; config: RuntimeConfig }
  | { status: 'error'; problem?: ProblemDetails };

// One request per page load, shared by every component that needs runtime configuration.
let pending: Promise<RuntimeConfigState> | null = null;

function loadRuntimeConfig(): Promise<RuntimeConfigState> {
  pending ??= api
    .GET('/config')
    .then(({ data, error }): RuntimeConfigState => {
      if (data) return { status: 'ready', config: data };
      pending = null;
      return { status: 'error', problem: error };
    })
    .catch((): RuntimeConfigState => {
      pending = null;
      return { status: 'error' };
    });
  return pending;
}

/** GET /api/v1/config with loading/error state and a retry function. */
export function useRuntimeConfig(): RuntimeConfigState & { retry: () => void } {
  const [state, setState] = useState<RuntimeConfigState>({ status: 'loading' });
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    let active = true;
    void loadRuntimeConfig().then((next) => {
      if (active) setState(next);
    });
    return () => {
      active = false;
    };
  }, [attempt]);
  const retry = useCallback(() => {
    setState({ status: 'loading' });
    setAttempt((n) => n + 1);
  }, []);
  return { ...state, retry };
}
