'use client';

import { Hash, Layers3, LockKeyhole } from 'lucide-react';
import type { Route } from 'next';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { orgPath, useOrg } from '@/features/org/org-context';
import { useChannels, useSpaces } from './data';

export function SpacesSidebarTree() {
  const org = useOrg();
  const spaces = useSpaces();
  const pathname = usePathname();
  if (!spaces.data?.some((space) => space.joined)) return null;
  return (
    <div className="mt-5 border-t border-border pt-4">
      <h2 className="px-2.5 pb-2 text-11 font-heading tracking-wide text-fg-muted uppercase">My Spaces</h2>
      <div className="flex flex-col gap-1">
        {spaces.data
          .filter((space) => space.joined)
          .map((space) => (
            <div key={space.id}>
              <Link
                href={orgPath(org, `/spaces/${space.id}`) as Route}
                aria-current={pathname === orgPath(org, `/spaces/${space.id}`) ? 'page' : undefined}
                className="flex min-w-0 items-center gap-2 rounded-sm px-3 py-2 text-13 text-fg-muted hover:bg-hover hover:text-fg aria-[current=page]:bg-primary/10 aria-[current=page]:text-primary-ink"
              >
                <Layers3 className="size-3.5 shrink-0" aria-hidden />
                <span className="truncate">{space.name}</span>
              </Link>
              <SidebarChannels spaceId={space.id} />
            </div>
          ))}
      </div>
    </div>
  );
}

function SidebarChannels({ spaceId }: { spaceId: string }) {
  const org = useOrg();
  const pathname = usePathname();
  const channels = useChannels(spaceId);
  if (!channels.data) return null;
  return (
    <div className="ml-3 flex flex-col border-l border-border pl-2">
      {channels.data.map((channel) => {
        const href = orgPath(org, `/spaces/${spaceId}/channels/${channel.id}`);
        return (
          <Link
            key={channel.id}
            href={href as Route}
            aria-current={pathname === href ? 'page' : undefined}
            className="flex min-w-0 items-center gap-2 rounded-sm px-2.5 py-1.5 text-12 text-fg-muted hover:bg-hover hover:text-fg aria-[current=page]:bg-primary/10 aria-[current=page]:text-primary-ink"
          >
            {channel.visibility === 'private' ? (
              <LockKeyhole className="size-3.5 shrink-0" aria-hidden />
            ) : (
              <Hash className="size-3.5 shrink-0" aria-hidden />
            )}
            <span className="truncate">{channel.name}</span>
          </Link>
        );
      })}
    </div>
  );
}
