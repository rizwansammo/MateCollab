import type { Meta, StoryObj } from '@storybook/react-vite';
import { Avatar, AvatarGroup, presenceLabels, type Presence } from '../components/avatar';
import { Badge, Kbd, NavCount } from '../components/badge';
import { EmptyState, Progress, Skeleton, Spinner } from '../components/feedback';

const meta: Meta = { title: 'Components/Display' };
export default meta;
type Story = StoryObj;

const people = [
  'Ayesha Rahman',
  'Daniel Chen',
  'Omar Faruque',
  'Shahbaj Tamjid',
  'Sarah Islam',
  'Nadia Karim',
];

export const Avatars: Story = {
  render: () => (
    <div className="flex flex-col gap-6">
      <div className="flex items-end gap-3">
        {people.map((name, i) => (
          <Avatar key={name} name={name} size={[24, 27, 32, 35, 40, 58][i]} />
        ))}
      </div>
      <div className="flex flex-wrap items-center gap-5">
        {(Object.keys(presenceLabels) as Presence[]).map((presence, i) => {
          const name = people[i] ?? 'Team member';
          return (
            <span key={presence} className="flex items-center gap-2 text-13">
              <Avatar name={name} size={35} presence={presence} decorative />
              {name} · {presenceLabels[presence]}
            </span>
          );
        })}
      </div>
      <AvatarGroup people={people.map((name) => ({ name }))} max={3} />
    </div>
  ),
};

export const Badges: Story = {
  render: () => (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-2.5">
        <Badge>Sample</Badge>
        <Badge tone="brand">Open</Badge>
        <Badge tone="success">Resolved</Badge>
        <Badge tone="warning">Pending</Badge>
        <Badge tone="danger">Failed</Badge>
        <Badge tone="brand" caps>
          Up next
        </Badge>
        <Badge caps>Guest</Badge>
      </div>
      <div className="flex items-center gap-2.5 text-13">
        Search <Kbd>Ctrl K</Kbd> · Close <Kbd>Esc</Kbd>
      </div>
      <div className="flex w-56 flex-col gap-2 text-13">
        <span className="flex items-center gap-2">
          general <NavCount>3</NavCount>
        </span>
        <span className="flex items-center gap-2 rounded-md bg-accent px-2 py-1 font-semibold text-primary-ink">
          devops <NavCount active>2</NavCount>
        </span>
      </div>
    </div>
  ),
};

export const Feedback: Story = {
  render: () => (
    <div className="grid max-w-3xl gap-6 phone:grid-cols-2">
      <div className="rounded-md border border-border bg-surface">
        <EmptyState
          title="You’re all caught up"
          description="New updates will appear here."
          headingLevel={3}
        />
      </div>
      <div className="flex flex-col gap-4 rounded-md border border-border p-6">
        <div className="flex items-center gap-3">
          <Skeleton className="size-10 rounded-full" />
          <div className="flex-1">
            <Skeleton className="mb-2.5 h-4 w-40" />
            <Skeleton className="h-4 w-full" />
          </div>
        </div>
        <Progress value={66} label="Uploading roadmap.pdf" valueText="1.6 MB of 2.4 MB" />
        <Progress value={null} label="Preparing preview" />
        <span className="flex items-center gap-2 text-13 text-fg-muted">
          <Spinner label="Loading members" /> Loading members…
        </span>
      </div>
    </div>
  ),
};
