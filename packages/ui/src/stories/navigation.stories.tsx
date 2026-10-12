import type { Meta, StoryObj } from '@storybook/react-vite';
import {
  Bell,
  Bookmark,
  CircleCheck,
  Files,
  Hash,
  House,
  Layers,
  LifeBuoy,
  MessageSquare,
  Video,
} from 'lucide-react';
import { useState } from 'react';
import { ResizeHandle } from '../components/resize-handle';
import { SearchTrigger } from '../components/search-trigger';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../components/tabs';
import { Rail, RailItem } from '../shell/rail';
import { SidebarItem, SidebarSectionLabel } from '../shell/sidebar';

const meta: Meta = { title: 'Navigation/Elements' };
export default meta;
type Story = StoryObj;

export const UnderlineTabs: Story = {
  render: () => (
    <Tabs defaultValue="messages" className="max-w-xl">
      <TabsList aria-label="Conversation views">
        <TabsTrigger value="messages">Messages</TabsTrigger>
        <TabsTrigger value="files">Files</TabsTrigger>
        <TabsTrigger value="pinned">Pinned</TabsTrigger>
        <TabsTrigger value="members" disabled>
          Members
        </TabsTrigger>
      </TabsList>
      <TabsContent value="messages" className="text-13 text-fg-muted">
        Arrow keys move between tabs.
      </TabsContent>
      <TabsContent value="files" className="text-13 text-fg-muted">
        Files shared in this conversation.
      </TabsContent>
      <TabsContent value="pinned" className="text-13 text-fg-muted">
        Pinned messages.
      </TabsContent>
    </Tabs>
  ),
};

export const GlobalSearchTrigger: Story = {
  render: function Render() {
    const [count, setCount] = useState(0);
    return (
      <div className="flex max-w-lg flex-col gap-3">
        <SearchTrigger onActivate={() => setCount((c) => c + 1)} />
        <p className="text-12 text-fg-muted" aria-live="polite">
          Opened {count} time{count === 1 ? '' : 's'} (click or press Ctrl/⌘ K).
        </p>
      </div>
    );
  },
};

export const SidebarResize: Story = {
  render: function Render() {
    const [width, setWidth] = useState(274);
    return (
      <div className="flex h-60 overflow-hidden rounded-md border border-border">
        <div className="relative shrink-0 border-r border-border bg-sidebar p-4 text-13" style={{ width }}>
          {width} px — drag the edge or focus it and use ← / →
          <ResizeHandle label="Resize sidebar" value={width} min={240} max={320} onChange={setWidth} />
        </div>
        <div className="flex-1 p-4 text-13 text-fg-muted">Workspace</div>
      </div>
    );
  },
};

export const SidebarItems: Story = {
  render: () => (
    <div className="w-68.5 rounded-md border border-border bg-sidebar p-3">
      <SidebarItem href="#overview" icon={<House aria-hidden />} label="Overview" active />
      <SidebarItem href="#actions" icon={<CircleCheck aria-hidden />} label="My action items" count={3} />
      <SidebarItem href="#saved" icon={<Bookmark aria-hidden />} label="Saved for later" />
      <SidebarItem href="#support" icon={<LifeBuoy aria-hidden />} label="My support requests" />
      <SidebarSectionLabel>Favourite conversations</SidebarSectionLabel>
      <SidebarItem href="#general" icon={<Hash aria-hidden />} label="general" count={3} />
      <SidebarItem href="#devops" icon={<Hash aria-hidden />} label="devops" count={2} active={false} />
    </div>
  ),
};

const modules = [
  { id: 'home', label: 'Home', icon: <House aria-hidden /> },
  { id: 'chats', label: 'Chats', icon: <MessageSquare aria-hidden /> },
  { id: 'spaces', label: 'Spaces', icon: <Layers aria-hidden /> },
  { id: 'meetings', label: 'Meetings', icon: <Video aria-hidden /> },
  { id: 'files', label: 'Files', icon: <Files aria-hidden /> },
  { id: 'activity', label: 'Activity', icon: <Bell aria-hidden /> },
];

export const RailItems: Story = {
  parameters: { padded: false },
  render: () => (
    <div className="flex h-130">
      <Rail>
        {modules.map((m) => (
          <RailItem
            key={m.id}
            href={`#${m.id}`}
            icon={m.icon}
            label={m.label}
            active={m.id === 'home'}
            indicator={m.id === 'activity'}
          />
        ))}
      </Rail>
    </div>
  ),
};
