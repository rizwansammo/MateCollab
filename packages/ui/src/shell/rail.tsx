'use client';

import { Slot } from 'radix-ui';
import type { ComponentProps, ReactNode } from 'react';
import { cn } from '../lib/cn';

export interface RailProps {
  /** Brand button (white Library mark), hidden on phones. */
  brand?: ReactNode;
  /** <RailItem> elements: the six modules. */
  children: ReactNode;
  /** Help, Settings and the profile menu, hidden on phones. */
  footer?: ReactNode;
  label?: string;
}

/** Primary navigation: Navy 70 px rail; a 62 px bottom bar at ≤ 760 px. */
export function Rail({ brand, children, footer, label = 'Primary' }: RailProps) {
  return (
    <nav
      aria-label={label}
      className={cn(
        'z-30 flex w-17.5 shrink-0 flex-col items-center bg-rail text-rail-fg',
        'max-phone:fixed max-phone:inset-x-0 max-phone:bottom-0 max-phone:h-15.5 max-phone:w-auto max-phone:flex-row',
        'max-phone:border-t max-phone:border-rail-line',
      )}
    >
      {brand ? (
        <div className="flex h-19.5 w-full shrink-0 items-center justify-center max-phone:hidden">
          {brand}
        </div>
      ) : null}
      <ul
        role="list"
        className="flex w-full flex-col gap-2 max-phone:h-full max-phone:flex-row max-phone:justify-around max-phone:gap-0"
      >
        {children}
      </ul>
      {footer ? (
        <div className="mt-auto flex flex-col items-center gap-4.5 py-5.5 max-phone:hidden">{footer}</div>
      ) : null}
    </nav>
  );
}

export interface RailItemProps extends Omit<ComponentProps<'a'>, 'children'> {
  icon: ReactNode;
  label: string;
  active?: boolean;
  /** Shows a dot on the icon (e.g. unread activity); describe it in indicatorLabel. */
  indicator?: boolean;
  indicatorLabel?: string;
  /** Render the single child element (e.g. a Next.js <Link>) instead of an <a>; the icon and label are placed inside it. */
  asChild?: boolean;
  children?: ReactNode;
}

/** One module in the rail / bottom navigation: icon, 11 px label, active indicator. */
export function RailItem({
  icon,
  label,
  active = false,
  indicator = false,
  indicatorLabel = 'New activity',
  asChild = false,
  className,
  children,
  ...props
}: RailItemProps) {
  const Comp = asChild ? Slot.Root : 'a';
  const content = (
    <>
      {active ? (
        <span
          aria-hidden
          className={cn(
            'absolute top-4.5 -left-2 h-6 w-0.75 rounded-r-[3px] bg-rail-indicator',
            'max-phone:top-0 max-phone:left-[calc(50%-12px)] max-phone:h-0.5 max-phone:w-6 max-phone:rounded-t-none max-phone:rounded-b-[2px]',
          )}
        />
      ) : null}
      <span className="relative inline-flex [&_svg]:size-5.25 max-phone:[&_svg]:size-5">
        {icon}
        {indicator ? (
          <span aria-hidden className="absolute -top-px -right-1 size-1.5 rounded-full bg-rail-indicator" />
        ) : null}
      </span>
      <span className="text-11 leading-none font-medium tracking-[0.2px]">{label}</span>
      {indicator ? <span className="sr-only">, {indicatorLabel}</span> : null}
    </>
  );
  return (
    <li className="flex max-phone:flex-1">
      <Comp
        aria-current={active ? 'page' : undefined}
        className={cn(
          'relative mx-2 flex h-14.75 flex-1 flex-col items-center justify-center gap-1.5 rounded-md text-rail-fg',
          'transition-colors duration-140 hover:bg-rail-hover hover:text-white',
          'aria-[current=page]:bg-rail-hover aria-[current=page]:text-rail-fg-active',
          'max-phone:mx-0 max-phone:h-full max-phone:gap-1 max-phone:rounded-none',
          className,
        )}
        {...props}
      >
        {asChild ? <Slot.Slottable>{children}</Slot.Slottable> : null}
        {content}
      </Comp>
    </li>
  );
}
