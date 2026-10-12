import { EmptyState, Page, PageHeader, SectionTitle } from '@matecollab/ui';
import type { ReactNode } from 'react';

export interface PlaceholderSection {
  title: string;
  icon?: ReactNode;
  emptyTitle: string;
  emptyDescription: string;
}

/**
 * Module page before its feature exists: the real page header and layout with honest empty
 * states. No sample data and no actions that would pretend to work.
 */
export function ModulePlaceholder({
  eyebrow,
  title,
  description,
  sections,
}: {
  eyebrow?: string;
  title: string;
  description: string;
  sections: PlaceholderSection[];
}) {
  return (
    <Page>
      <PageHeader eyebrow={eyebrow} title={title} description={description} />
      {sections.map((section) => (
        <section key={section.title} aria-label={section.title} className="mt-7.5 first:mt-0">
          <SectionTitle title={section.title} />
          <div className="rounded-md border border-border bg-surface">
            <EmptyState
              icon={section.icon}
              title={section.emptyTitle}
              description={section.emptyDescription}
              headingLevel={3}
            />
          </div>
        </section>
      ))}
    </Page>
  );
}
