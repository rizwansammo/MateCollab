import { NextResponse, type NextRequest } from 'next/server';
import { buildContentSecurityPolicy, staticSecurityHeaders } from '@/lib/security-headers';

// Next.js 16 request interception (formerly middleware). Every HTML response gets a fresh CSP
// nonce, which Next.js also applies to its own scripts, plus the baseline security headers.
// Authentication redirects are added together with sign-in.
export function proxy(request: NextRequest) {
  const nonce = Buffer.from(crypto.randomUUID()).toString('base64');
  const csp = buildContentSecurityPolicy({
    nonce,
    dev: process.env.NODE_ENV !== 'production',
    publicUrl: process.env.MC_PUBLIC_URL,
    livekitUrl: process.env.MC_LIVEKIT_PUBLIC_URL,
    storageUrl: process.env.MC_S3_PUBLIC_ENDPOINT,
  });

  const requestHeaders = new Headers(request.headers);
  requestHeaders.set('x-nonce', nonce);
  requestHeaders.set('Content-Security-Policy', csp);

  const response = NextResponse.next({ request: { headers: requestHeaders } });
  response.headers.set('Content-Security-Policy', csp);
  for (const [name, value] of Object.entries(staticSecurityHeaders)) response.headers.set(name, value);
  return response;
}

export const config = {
  matcher: [
    {
      // HTML routes only: static assets, brand files and icons do not need a nonce.
      source:
        '/((?!_next/static|_next/image|brand/|favicon.ico|icon.svg|apple-icon.png|manifest.webmanifest).*)',
      missing: [
        { type: 'header', key: 'next-router-prefetch' },
        { type: 'header', key: 'purpose', value: 'prefetch' },
      ],
    },
  ],
};
