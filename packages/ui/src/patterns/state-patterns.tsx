'use client';

import { Inbox, Lock, MessageSquare, TriangleAlert, Upload, WifiOff } from 'lucide-react';
import type { ReactNode } from 'react';
import { Button } from '../components/button';
import { EmptyState, Progress, Skeleton } from '../components/feedback';

// The fifteen "moments in between" from the prototype's interaction review, as reusable
// patterns. Copy keeps the prototype's voice; prototype-only references (sample tokens, fixed
// 25 MB limit) are replaced with production wording.

export type StatePatternId =
  | 'empty-chat'
  | 'no-spaces'
  | 'no-meetings'
  | 'no-files'
  | 'no-search-results'
  | 'loading-messages'
  | 'uploading-file'
  | 'upload-failure'
  | 'disconnected'
  | 'call-connection-issue'
  | 'permission-denied'
  | 'integration-error'
  | 'expired-invitation'
  | 'guest-restricted'
  | 'offline';

type Icon = 'inbox' | 'chat' | 'offline' | 'lock' | 'alert';

export interface StatePatternDefinition {
  id: StatePatternId;
  /** Name used in catalogues and pickers. */
  name: string;
  title: string;
  description: string;
  icon: Icon;
  actionLabel: string;
  /** Live-region role for states that appear while the user is working. */
  role?: 'status' | 'alert';
}

export const statePatterns: readonly StatePatternDefinition[] = [
  {
    id: 'empty-chat',
    name: 'Empty chat',
    title: 'Start the conversation',
    description: 'A good conversation starts with a simple hello.',
    icon: 'chat',
    actionLabel: 'Start conversation',
  },
  {
    id: 'no-spaces',
    name: 'No Spaces',
    title: 'Your next chapter starts here',
    description: 'Create a Space to bring your team and their work together.',
    icon: 'inbox',
    actionLabel: 'Create Space',
  },
  {
    id: 'no-meetings',
    name: 'No meetings',
    title: 'A little room in your calendar',
    description: 'Schedule a conversation when your team needs one.',
    icon: 'inbox',
    actionLabel: 'Schedule meeting',
  },
  {
    id: 'no-files',
    name: 'No files',
    title: 'A home for your shared files',
    description: 'Upload a file or attach one to a conversation.',
    icon: 'inbox',
    actionLabel: 'Upload file',
  },
  {
    id: 'no-search-results',
    name: 'No search results',
    title: 'No results found',
    description: 'Try a different phrase or search across all content.',
    icon: 'inbox',
    actionLabel: 'Clear search',
  },
  {
    id: 'loading-messages',
    name: 'Loading messages',
    title: 'Loading messages',
    description: '',
    icon: 'inbox',
    actionLabel: '',
  },
  {
    id: 'uploading-file',
    name: 'Uploading file',
    title: 'Uploading',
    description: '',
    icon: 'inbox',
    actionLabel: 'Cancel upload',
  },
  {
    id: 'upload-failure',
    name: 'Upload failure',
    title: 'Your file couldn’t be uploaded',
    description:
      'The file is larger than your organisation’s upload limit. Choose a smaller file and try again.',
    icon: 'alert',
    actionLabel: 'Choose another file',
    role: 'alert',
  },
  {
    id: 'disconnected',
    name: 'Disconnected',
    title: 'Reconnecting to your workspace',
    description: 'Your draft is still here. We’re trying to restore your connection.',
    icon: 'offline',
    actionLabel: 'Reconnect',
    role: 'status',
  },
  {
    id: 'call-connection-issue',
    name: 'Call connection issue',
    title: 'Your connection is unstable',
    description: 'You can try reconnecting or leave and join the meeting again.',
    icon: 'offline',
    actionLabel: 'Reconnect',
    role: 'status',
  },
  {
    id: 'permission-denied',
    name: 'Permission denied',
    title: 'Administrator access required',
    description: 'Only Organisation Owners and Admins can manage this area.',
    icon: 'lock',
    actionLabel: 'Back to workspace',
  },
  {
    id: 'integration-error',
    name: 'Integration error',
    title: 'We couldn’t connect to the HelpDesk',
    description: 'Check the service URL and access token, then test the connection again.',
    icon: 'alert',
    actionLabel: 'Review integration',
    role: 'alert',
  },
  {
    id: 'expired-invitation',
    name: 'Expired invitation',
    title: 'This invitation has expired',
    description: 'Ask your organisation Admin for a new invitation.',
    icon: 'lock',
    actionLabel: 'Go to sign in',
  },
  {
    id: 'guest-restricted',
    name: 'Guest-restricted content',
    title: 'This Space is private',
    description: 'Guests can only access the Spaces they have been invited to.',
    icon: 'lock',
    actionLabel: 'Back to workspace',
  },
  {
    id: 'offline',
    name: 'Offline',
    title: 'You’re offline',
    description: 'Saved content is available. New messages will sync when you reconnect.',
    icon: 'offline',
    actionLabel: 'Try again',
    role: 'status',
  },
];

const icons: Record<Icon, ReactNode> = {
  inbox: <Inbox />,
  chat: <MessageSquare />,
  offline: <WifiOff />,
  lock: <Lock />,
  alert: <TriangleAlert />,
};

export interface StatePatternProps {
  id: StatePatternId;
  onAction?: () => void;
  /** For the uploading pattern. */
  upload?: { fileName: string; loadedLabel: string; percent: number };
}

/** Renders one of the standard empty, loading and recovery states. */
export function StatePattern({ id, onAction, upload }: StatePatternProps) {
  const def = statePatterns.find((p) => p.id === id);
  if (!def) return null;

  if (id === 'loading-messages') {
    return (
      <div role="status" aria-label="Loading messages" className="flex w-4/5 flex-col gap-7.5 px-5 py-12.5">
        {[0, 1, 2].map((row) => (
          <div key={row} className="flex items-center gap-2.25">
            <Skeleton className="size-10 shrink-0 rounded-full" />
            <div className="min-w-0 flex-1">
              <Skeleton className="mb-3 h-4 w-40" />
              <Skeleton className="h-4 w-full" />
            </div>
          </div>
        ))}
      </div>
    );
  }

  if (id === 'uploading-file') {
    const file = upload ?? {
      fileName: 'infrastructure-roadmap.pdf',
      loadedLabel: '1.6 MB of 2.4 MB',
      percent: 66,
    };
    return (
      <div role="status" className="flex w-full max-w-90 flex-col items-center gap-3 px-6 py-12 text-center">
        <Upload className="size-7.5 text-fg-muted" aria-hidden />
        <h2 className="text-16 font-heading">Uploading {file.fileName}</h2>
        <p className="text-13 text-fg-muted">{file.loadedLabel}</p>
        <Progress
          value={file.percent}
          label={`Uploading ${file.fileName}`}
          valueText={file.loadedLabel}
          className="my-1"
        />
        <Button onClick={onAction}>{def.actionLabel}</Button>
      </div>
    );
  }

  return (
    <EmptyState
      role={def.role}
      icon={icons[def.icon]}
      title={def.title}
      description={def.description}
      action={
        <Button variant="primary" onClick={onAction}>
          {def.actionLabel}
        </Button>
      }
    />
  );
}
