import '@fontsource-variable/inter';
import './globals.css';
import { Toaster, TooltipProvider } from '@matecollab/ui';
import type { Metadata, Viewport } from 'next';
import { cookies } from 'next/headers';
import type { ReactNode } from 'react';
import { brandValues } from '@/lib/brand-values';
import { THEME_COOKIE, parseTheme } from '@/lib/preferences';

export const metadata: Metadata = {
  title: { default: 'MateCollab', template: '%s · MateCollab' },
  description: 'Conversations, meetings and teamwork, together in one focused workspace.',
  applicationName: 'MateCollab',
  robots: { index: false, follow: false },
};

export const viewport: Viewport = {
  colorScheme: 'light dark',
  themeColor: [
    { media: '(prefers-color-scheme: light)', color: brandValues.lightBackground },
    { media: '(prefers-color-scheme: dark)', color: brandValues.darkBackground },
  ],
};

// The theme preference comes from a cookie, so the server renders the right `data-theme` and
// "system" resolves in CSS (light-dark()): the first paint is already correct.
export default async function RootLayout({ children }: { children: ReactNode }) {
  const theme = parseTheme((await cookies()).get(THEME_COOKIE)?.value);
  return (
    <html lang="en-GB" data-theme={theme}>
      <body>
        <TooltipProvider>{children}</TooltipProvider>
        <Toaster />
      </body>
    </html>
  );
}
