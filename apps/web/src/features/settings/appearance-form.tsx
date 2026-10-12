'use client';

import { RadioCard, RadioCardGroup, toast } from '@matecollab/ui';
import { Monitor, Moon, Sun } from 'lucide-react';
import { useState } from 'react';
import { applyTheme, parseTheme, themeOptions, type ThemePreference } from '@/lib/preferences';

const icons = { light: <Sun aria-hidden />, dark: <Moon aria-hidden />, system: <Monitor aria-hidden /> };

// Miniature of the frame drawn with the real tokens: `data-theme` on the pane makes the tokens
// inside it resolve to that theme regardless of the page's current theme.
function Pane({ theme }: { theme: 'light' | 'dark' }) {
  return (
    <span data-theme={theme} className="flex h-full flex-1 bg-bg">
      <span className="w-1/4 bg-sidebar" />
      <span className="flex flex-1 flex-col gap-1.75 p-3.5">
        <span className="h-1.25 w-full rounded-full bg-border-control/45" />
        <span className="h-1.25 w-3/5 rounded-full bg-border-control/45" />
        <span className="h-1.25 w-4/5 rounded-full bg-border-control/45" />
      </span>
    </span>
  );
}

function ThemePreview({ theme }: { theme: ThemePreference }) {
  return (
    <span className="flex h-25 w-full">
      {theme === 'system' ? (
        <>
          <Pane theme="light" />
          <Pane theme="dark" />
        </>
      ) : (
        <Pane theme={theme} />
      )}
    </span>
  );
}

export function AppearanceForm({ initialTheme }: { initialTheme: ThemePreference }) {
  const [theme, setTheme] = useState(initialTheme);
  return (
    <section aria-labelledby="theme-heading" className="max-w-177.5">
      <h2 id="theme-heading" className="text-16 font-heading">
        Theme
      </h2>
      <p className="mt-1 text-14 text-fg-muted">Choose a comfortable view for your day.</p>
      <RadioCardGroup
        value={theme}
        aria-labelledby="theme-heading"
        onValueChange={(value) => {
          const next = parseTheme(value);
          setTheme(next);
          applyTheme(next);
          toast.success(`Theme set to ${themeOptions.find((o) => o.value === next)?.label ?? next}`);
        }}
        className="mt-6 grid-cols-3 max-phone:grid-cols-1"
      >
        {themeOptions.map((option) => (
          <RadioCard
            key={option.value}
            value={option.value}
            preview={<ThemePreview theme={option.value} />}
            label={
              <>
                {icons[option.value]}
                {option.label}
              </>
            }
          />
        ))}
      </RadioCardGroup>
      <p className="mt-6 border-t border-border pt-5 text-12 text-fg-muted">
        Applied immediately and remembered on this device. It moves to your account once sign-in is available.
      </p>
    </section>
  );
}
