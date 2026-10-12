import path from 'node:path';
import type { NextConfig } from 'next';

const config: NextConfig = {
  // Self-contained server for the container image (infra/docker/web.Dockerfile).
  output: 'standalone',
  outputFileTracingRoot: path.join(import.meta.dirname, '../..'),
  // Workspace packages ship TypeScript source.
  transpilePackages: ['@matecollab/ui', '@matecollab/contracts'],
  reactStrictMode: true,
  poweredByHeader: false,
  // The dev server is reached through Nginx on http://localhost:8080 (one origin with the API),
  // and from the visual-check container via host.docker.internal.
  allowedDevOrigins: ['localhost', '127.0.0.1', 'host.docker.internal'],
  typedRoutes: true,
  // The floating dev indicator would cover the rail's account button.
  devIndicators: false,
  async redirects() {
    return [
      { source: '/', destination: '/home', permanent: false },
      { source: '/settings', destination: '/settings/appearance', permanent: false },
    ];
  },
};

export default config;
