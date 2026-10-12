import { cn } from '@matecollab/ui';
import type { ReactNode } from 'react';

/** A bordered settings section: title, description, optional action and content. */
export function Panel({
  title,
  description,
  action,
  children,
  className,
  id,
}: {
  title: ReactNode;
  description?: ReactNode;
  action?: ReactNode;
  children?: ReactNode;
  className?: string;
  id?: string;
}) {
  const headingId = id ? `${id}-title` : undefined;
  return (
    <section
      aria-labelledby={headingId}
      className={cn('mt-6 rounded-md border border-border bg-card first:mt-0', className)}
    >
      <header className="flex items-start justify-between gap-4 px-5.5 py-4.5 max-phone:flex-col max-phone:px-4.5">
        <div className="min-w-0">
          <h2 id={headingId} className="text-15 font-heading tracking-tight">
            {title}
          </h2>
          {description ? <p className="mt-1 text-13 leading-[1.6] text-fg-muted">{description}</p> : null}
        </div>
        {action ? <div className="flex shrink-0 flex-wrap items-center gap-2">{action}</div> : null}
      </header>
      {children ? (
        <div className="border-t border-border px-5.5 py-4.5 max-phone:px-4.5">{children}</div>
      ) : null}
    </section>
  );
}

/** Two-column label/value list used inside panels. */
export function DetailRow({ label, children }: { label: ReactNode; children: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 border-b border-border py-3 text-13 last:border-b-0">
      <span className="text-fg-muted">{label}</span>
      <span className="min-w-0 text-right">{children}</span>
    </div>
  );
}
