import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { Avatar, initialsOf, toneOf } from '../src/components/avatar';
import { Button } from '../src/components/button';
import { Checkbox, Switch } from '../src/components/choice';
import { Dialog, DialogContent, DialogTrigger } from '../src/components/dialog';
import { Field } from '../src/components/field';
import { IconButton } from '../src/components/icon-button';
import { Input } from '../src/components/input';
import { ResizeHandle } from '../src/components/resize-handle';
import { SearchTrigger } from '../src/components/search-trigger';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../src/components/tabs';
import { Toaster, toast } from '../src/components/toast';

describe('Button', () => {
  it('defaults to type="button" so it never submits a form by accident', () => {
    render(<Button>Save</Button>);
    expect(screen.getByRole('button', { name: 'Save' })).toHaveAttribute('type', 'button');
  });

  it('blocks clicks and announces busy while loading', async () => {
    const onClick = vi.fn();
    render(
      <Button loading onClick={onClick}>
        Save
      </Button>,
    );
    const button = screen.getByRole('button', { name: 'Save' });
    expect(button).toBeDisabled();
    expect(button).toHaveAttribute('aria-busy', 'true');
    await userEvent.click(button);
    expect(onClick).not.toHaveBeenCalled();
  });

  it('renders a link with button styling via asChild', () => {
    render(
      <Button asChild>
        <a href="/files">Open files</a>
      </Button>,
    );
    expect(screen.getByRole('link', { name: 'Open files' })).toHaveClass('rounded-md');
  });
});

describe('IconButton', () => {
  it('uses its label as the accessible name and shows it as a tooltip on focus', async () => {
    render(
      <IconButton label="Quick create">
        <svg />
      </IconButton>,
    );
    await userEvent.tab();
    expect(screen.getByRole('button', { name: 'Quick create' })).toHaveFocus();
    expect(await screen.findByRole('tooltip')).toHaveTextContent('Quick create');
  });

  it('announces toggle state', () => {
    render(
      <IconButton label="Grid view" pressed>
        <svg />
      </IconButton>,
    );
    expect(screen.getByRole('button', { name: 'Grid view' })).toHaveAttribute('aria-pressed', 'true');
  });
});

describe('Field', () => {
  it('labels the control and links its hint and error', () => {
    render(
      <Field label="Work email" description="We send invitations here." error="Enter a valid email." required>
        <Input />
      </Field>,
    );
    const input = screen.getByLabelText(/Work email/);
    expect(input).toBeRequired();
    expect(input).toHaveAttribute('aria-invalid', 'true');
    expect(input).toHaveAccessibleDescription('Enter a valid email. We send invitations here.');
  });

  it('wires Radix controls too', () => {
    render(
      <Field label="Desktop notifications">
        <Switch />
      </Field>,
    );
    expect(screen.getByRole('switch', { name: 'Desktop notifications' })).toBeInTheDocument();
  });
});

describe('Choice controls', () => {
  it('toggles a checkbox with the keyboard', async () => {
    render(<Checkbox aria-label="Include previews" />);
    const box = screen.getByRole('checkbox', { name: 'Include previews' });
    box.focus();
    await userEvent.keyboard(' ');
    expect(box).toHaveAttribute('data-state', 'checked');
  });
});

describe('Avatar', () => {
  it('derives initials from the first and last word', () => {
    expect(initialsOf('Ayesha Rahman')).toBe('AR');
    expect(initialsOf('Omar Al Faruque')).toBe('OF');
    expect(initialsOf('Daniel')).toBe('D');
    expect(initialsOf('  ')).toBe('?');
  });

  it('keeps a stable tone for the same seed', () => {
    expect(toneOf('user-1')).toBe(toneOf('user-1'));
    expect(toneOf('user-1')).toBeGreaterThanOrEqual(1);
    expect(toneOf('user-1')).toBeLessThanOrEqual(5);
  });

  it('names the person and their presence', () => {
    render(<Avatar name="Daniel Chen" presence="dnd" />);
    expect(screen.getByRole('img', { name: 'Daniel Chen, Do not disturb' })).toBeInTheDocument();
  });
});

describe('Tabs', () => {
  it('moves between tabs with arrow keys', async () => {
    render(
      <Tabs defaultValue="messages">
        <TabsList aria-label="Views">
          <TabsTrigger value="messages">Messages</TabsTrigger>
          <TabsTrigger value="files">Files</TabsTrigger>
        </TabsList>
        <TabsContent value="messages">Message list</TabsContent>
        <TabsContent value="files">File list</TabsContent>
      </Tabs>,
    );
    screen.getByRole('tab', { name: 'Messages' }).focus();
    await userEvent.keyboard('{ArrowRight}');
    expect(screen.getByRole('tab', { name: 'Files' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('tabpanel')).toHaveTextContent('File list');
  });
});

describe('Dialog', () => {
  it('moves focus inside, closes on Escape and restores focus to the trigger', async () => {
    render(
      <Dialog>
        <DialogTrigger asChild>
          <Button>Create Space</Button>
        </DialogTrigger>
        <DialogContent title="Create a Space" description="Name your Space.">
          <Input aria-label="Space name" />
        </DialogContent>
      </Dialog>,
    );
    const trigger = screen.getByRole('button', { name: 'Create Space' });
    await userEvent.click(trigger);
    const dialog = await screen.findByRole('dialog', { name: 'Create a Space' });
    expect(dialog).toHaveAccessibleDescription('Name your Space.');
    expect(dialog.contains(document.activeElement)).toBe(true);
    await userEvent.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(trigger).toHaveFocus();
  });
});

describe('ResizeHandle', () => {
  function Harness() {
    const [width, setWidth] = useState(274);
    return <ResizeHandle label="Resize sidebar" value={width} min={240} max={320} onChange={setWidth} />;
  }

  it('resizes with arrow keys within bounds and jumps with Home/End', async () => {
    render(<Harness />);
    const handle = screen.getByRole('separator', { name: 'Resize sidebar' });
    handle.focus();
    await userEvent.keyboard('{ArrowRight}');
    expect(handle).toHaveAttribute('aria-valuenow', '284');
    await userEvent.keyboard('{End}{ArrowRight}');
    expect(handle).toHaveAttribute('aria-valuenow', '320');
    await userEvent.keyboard('{Home}{ArrowLeft}');
    expect(handle).toHaveAttribute('aria-valuenow', '240');
  });
});

describe('SearchTrigger', () => {
  it('opens on click and on Ctrl/⌘ K', async () => {
    const onActivate = vi.fn();
    render(<SearchTrigger onActivate={onActivate} />);
    await userEvent.click(screen.getByRole('button', { name: /Search anything/ }));
    await userEvent.keyboard('{Control>}k{/Control}');
    await userEvent.keyboard('{Meta>}k{/Meta}');
    expect(onActivate).toHaveBeenCalledTimes(3);
  });
});

describe('Toast', () => {
  it('shows queued notifications in the toaster', async () => {
    render(<Toaster />);
    toast.success('Settings saved', { description: 'Your theme was updated.' });
    expect(await screen.findByText('Settings saved')).toBeInTheDocument();
    expect(screen.getByText('Your theme was updated.')).toBeInTheDocument();
  });
});
