import { cn } from '../lib/cn';

// The only way application code renders the MateCollab identity (docs/BRANDING-ASSETS.md §5).
// Files are copied unchanged from MateCollab-Logo-Library by `pnpm brand:sync` into
// apps/web/public/brand (served at /brand). Theme changes select a different Library file;
// nothing is recoloured, redrawn or set in the HemiHead typeface.

export const BRAND_BASE_PATH = '/brand';

export const brandFiles = {
  markViolet: 'icons/transparent-violet.svg',
  markWhite: 'icons/transparent-white.svg',
  lockupLight: 'lockups/MateCollab-transparent.svg',
  lockupDark: 'lockups/MateCollab-white.svg',
} as const;

/** Lockup aspect ratio from the Library SVG viewBox (1670 × 320). */
const LOCKUP_RATIO = 1670 / 320;

const src = (file: string) => `${BRAND_BASE_PATH}/${file}`;

export interface BrandMarkProps {
  /** `white` on Navy/dark surfaces (the rail); `violet` on light; `theme` follows the theme. */
  variant?: 'violet' | 'white' | 'theme';
  /** Rendered size in px (≥ 16). */
  size?: number;
  /** Accessible name; pass "" when a visible label already names the product. */
  alt?: string;
  className?: string;
}

/** The MateCollab icon mark. */
export function BrandMark({ variant = 'theme', size = 28, alt = 'MateCollab', className }: BrandMarkProps) {
  const dimension = Math.max(16, size);
  const img = (file: string, extra?: string) => (
    <img
      src={src(file)}
      alt={alt}
      width={dimension}
      height={dimension}
      decoding="async"
      draggable={false}
      className={cn('shrink-0 select-none', extra, className)}
    />
  );
  if (variant === 'violet') return img(brandFiles.markViolet);
  if (variant === 'white') return img(brandFiles.markWhite);
  return (
    <>
      {img(brandFiles.markViolet, 'dark:hidden')}
      {img(brandFiles.markWhite, 'hidden dark:inline-block')}
    </>
  );
}

export interface BrandLockupProps {
  /** `light` = violet mark + navy wordmark; `dark` = white; `theme` follows the theme. */
  variant?: 'light' | 'dark' | 'theme';
  /** Rendered height in px (≥ 18). */
  height?: number;
  alt?: string;
  className?: string;
}

/** Icon + wordmark lockup (the wordmark exists only as outlined artwork in the Library). */
export function BrandLockup({
  variant = 'theme',
  height = 22,
  alt = 'MateCollab',
  className,
}: BrandLockupProps) {
  const h = Math.max(18, height);
  const w = Math.round(h * LOCKUP_RATIO);
  const img = (file: string, extra?: string) => (
    <img
      src={src(file)}
      alt={alt}
      width={w}
      height={h}
      decoding="async"
      draggable={false}
      className={cn('shrink-0 select-none', extra, className)}
    />
  );
  if (variant === 'light') return img(brandFiles.lockupLight);
  if (variant === 'dark') return img(brandFiles.lockupDark);
  return (
    <>
      {img(brandFiles.lockupLight, 'dark:hidden')}
      {img(brandFiles.lockupDark, 'hidden dark:inline-block')}
    </>
  );
}
