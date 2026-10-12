import type { MetadataRoute } from 'next';
import { brandValues } from '@/lib/brand-values';

// Values from the Logo Library's site.webmanifest; icon paths point at the synced copies.
export default function manifest(): MetadataRoute.Manifest {
  return {
    name: 'MateCollab',
    short_name: 'MateCollab',
    start_url: '/',
    display: 'standalone',
    theme_color: brandValues.violet,
    background_color: brandValues.navy,
    icons: [
      { src: '/brand/favicons/icon-192.png', sizes: '192x192', type: 'image/png', purpose: 'any' },
      { src: '/brand/favicons/icon-512.png', sizes: '512x512', type: 'image/png', purpose: 'any' },
    ],
  };
}
