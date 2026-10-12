/* eslint-disable no-restricted-syntax -- Library brand values for browser metadata (manifest,
   theme-color), which cannot use CSS tokens. UI colours come from packages/ui tokens only. */

/** MateCollab-Logo-Library site.webmanifest values (ADR-008). */
export const brandValues = {
  violet: '#7047D1',
  navy: '#101A2D',
  /** Page background tokens (--color-bg) for the browser chrome theme-color. */
  lightBackground: '#FFFFFF',
  darkBackground: '#171E29',
} as const;
