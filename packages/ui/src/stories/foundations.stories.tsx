import type { Meta, StoryObj } from '@storybook/react-vite';

const meta: Meta = { title: 'Foundations/Tokens' };
export default meta;
type Story = StoryObj;

// Class names are written out in full so Tailwind generates them.
const swatches: { group: string; tokens: [name: string, className: string][] }[] = [
  {
    group: 'Surfaces',
    tokens: [
      ['bg', 'bg-bg'],
      ['surface', 'bg-surface'],
      ['card', 'bg-card'],
      ['popover', 'bg-popover'],
      ['sidebar', 'bg-sidebar'],
      ['muted', 'bg-muted'],
      ['subtle', 'bg-subtle'],
      ['rail', 'bg-rail'],
    ],
  },
  {
    group: 'Text and lines',
    tokens: [
      ['fg', 'bg-fg'],
      ['fg-muted', 'bg-fg-muted'],
      ['fg-nav', 'bg-fg-nav'],
      ['border', 'bg-border'],
      ['border-control', 'bg-border-control'],
    ],
  },
  {
    group: 'Brand and interaction',
    tokens: [
      ['brand-violet', 'bg-brand-violet'],
      ['brand-navy', 'bg-brand-navy'],
      ['primary', 'bg-primary'],
      ['primary-hover', 'bg-primary-hover'],
      ['primary-control', 'bg-primary-control'],
      ['primary-ink', 'bg-primary-ink'],
      ['accent', 'bg-accent'],
      ['focus', 'bg-focus'],
    ],
  },
  {
    group: 'Status and presence',
    tokens: [
      ['danger', 'bg-danger'],
      ['success', 'bg-success'],
      ['warning', 'bg-warning'],
      ['presence-available', 'bg-presence-available'],
      ['presence-busy', 'bg-presence-busy'],
      ['presence-away', 'bg-presence-away'],
      ['presence-dnd', 'bg-presence-dnd'],
      ['presence-offline', 'bg-presence-offline'],
    ],
  },
];

export const Colors: Story = {
  render: () => (
    <div className="flex flex-col gap-8">
      {swatches.map(({ group, tokens }) => (
        <section key={group}>
          <h2 className="mb-3 text-15 font-heading">{group}</h2>
          <div className="grid grid-cols-[repeat(auto-fill,minmax(140px,1fr))] gap-3">
            {tokens.map(([name, className]) => (
              <div key={name} className="overflow-hidden rounded-md border border-border">
                <div className={`h-14 ${className}`} />
                <p className="px-2.5 py-2 font-mono text-12 text-fg">{name}</p>
              </div>
            ))}
          </div>
        </section>
      ))}
    </div>
  ),
};

const sizes: [string, string][] = [
  ['text-32 · 32 px', 'text-32 font-heading tracking-heading'],
  ['text-28 · page title', 'text-28 font-heading tracking-heading'],
  ['text-19 · dialog title', 'text-19 font-heading tracking-title'],
  ['text-16 · sidebar title', 'text-16 font-heading'],
  ['text-15 · section title', 'text-15 font-heading tracking-tight'],
  ['text-14 · body', 'text-14'],
  ['text-13 · controls and navigation', 'text-13'],
  ['text-12 · secondary', 'text-12 text-fg-muted'],
  ['text-11 · minimum (labels, badges)', 'text-11 text-fg-muted'],
];

export const Typography: Story = {
  render: () => (
    <div className="flex flex-col gap-4">
      {sizes.map(([label, className]) => (
        <p key={label} className={className}>
          {label} — A shared space. A clearer day.
        </p>
      ))}
      <p className="text-11 font-heading tracking-eyebrow text-fg-muted uppercase">
        Eyebrow · tracking 1.7 px
      </p>
      <p className="text-13 tabular-nums">Tabular figures: 09:31 · 11:30 – 12:00 · 1,425</p>
    </div>
  ),
};

export const Radii: Story = {
  render: () => (
    <div className="flex flex-wrap gap-6">
      {[
        ['4 px · tooltip, badge, kbd, checkbox', 'rounded-sm'],
        ['6 px · input, button, card, menu', 'rounded-md'],
        ['8 px · modal, sheet', 'rounded-lg'],
        ['circle · avatar, presence', 'rounded-full'],
      ].map(([label, className]) => (
        <div key={label} className="flex flex-col items-center gap-2">
          <div className={`size-20 border border-border-control bg-surface ${className}`} />
          <span className="max-w-36 text-center text-12 text-fg-muted">{label}</span>
        </div>
      ))}
    </div>
  ),
};
