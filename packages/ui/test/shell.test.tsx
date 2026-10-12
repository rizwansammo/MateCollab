import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { StatePattern, statePatterns } from '../src/patterns/state-patterns';
import { AppFrame, useShell } from '../src/shell/app-frame';
import { ContextPanel } from '../src/shell/context-panel';
import { Rail, RailItem } from '../src/shell/rail';
import { SidebarItem } from '../src/shell/sidebar';
import { setDrawerViewport } from './setup';

function Toggle() {
  const { toggleNavigation } = useShell();
  return (
    <button type="button" onClick={toggleNavigation}>
      Toggle navigation
    </button>
  );
}

function Shell({ panel = false }: { panel?: boolean }) {
  const [width, setWidth] = useState(274);
  const [open, setOpen] = useState(panel);
  return (
    <AppFrame
      sidebarLabel="Chats navigation"
      sidebarWidth={width}
      onSidebarWidthChange={setWidth}
      rail={
        <Rail>
          <RailItem href="/home" icon={<svg />} label="Home" active />
          <RailItem href="/activity" icon={<svg />} label="Activity" indicator />
        </Rail>
      }
      header={<Toggle />}
      sidebar={<SidebarItem href="/chats/general" label="general" count={3} />}
      panel={
        open ? (
          <ContextPanel title="Thread" onClose={() => setOpen(false)}>
            <button type="button">Reply</button>
          </ContextPanel>
        ) : null
      }
    >
      <p>Workspace</p>
    </AppFrame>
  );
}

describe('AppFrame', () => {
  afterEach(() => setDrawerViewport(false));

  it('exposes the landmarks, skip link and current page', () => {
    render(<Shell />);
    expect(screen.getByRole('navigation', { name: 'Primary' })).toBeInTheDocument();
    expect(screen.getByRole('complementary', { name: 'Chats navigation' })).toBeInTheDocument();
    expect(screen.getByRole('main')).toHaveTextContent('Workspace');
    expect(screen.getByRole('link', { name: 'Skip to content' })).toHaveAttribute('href', '#main-content');
    expect(screen.getByRole('link', { name: 'Home' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('link', { name: 'Activity, New activity' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'general, 3 unread' })).toBeInTheDocument();
  });

  it('collapses the sidebar column on desktop', async () => {
    render(<Shell />);
    await userEvent.click(screen.getByRole('button', { name: 'Toggle navigation' }));
    expect(screen.queryByRole('complementary', { name: 'Chats navigation' })).not.toBeInTheDocument();
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('opens the sidebar as a drawer at tablet and phone widths', async () => {
    setDrawerViewport(true);
    render(<Shell />);
    await userEvent.click(screen.getByRole('button', { name: 'Toggle navigation' }));
    expect(await screen.findByRole('dialog', { name: 'Chats navigation' })).toBeInTheDocument();
  });

  it('closes the right panel with Escape from inside it', async () => {
    render(<Shell panel />);
    const panel = screen.getByRole('complementary', { name: 'Thread' });
    screen.getByRole('button', { name: 'Reply' }).focus();
    await userEvent.keyboard('{Escape}');
    expect(panel).not.toBeInTheDocument();
  });
});

describe('State patterns', () => {
  it('defines the fifteen prototype states', () => {
    expect(statePatterns).toHaveLength(15);
    expect(new Set(statePatterns.map((p) => p.id)).size).toBe(15);
  });

  it.each(statePatterns.map((p) => [p.id]))('renders %s accessibly', (id) => {
    const onAction = vi.fn();
    const { unmount } = render(<StatePattern id={id} onAction={onAction} />);
    if (id === 'loading-messages') {
      expect(screen.getByRole('status', { name: 'Loading messages' })).toBeInTheDocument();
    } else {
      expect(screen.getByRole('heading', { level: 2 })).toBeInTheDocument();
      expect(screen.getByRole('button')).toBeInTheDocument();
    }
    unmount();
  });
});
