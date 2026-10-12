import type { ReactNode } from 'react';
import { cn } from '../lib/cn';

/**
 * Standard page container: max 1500 px; padding 35/38 px (52 px sides ≥ 1650 px, 28 px
 * ≤ 1320 px, 30 px ≤ 1100 px, 24/20 px on phones). Conversation views are full-bleed instead.
 */
export function Page({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div
      className={cn(
        'mx-auto w-full max-w-375 px-9.5 pt-8.75 pb-12 wide:px-13',
        'max-compact:p-7 max-tablet:p-7.5 max-phone:px-5 max-phone:py-6',
        className,
      )}
    >
      {children}
    </div>
  );
}

export interface PageHeaderProps {
  /** Small uppercase line above the title. */
  eyebrow?: string;
  title: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  className?: string;
}

/** Page title block (28 px heading, weight 650, tight tracking). One <h1> per page. */
export function PageHeader({ eyebrow, title, description, actions, className }: PageHeaderProps) {
  return (
    <div
      className={cn(
        'mb-7.5 flex items-center justify-between gap-6 max-phone:flex-col max-phone:items-start max-phone:gap-5',
        className,
      )}
    >
      <div className="min-w-0">
        {eyebrow ? (
          <p className="mb-2.5 text-11 font-heading tracking-eyebrow text-fg-muted uppercase">{eyebrow}</p>
        ) : null}
        <h1 className="text-28 font-heading tracking-heading max-phone:text-24">{title}</h1>
        {description ? <p className="mt-2 text-14 leading-[1.6] text-fg-muted">{description}</p> : null}
      </div>
      {actions ? <div className="flex shrink-0 flex-wrap items-center gap-2.5">{actions}</div> : null}
    </div>
  );
}

/** Section heading inside a page, with an optional action (e.g. "View all"). */
export function SectionTitle({
  title,
  action,
  className,
}: {
  title: ReactNode;
  action?: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn('mt-7.5 mb-4 flex items-center justify-between gap-3 first:mt-0', className)}>
      <h2 className="text-15 font-heading tracking-tight">{title}</h2>
      {action}
    </div>
  );
}
