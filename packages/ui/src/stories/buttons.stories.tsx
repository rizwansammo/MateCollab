import type { Meta, StoryObj } from '@storybook/react-vite';
import { Bell, MessageSquare, Plus, Settings, Video } from 'lucide-react';
import { Button } from '../components/button';
import { IconButton } from '../components/icon-button';

const meta: Meta<typeof Button> = {
  title: 'Components/Button',
  component: Button,
  args: { children: 'Start meeting', variant: 'primary' },
};
export default meta;
type Story = StoryObj<typeof Button>;

export const Playground: Story = {};

export const Variants: Story = {
  render: () => (
    <div className="flex flex-wrap items-center gap-3">
      <Button variant="primary">
        <Video aria-hidden /> Start meeting
      </Button>
      <Button variant="secondary">
        <MessageSquare aria-hidden /> New message
      </Button>
      <Button variant="ghost">Cancel</Button>
      <Button variant="danger">Delete Space</Button>
      <Button variant="link">View all activity</Button>
    </div>
  ),
};

export const Sizes: Story = {
  render: () => (
    <div className="flex flex-wrap items-center gap-3">
      <Button variant="primary" size="sm">
        Small · 31 px
      </Button>
      <Button variant="primary" size="md">
        Default · 35 px
      </Button>
      <Button variant="primary" size="lg">
        Large · 38 px
      </Button>
    </div>
  ),
};

export const States: Story = {
  render: () => (
    <div className="flex flex-wrap items-center gap-3">
      <Button variant="primary" disabled>
        Disabled
      </Button>
      <Button variant="secondary" disabled>
        Disabled
      </Button>
      <Button variant="primary" loading>
        Saving
      </Button>
      <Button variant="secondary" asChild>
        <a href="#docs">Link styled as button</a>
      </Button>
    </div>
  ),
};

export const IconButtons: Story = {
  render: () => (
    <div className="flex flex-col gap-5">
      <div className="flex items-center gap-3">
        <IconButton label="Quick create">
          <Plus aria-hidden />
        </IconButton>
        <IconButton label="Notifications">
          <Bell aria-hidden />
        </IconButton>
        <IconButton label="List view" selected>
          <Settings aria-hidden />
        </IconButton>
        <IconButton label="Disabled action" disabled>
          <Settings aria-hidden />
        </IconButton>
        <IconButton label="Small" size="sm">
          <Plus aria-hidden />
        </IconButton>
      </div>
      <div className="flex w-fit items-center gap-3 rounded-md bg-rail p-3">
        <IconButton label="Help and keyboard shortcuts" tone="rail">
          <Bell aria-hidden />
        </IconButton>
        <IconButton label="Settings" tone="rail">
          <Settings aria-hidden />
        </IconButton>
      </div>
    </div>
  ),
};
