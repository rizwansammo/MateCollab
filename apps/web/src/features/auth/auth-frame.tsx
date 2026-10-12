import { BrandLockup, cn } from '@matecollab/ui';
import Link from 'next/link';
import type { ReactNode } from 'react';

// Sign-in, recovery, verification and invitation pages share the prototype's sign-in
// composition (MateCollab-UI `.login-page`): brand bar, a story column on large screens and a
// focused form column. The prototype's faded mark motif is omitted: the Logo Library rules forbid
// altering the mark's colour or opacity (docs/BRANDING-ASSETS.md).
export function AuthFrame({ children }: { children: ReactNode }) {
  return (
    <div className="min-h-dvh bg-bg text-fg">
      <header className="flex h-26.25 items-center px-[6vw] max-phone:h-21.25 max-phone:px-5">
        <Link href="/login" aria-label="MateCollab sign in" className="rounded-md">
          <BrandLockup height={30} alt="MateCollab" className="max-phone:h-6.5 max-phone:w-auto" />
        </Link>
      </header>
      <div className="grid min-h-[calc(100dvh-6.5625rem)] grid-cols-[1.1fr_1fr] max-phone:min-h-[calc(100dvh-5.3125rem)] max-phone:grid-cols-1">
        <section
          aria-label="About MateCollab"
          className="relative overflow-hidden border-t border-r border-border bg-surface px-[8vw] pt-17.5 pb-10 max-tablet:px-[6vw] max-phone:hidden"
        >
          <p className="mb-2.5 text-11 font-heading tracking-eyebrow text-fg-muted">WORK BETTER, TOGETHER.</p>
          <h1 className="my-6.25 text-display font-strong tracking-display max-tablet:text-display-sm">
            A shared space.
            <br />A clearer day.
          </h1>
          <p className="text-16 leading-[1.9] text-fg-muted">
            Conversations, meetings and teamwork.
            <br />
            Together in one focused workspace.
          </p>
          <small className="absolute bottom-8.75 left-[8vw] text-11 text-fg-muted max-tablet:left-[6vw]">
            A NetaMate Solutions product
          </small>
        </section>
        <main className="my-12.5 w-full max-w-102.5 self-center justify-self-center px-3.75 max-tablet:max-w-87.5 max-phone:my-10 max-phone:max-w-107.5 max-phone:self-start max-phone:px-6.25">
          {children}
        </main>
      </div>
    </div>
  );
}

/** Heading and lead text of an auth form (prototype `.login-form h2` / `> p`). */
export function AuthHeading({ title, children }: { title: ReactNode; children?: ReactNode }) {
  return (
    <div className="mb-8">
      <h2 className="mb-3 text-28 font-heading tracking-heading max-phone:text-24">{title}</h2>
      {children ? <div className="text-14 text-fg-muted">{children}</div> : null}
    </div>
  );
}

/** Centered secondary link under a form (prototype `.text-link`). */
export function AuthLink({ href, children }: { href: string; children: ReactNode }) {
  return (
    <Link
      href={href as never}
      className="mx-auto my-5.5 block w-fit text-13 font-strong text-primary-ink underline-offset-4 hover:underline"
    >
      {children}
    </Link>
  );
}

/** Small reassurance line at the bottom of a form (prototype `.footnote`). */
export function AuthFootnote({
  icon,
  children,
  className,
}: {
  icon?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <p
      className={cn(
        'mt-6.75 flex items-center justify-center gap-1.75 text-center text-11 leading-[1.7] text-fg-muted [&_svg]:size-3.5',
        className,
      )}
    >
      {icon}
      {children}
    </p>
  );
}

/** Inline alert for form-level errors and confirmations. */
export function FormNotice({
  tone = 'danger',
  children,
}: {
  tone?: 'danger' | 'success' | 'info';
  children: ReactNode;
}) {
  return (
    <div
      role={tone === 'danger' ? 'alert' : 'status'}
      className={cn(
        'mb-5 rounded-md border px-3.5 py-3 text-13',
        tone === 'danger' && 'border-danger/30 bg-danger-badge text-danger-badge-fg',
        tone === 'success' && 'border-success/30 bg-success-bg text-success',
        tone === 'info' && 'border-org-line bg-accent text-accent-strong-fg',
      )}
    >
      {children}
    </div>
  );
}
