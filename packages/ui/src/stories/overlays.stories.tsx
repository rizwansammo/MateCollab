import type { Meta, StoryObj } from '@storybook/react-vite';
import { CalendarDays, CircleCheck, LifeBuoy, Pencil, Trash2, Users } from 'lucide-react';
import { useEffect, useState } from 'react';
import { Button } from '../components/button';
import { ConfirmDialog, Dialog, DialogContent, DialogFooter, DialogTrigger } from '../components/dialog';
import { Drawer } from '../components/drawer';
import { Field } from '../components/field';
import { Input } from '../components/input';
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '../components/menu';
import { Toaster, toast } from '../components/toast';
import { Tooltip } from '../components/tooltip';

const meta: Meta = { title: 'Components/Overlays' };
export default meta;
type Story = StoryObj;

export const TooltipOpen: Story = {
  name: 'Tooltip',
  render: () => (
    <div className="pt-12 pl-24">
      <Tooltip content="Schedule meeting" defaultOpen>
        <Button>Hover or focus me</Button>
      </Tooltip>
    </div>
  ),
};

export const Dropdown: Story = {
  render: function Render() {
    const [presence, setPresence] = useState('available');
    return (
      <div className="h-110">
        <DropdownMenu defaultOpen modal={false}>
          <DropdownMenuTrigger asChild>
            <Button>Quick create</Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent>
            <DropdownMenuItem>
              <CalendarDays aria-hidden /> Schedule meeting
            </DropdownMenuItem>
            <DropdownMenuItem>
              <Users aria-hidden /> Create Space
            </DropdownMenuItem>
            <DropdownMenuItem>
              <LifeBuoy aria-hidden /> Support request
            </DropdownMenuItem>
            <DropdownMenuItem disabled>
              <CircleCheck aria-hidden /> New action item
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuLabel>Status</DropdownMenuLabel>
            <DropdownMenuRadioGroup value={presence} onValueChange={setPresence}>
              <DropdownMenuRadioItem value="available">Available</DropdownMenuRadioItem>
              <DropdownMenuRadioItem value="busy">Busy</DropdownMenuRadioItem>
              <DropdownMenuRadioItem value="dnd">Do not disturb</DropdownMenuRadioItem>
            </DropdownMenuRadioGroup>
            <DropdownMenuSeparator />
            <DropdownMenuItem tone="danger">
              <Trash2 aria-hidden /> Leave organisation
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    );
  },
};

export const ContextMenuExample: Story = {
  name: 'Context menu',
  render: () => (
    <ContextMenu>
      <ContextMenuTrigger className="flex h-40 max-w-md items-center justify-center rounded-md border border-dashed border-border-control text-13 text-fg-muted">
        Right-click, long-press, or focus and press Shift + F10
      </ContextMenuTrigger>
      <ContextMenuContent>
        <ContextMenuItem>
          <Pencil aria-hidden /> Edit message
        </ContextMenuItem>
        <ContextMenuItem>
          <LifeBuoy aria-hidden /> Create support request
        </ContextMenuItem>
        <ContextMenuSeparator />
        <ContextMenuItem tone="danger">
          <Trash2 aria-hidden /> Delete message
        </ContextMenuItem>
      </ContextMenuContent>
    </ContextMenu>
  ),
};

export const Modal: Story = {
  render: () => (
    <Dialog defaultOpen>
      <DialogTrigger asChild>
        <Button variant="primary">Create Space</Button>
      </DialogTrigger>
      <DialogContent
        title="Create a Space"
        description="Spaces bring a team’s conversations, files and meetings together."
      >
        <Field label="Space name" required>
          <Input placeholder="e.g. Engineering" />
        </Field>
        <DialogFooter>
          <Button>Cancel</Button>
          <Button variant="primary">Create Space</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  ),
};

export const Confirmation: Story = {
  render: function Render() {
    const [open, setOpen] = useState(true);
    return (
      <>
        <Button variant="danger" onClick={() => setOpen(true)}>
          Delete Space
        </Button>
        <ConfirmDialog
          open={open}
          onOpenChange={setOpen}
          title="Delete this Space?"
          description="Channels, files and meeting history in Engineering will be removed for everyone. This cannot be undone."
          confirmLabel="Delete Space"
          onConfirm={() => setOpen(false)}
        />
      </>
    );
  },
};

export const NavigationDrawer: Story = {
  render: function Render() {
    const [open, setOpen] = useState(true);
    return (
      <>
        <Button onClick={() => setOpen(true)}>Open navigation</Button>
        <Drawer open={open} onOpenChange={setOpen} title="Workspace navigation">
          <div className="p-4 text-13 text-fg-muted">Context sidebar content</div>
        </Drawer>
      </>
    );
  },
};

export const SideSheet: Story = {
  render: function Render() {
    const [open, setOpen] = useState(true);
    return (
      <>
        <Button onClick={() => setOpen(true)}>Open details</Button>
        <Drawer
          open={open}
          onOpenChange={setOpen}
          side="right"
          title="Details"
          description="Engineering · # general"
          showHeader
          className="max-w-95 bg-bg"
        >
          <div className="p-6 text-13 text-fg-muted">Panel content</div>
        </Drawer>
      </>
    );
  },
};

export const Toasts: Story = {
  render: function Render() {
    useEffect(() => {
      toast.success('All activity marked as read', { duration: Infinity });
      toast('Link copied', { duration: Infinity });
      toast.error('Upload failed', {
        description: 'Choose a file under your organisation’s limit.',
        duration: Infinity,
        action: { label: 'Retry', onClick: () => {} },
      });
    }, []);
    return (
      <div className="h-80">
        <Button onClick={() => toast.success('Saved')}>Show a toast</Button>
        <Toaster />
      </div>
    );
  },
};
