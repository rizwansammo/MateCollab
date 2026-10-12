import type { StorybookConfig } from '@storybook/react-vite';
import tailwindcss from '@tailwindcss/vite';
import { mergeConfig } from 'vite';

const config: StorybookConfig = {
  framework: { name: '@storybook/react-vite', options: {} },
  stories: ['../src/**/*.stories.@(ts|tsx)'],
  addons: ['@storybook/addon-docs', '@storybook/addon-a11y'],
  // Brand files synced from the Logo Library (pnpm brand:sync) are served exactly as in the app.
  staticDirs: [{ from: '../../../apps/web/public/brand', to: '/brand' }],
  core: { disableTelemetry: true, disableWhatsNewNotifications: true },
  async viteFinal(viteConfig) {
    return mergeConfig(viteConfig, {
      plugins: [tailwindcss()],
      build: {
        rollupOptions: {
          // Radix ships "use client" directives for React Server Components; irrelevant here.
          onwarn(warning, warn) {
            if (warning.code !== 'MODULE_LEVEL_DIRECTIVE') warn(warning);
          },
        },
      },
    });
  },
};

export default config;
