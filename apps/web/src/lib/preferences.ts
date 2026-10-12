// UI preferences kept in first-party cookies so the server renders the right theme and sidebar
// width on the first paint (no flash, no layout shift). They hold presentation choices only;
// once accounts exist they are mirrored from user preferences on the server (FRONTEND.md §7).

export const THEME_COOKIE = 'mc_theme';
export const SIDEBAR_WIDTH_COOKIE = 'mc_sidebar_w';

export type ThemePreference = 'light' | 'dark' | 'system';

export const themeOptions: { value: ThemePreference; label: string; description: string }[] = [
  { value: 'light', label: 'Light', description: 'Bright surfaces with navy text.' },
  { value: 'dark', label: 'Dark', description: 'Low-glare surfaces for dim rooms.' },
  { value: 'system', label: 'Match system', description: 'Follows your device setting.' },
];

export function parseTheme(value: string | null | undefined): ThemePreference {
  return value === 'light' || value === 'dark' || value === 'system' ? value : 'system';
}

export const SIDEBAR_WIDTH = { default: 274, min: 240, max: 320 } as const;

export function parseSidebarWidth(value: string | null | undefined): number {
  const width = Number(value);
  if (!Number.isFinite(width)) return SIDEBAR_WIDTH.default;
  return Math.min(SIDEBAR_WIDTH.max, Math.max(SIDEBAR_WIDTH.min, Math.round(width)));
}

/** Browser only: persist a preference for a year. */
export function writePreferenceCookie(name: string, value: string) {
  const secure = window.location.protocol === 'https:' ? '; Secure' : '';
  document.cookie = `${name}=${encodeURIComponent(value)}; Path=/; Max-Age=31536000; SameSite=Lax${secure}`;
}

/** Browser only: switch the theme immediately and remember it. */
export function applyTheme(theme: ThemePreference) {
  document.documentElement.dataset.theme = theme;
  writePreferenceCookie(THEME_COOKIE, theme);
}
