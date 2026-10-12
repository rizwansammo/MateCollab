'use client';

import { Check, ChevronRight, Circle } from 'lucide-react';
import { ContextMenu as ContextPrimitive, DropdownMenu as DropdownPrimitive } from 'radix-ui';
import type { ComponentProps } from 'react';
import { cn } from '../lib/cn';

// Dropdown and context menus share one look (prototype `.mc-menu`): 220 px minimum, 6 px radius,
// 5 px padding, 13 px items with 15 px icons. Keyboard: arrows, Home/End, type-ahead, Esc.

const contentClasses =
  'z-65 min-w-55 animate-pop-in overflow-hidden rounded-md border border-border bg-popover p-1.25 text-13 text-fg shadow-popover';
const itemClasses = [
  'relative flex cursor-pointer select-none items-center gap-2.25 rounded-sm px-2.5 py-2 outline-hidden',
  'data-highlighted:bg-muted data-disabled:pointer-events-none data-disabled:opacity-50',
  '[&_svg]:shrink-0 [&_svg:not([class*=size-])]:size-3.75 [&_svg]:text-fg-muted',
].join(' ');
const dangerClasses = 'text-danger [&_svg]:text-danger data-highlighted:bg-danger-badge';
const separatorClasses = '-mx-1.25 my-1.25 h-px bg-border';
const labelClasses = 'px-2.5 pt-2 pb-1 text-11 font-heading uppercase tracking-label text-fg-muted';

type ItemTone = { tone?: 'default' | 'danger' };

// ---- Dropdown menu ----------------------------------------------------------------------

export const DropdownMenu = DropdownPrimitive.Root;
export const DropdownMenuTrigger = DropdownPrimitive.Trigger;
export const DropdownMenuGroup = DropdownPrimitive.Group;
export const DropdownMenuRadioGroup = DropdownPrimitive.RadioGroup;
export const DropdownMenuSub = DropdownPrimitive.Sub;

export function DropdownMenuContent({
  className,
  sideOffset = 6,
  align = 'start',
  ...props
}: ComponentProps<typeof DropdownPrimitive.Content>) {
  return (
    <DropdownPrimitive.Portal>
      <DropdownPrimitive.Content
        sideOffset={sideOffset}
        align={align}
        collisionPadding={8}
        className={cn(contentClasses, className)}
        {...props}
      />
    </DropdownPrimitive.Portal>
  );
}

export function DropdownMenuItem({
  className,
  tone = 'default',
  ...props
}: ComponentProps<typeof DropdownPrimitive.Item> & ItemTone) {
  return (
    <DropdownPrimitive.Item
      className={cn(itemClasses, tone === 'danger' && dangerClasses, className)}
      {...props}
    />
  );
}

export function DropdownMenuCheckboxItem({
  className,
  children,
  ...props
}: ComponentProps<typeof DropdownPrimitive.CheckboxItem>) {
  return (
    <DropdownPrimitive.CheckboxItem className={cn(itemClasses, 'pr-8', className)} {...props}>
      {children}
      <DropdownPrimitive.ItemIndicator className="absolute right-2.5 inline-flex">
        <Check className="size-4 text-primary-ink!" aria-hidden />
      </DropdownPrimitive.ItemIndicator>
    </DropdownPrimitive.CheckboxItem>
  );
}

export function DropdownMenuRadioItem({
  className,
  children,
  ...props
}: ComponentProps<typeof DropdownPrimitive.RadioItem>) {
  return (
    <DropdownPrimitive.RadioItem className={cn(itemClasses, 'pr-8', className)} {...props}>
      {children}
      <DropdownPrimitive.ItemIndicator className="absolute right-2.5 inline-flex">
        <Circle className="size-2 fill-primary-control text-primary-control!" aria-hidden />
      </DropdownPrimitive.ItemIndicator>
    </DropdownPrimitive.RadioItem>
  );
}

export function DropdownMenuLabel({ className, ...props }: ComponentProps<typeof DropdownPrimitive.Label>) {
  return <DropdownPrimitive.Label className={cn(labelClasses, className)} {...props} />;
}

export function DropdownMenuSeparator({
  className,
  ...props
}: ComponentProps<typeof DropdownPrimitive.Separator>) {
  return <DropdownPrimitive.Separator className={cn(separatorClasses, className)} {...props} />;
}

export function DropdownMenuSubTrigger({
  className,
  children,
  ...props
}: ComponentProps<typeof DropdownPrimitive.SubTrigger>) {
  return (
    <DropdownPrimitive.SubTrigger
      className={cn(itemClasses, 'data-[state=open]:bg-muted', className)}
      {...props}
    >
      {children}
      <ChevronRight className="ml-auto size-4" aria-hidden />
    </DropdownPrimitive.SubTrigger>
  );
}

export function DropdownMenuSubContent({
  className,
  ...props
}: ComponentProps<typeof DropdownPrimitive.SubContent>) {
  return (
    <DropdownPrimitive.Portal>
      <DropdownPrimitive.SubContent className={cn(contentClasses, className)} {...props} />
    </DropdownPrimitive.Portal>
  );
}

// ---- Context menu (right click / long press / Shift+F10) --------------------------------

export const ContextMenu = ContextPrimitive.Root;
export const ContextMenuTrigger = ContextPrimitive.Trigger;
export const ContextMenuGroup = ContextPrimitive.Group;

export function ContextMenuContent({ className, ...props }: ComponentProps<typeof ContextPrimitive.Content>) {
  return (
    <ContextPrimitive.Portal>
      <ContextPrimitive.Content collisionPadding={8} className={cn(contentClasses, className)} {...props} />
    </ContextPrimitive.Portal>
  );
}

export function ContextMenuItem({
  className,
  tone = 'default',
  ...props
}: ComponentProps<typeof ContextPrimitive.Item> & ItemTone) {
  return (
    <ContextPrimitive.Item
      className={cn(itemClasses, tone === 'danger' && dangerClasses, className)}
      {...props}
    />
  );
}

export function ContextMenuLabel({ className, ...props }: ComponentProps<typeof ContextPrimitive.Label>) {
  return <ContextPrimitive.Label className={cn(labelClasses, className)} {...props} />;
}

export function ContextMenuSeparator({
  className,
  ...props
}: ComponentProps<typeof ContextPrimitive.Separator>) {
  return <ContextPrimitive.Separator className={cn(separatorClasses, className)} {...props} />;
}
