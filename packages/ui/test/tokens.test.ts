// Guards the design tokens: every text pairing meets WCAG 2.2 AA (4.5:1) and every control
// boundary or state colour meets 3:1, in both themes. Values are read from theme.css itself.
import { readFileSync } from 'node:fs';
import path from 'node:path';

const css = readFileSync(path.resolve(__dirname, '../src/styles/theme.css'), 'utf8');

type Theme = 'light' | 'dark';
const tokens = new Map<string, Record<Theme, string>>();
for (const match of css.matchAll(/--color-([a-z0-9-]+):\s*([^;]+);/g)) {
  const [, name, raw] = match as unknown as [string, string, string];
  const value = raw.trim();
  const [, light, dark] = /^light-dark\((#[0-9a-f]{6}),\s*(#[0-9a-f]{6})\)$/i.exec(value) ?? [];
  if (light && dark) tokens.set(name, { light, dark });
  else if (/^#[0-9a-f]{6}$/i.test(value)) tokens.set(name, { light: value, dark: value });
}

function luminance(hex: string): number {
  const channels = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255);
  const [r, g, b] = channels.map((c) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4)) as [
    number,
    number,
    number,
  ];
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function contrast(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x) as [number, number];
  return (hi + 0.05) / (lo + 0.05);
}

function color(name: string, theme: Theme): string {
  const value = tokens.get(name)?.[theme];
  if (!value) throw new Error(`token --color-${name} not found or not a plain colour`);
  return value;
}

const textPairs: [fg: string, bg: string][] = [
  ['fg', 'bg'],
  ['fg', 'sidebar'],
  ['fg', 'card'],
  ['fg', 'popover'],
  ['fg-muted', 'bg'],
  ['fg-muted', 'sidebar'],
  ['fg-muted', 'surface'],
  ['fg-muted', 'muted'],
  ['fg-muted', 'popover'],
  ['fg-nav', 'sidebar'],
  ['primary-ink', 'bg'],
  ['primary-ink', 'sidebar'],
  ['primary-ink', 'accent'],
  ['danger', 'bg'],
  ['danger-badge-fg', 'danger-badge'],
  ['success', 'success-bg'],
  ['warning', 'warning-bg'],
  ['nav-count-fg', 'nav-count'],
  ['accent-strong-fg', 'accent-strong'],
  ['org-fg', 'org'],
  ['tooltip-fg', 'tooltip'],
  ['white', 'primary'],
  ['white', 'primary-hover'],
  ['white', 'danger-solid'],
  ['white', 'danger-solid-hover'],
  ['rail-fg', 'rail'],
  ['rail-fg-active', 'rail'],
  ['tone-1-fg', 'tone-1'],
  ['tone-2-fg', 'tone-2'],
  ['tone-3-fg', 'tone-3'],
  ['tone-4-fg', 'tone-4'],
  ['tone-5-fg', 'tone-5'],
];

const controlPairs: [fg: string, bg: string][] = [
  ['border-control', 'bg'],
  ['switch-off', 'bg'],
  ['primary-control', 'bg'],
  ['primary-control', 'white'],
  ['focus', 'bg'],
  ['focus', 'sidebar'],
  ['rail-indicator', 'rail'],
];

describe.each(['light', 'dark'] as const)('%s theme tokens', (theme) => {
  it.each(textPairs)('text %s on %s reaches 4.5:1', (fg, bg) => {
    expect(contrast(color(fg, theme), color(bg, theme))).toBeGreaterThanOrEqual(4.5);
  });

  it.each(controlPairs)('control %s on %s reaches 3:1', (fg, bg) => {
    expect(contrast(color(fg, theme), color(bg, theme))).toBeGreaterThanOrEqual(3);
  });
});

it('uses the Logo Library brand values', () => {
  expect(color('brand-violet', 'light').toLowerCase()).toBe('#7047d1');
  expect(color('brand-navy', 'light').toLowerCase()).toBe('#101a2d');
  expect(color('rail', 'dark').toLowerCase()).toBe('#101a2d');
});
