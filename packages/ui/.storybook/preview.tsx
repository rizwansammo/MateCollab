import type { Decorator, Preview } from '@storybook/react-vite';
import { TooltipProvider } from '../src/components/tooltip';
import './preview.css';

// The toolbar theme is applied to <html data-theme>, exactly as the app does, so portalled
// content (menus, dialogs, tooltips, toasts) follows it. Visual tests render every story with
// `globals=theme:light` and `globals=theme:dark`.
const withTheme: Decorator = (Story, context) => {
  const theme = context.globals.theme === 'dark' ? 'dark' : 'light';
  document.documentElement.setAttribute('data-theme', theme);
  const padded = context.parameters.padded !== false;
  return (
    <TooltipProvider>
      <div className={padded ? 'min-h-screen bg-bg p-6 text-fg' : 'min-h-screen bg-bg text-fg'}>
        <Story />
      </div>
    </TooltipProvider>
  );
};

const preview: Preview = {
  globalTypes: {
    theme: {
      description: 'Colour theme',
      toolbar: {
        title: 'Theme',
        icon: 'mirror',
        items: [
          { value: 'light', title: 'Light', icon: 'sun' },
          { value: 'dark', title: 'Dark', icon: 'moon' },
        ],
        dynamicTitle: true,
      },
    },
  },
  initialGlobals: { theme: 'light' },
  decorators: [withTheme],
  parameters: {
    layout: 'fullscreen',
    padded: true,
    controls: { expanded: true },
    a11y: { test: 'error' },
    options: {
      storySort: { order: ['Foundations', 'Components', 'Navigation', 'Shell', 'Patterns', 'Brand'] },
    },
  },
};

export default preview;
