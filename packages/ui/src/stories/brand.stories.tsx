import type { Meta, StoryObj } from '@storybook/react-vite';
import { BrandLockup, BrandMark } from '../components/brand';

const meta: Meta = {
  title: 'Brand/Library assets',
  parameters: {
    docs: {
      description: {
        component:
          'Rendered from unchanged MateCollab-Logo-Library files (synced with `pnpm brand:sync`). Never recolour, redraw or set the wordmark in HemiHead.',
      },
    },
  },
};
export default meta;
type Story = StoryObj;

export const Marks: Story = {
  render: () => (
    <div className="flex flex-wrap items-center gap-8">
      <div className="flex items-center gap-4 rounded-md border border-border p-4">
        {[16, 24, 28, 48].map((size) => (
          <BrandMark key={size} size={size} variant="violet" alt={`MateCollab mark ${size} px`} />
        ))}
      </div>
      <div className="flex items-center gap-4 rounded-md bg-rail p-4">
        {[16, 24, 28, 48].map((size) => (
          <BrandMark key={size} size={size} variant="white" alt={`MateCollab mark on Navy ${size} px`} />
        ))}
      </div>
      <div className="flex items-center gap-4 rounded-md border border-border p-4">
        <BrandMark size={32} variant="theme" />
        <span className="text-12 text-fg-muted">Follows the theme</span>
      </div>
    </div>
  ),
};

export const Lockups: Story = {
  render: () => (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center gap-8 rounded-md border border-border p-5">
        <BrandLockup variant="theme" height={22} />
        <BrandLockup variant="theme" height={40} />
      </div>
      <div className="flex flex-wrap items-center gap-8 rounded-md bg-rail p-5">
        <BrandLockup variant="dark" height={22} alt="MateCollab on Navy" />
        <BrandLockup variant="dark" height={40} alt="MateCollab on Navy, large" />
      </div>
    </div>
  ),
};
