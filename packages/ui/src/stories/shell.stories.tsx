import type { Meta, StoryObj } from '@storybook/react-vite';
import {
  Bell,
  Bookmark,
  CircleCheck,
  CircleHelp,
  Files,
  Hash,
  House,
  Layers,
  LifeBuoy,
  MessageSquare,
  PanelLeftClose,
  Plus,
  Settings,
  Video,
} from 'lucide-react';
import { useState } from 'react';
import { Avatar } from '../components/avatar';
import { BrandLockup, BrandMark } from '../components/brand';
import { Button } from '../components/button';
import { EmptyState } from '../components/feedback';
import { IconButton } from '../components/icon-button';
import { SearchTrigger } from '../components/search-trigger';
import { AppFrame, SIDEBAR_DEFAULT_WIDTH, useShell } from '../shell/app-frame';
import { ContextPanel } from '../shell/context-panel';
import { Breadcrumb, GlobalBar } from '../shell/global-bar';
import { Page, PageHeader, SectionTitle } from '../shell/page';
import { Rail, RailItem } from '../shell/rail';
import { OrgSwitcherButton, Sidebar, SidebarItem, SidebarSectionLabel, SidebarTitle } from '../shell/sidebar';

// Reference composition of the application frame at the prototype's breakpoints. apps/web
// composes the same parts with real routes; these stories are its visual baseline.

const modules = [
  { id: 'home', label: 'Home', icon: <House aria-hidden /> },
  { id: 'chats', label: 'Chats', icon: <MessageSquare aria-hidden /> },
  { id: 'spaces', label: 'Spaces', icon: <Layers aria-hidden /> },
  { id: 'meetings', label: 'Meetings', icon: <Video aria-hidden /> },
  { id: 'files', label: 'Files', icon: <Files aria-hidden /> },
  { id: 'activity', label: 'Activity', icon: <Bell aria-hidden /> },
];

function NavToggle() {
  const { toggleNavigation, sidebarCollapsed } = useShell();
  return (
    <IconButton label={sidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'} onClick={toggleNavigation}>
      <PanelLeftClose aria-hidden />
    </IconButton>
  );
}

function ExampleShell({ withPanel = false }: { withPanel?: boolean }) {
  const [width, setWidth] = useState(SIDEBAR_DEFAULT_WIDTH);
  const [panelOpen, setPanelOpen] = useState(withPanel);
  return (
    <AppFrame
      sidebarLabel="Home navigation"
      sidebarWidth={width}
      onSidebarWidthChange={setWidth}
      rail={
        <Rail
          brand={
            <a href="#home" aria-label="MateCollab home" className="rounded-md p-1.5">
              <BrandMark variant="white" size={28} alt="" />
            </a>
          }
          footer={
            <>
              <IconButton label="Help and keyboard shortcuts" tone="rail" size="lg" tooltipSide="right">
                <CircleHelp aria-hidden />
              </IconButton>
              <IconButton label="Settings" tone="rail" size="lg" tooltipSide="right">
                <Settings aria-hidden />
              </IconButton>
              <Avatar
                name="Ayesha Rahman"
                size={35}
                presence="available"
                presenceRingClassName="border-rail"
              />
            </>
          }
        >
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
      }
      header={
        <GlobalBar
          start={
            <>
              <NavToggle />
              <Breadcrumb items={[{ label: 'NetaMate Solutions' }, { label: 'Home', current: true }]} />
            </>
          }
          center={<SearchTrigger onActivate={() => {}} shortcut={false} />}
          end={
            <>
              <IconButton label="Quick create">
                <Plus aria-hidden />
              </IconButton>
              <IconButton label="Activity notifications" className="max-phone:hidden">
                <Bell aria-hidden />
              </IconButton>
            </>
          }
        />
      }
      sidebar={
        <Sidebar
          header={<OrgSwitcherButton name="NetaMate Solutions" subtitle="Business workspace" />}
          footer={
            <>
              <p className="mb-4 flex items-center gap-1.75 text-11 text-fg-muted">
                <span aria-hidden className="size-1.25 rounded-full bg-presence-available" /> All systems
                connected
              </p>
              <BrandLockup height={20} />
            </>
          }
        >
          <SidebarTitle>Your workspace</SidebarTitle>
          <SidebarItem href="#overview" icon={<House aria-hidden />} label="Overview" active />
          <SidebarItem href="#actions" icon={<CircleCheck aria-hidden />} label="My action items" />
          <SidebarItem href="#saved" icon={<Bookmark aria-hidden />} label="Saved for later" />
          <SidebarItem href="#support" icon={<LifeBuoy aria-hidden />} label="My support requests" />
          <SidebarSectionLabel>Favourite conversations</SidebarSectionLabel>
          <SidebarItem href="#general" icon={<Hash aria-hidden />} label="general" />
        </Sidebar>
      }
      panel={
        panelOpen ? (
          <ContextPanel
            title="Thread"
            description="# general · Engineering"
            onClose={() => setPanelOpen(false)}
          >
            <EmptyState
              title="No replies yet"
              description="Replies to this message appear here."
              headingLevel={3}
            />
          </ContextPanel>
        ) : null
      }
    >
      <Page>
        <PageHeader
          eyebrow="Your workspace, in focus"
          title="Good morning, Ayesha."
          description="A little clarity before the day gets busy."
          actions={
            <>
              <Button>
                <MessageSquare aria-hidden /> New message
              </Button>
              <Button variant="primary" onClick={() => setPanelOpen(true)}>
                <Video aria-hidden /> Start meeting
              </Button>
            </>
          }
        />
        <SectionTitle
          title="Pick up where you left off"
          action={<Button variant="link">View chats</Button>}
        />
        <div className="rounded-md border border-border">
          <EmptyState
            icon={<MessageSquare />}
            title="Start the conversation"
            description="A good conversation starts with a simple hello."
            headingLevel={3}
          />
        </div>
      </Page>
    </AppFrame>
  );
}

const meta: Meta = {
  title: 'Shell/App frame',
  parameters: { padded: false },
  render: () => <ExampleShell />,
};
export default meta;
type Story = StoryObj;

export const Desktop: Story = { tags: ['vp-1440'] };
export const CompactDesktop: Story = { tags: ['vp-1280'] };
export const Tablet: Story = { tags: ['vp-1024'] };
export const Phone: Story = { tags: ['vp-390'] };
export const DockedPanel: Story = { tags: ['vp-1440'], render: () => <ExampleShell withPanel /> };
export const OverlayPanel: Story = { tags: ['vp-1280'], render: () => <ExampleShell withPanel /> };
