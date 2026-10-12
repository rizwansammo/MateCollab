'use client';

import { API_BASE_PATH, type paths, type ProblemDetails, type RuntimeConfig } from '@matecollab/contracts';
import createClient from 'openapi-fetch';
import { useCallback, useEffect, useState } from 'react';

/** Typed client for the same-origin API; request and response types come from the OpenAPI contract. */
export const api = createClient<paths>({ baseUrl: API_BASE_PATH, headers: { Accept: 'application/json' } });

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
