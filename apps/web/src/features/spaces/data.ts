'use client';

import type { Space, Channel } from '@matecollab/contracts';
import { useQuery } from '@tanstack/react-query';
import { ApiError, api, unwrap } from '@/lib/api';
import { orgKey, useOrg } from '@/features/org/org-context';

export function useSpaces() {
  const org = useOrg();
  return useQuery({
    queryKey: orgKey(org, 'spaces'),
    queryFn: async (): Promise<Space[]> =>
      (await unwrap(api.GET('/orgs/{orgId}/spaces', { params: { path: { orgId: org.id } } }))).data,
  });
}

export function useSpace(spaceId: string) {
  const org = useOrg();
  return useQuery({
    queryKey: orgKey(org, 'space', spaceId),
    queryFn: async () => {
      const { data, error, response } = await api.GET('/orgs/{orgId}/spaces/{spaceId}', {
        params: { path: { orgId: org.id, spaceId } },
      });
      if (!data) throw new ApiError(response.status, error);
      return { space: data, etag: response.headers.get('ETag') ?? '' };
    },
  });
}

export function useChannels(spaceId: string) {
  const org = useOrg();
  return useQuery({
    queryKey: orgKey(org, 'space', spaceId, 'channels'),
    queryFn: async (): Promise<Channel[]> =>
      (
        await unwrap(
          api.GET('/orgs/{orgId}/spaces/{spaceId}/channels', {
            params: { path: { orgId: org.id, spaceId } },
          }),
        )
      ).data,
    enabled: Boolean(spaceId),
  });
}
