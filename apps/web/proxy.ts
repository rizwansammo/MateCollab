import { NextResponse, type NextRequest } from 'next/server';
import { buildContentSecurityPolicy, staticSecurityHeaders } from '@/lib/security-headers';

// Session cookie set by the API (HttpOnly). Only its presence is checked here; the API validates it.
const SESSION_COOKIE = '__Host-mc_session';

// Pages that work without a session.
const PUBLIC_PATHS = ['/login', '/forgot-password', '/reset-password', '/verify-email', '/invite'];
// Pages whose links carry one-time tokens (in the fragment): never leak the URL as a referrer.
const TOKEN_PATHS = ['/reset-password', '/verify-email', '/invite'];

const matches = (pathname: string, paths: string[]) =>
  paths.some((p) => pathname === p || pathname.startsWith(`${p}/`));

// Next.js 16 request interception (formerly middleware). Every HTML response gets a fresh CSP
// nonce, which Next.js also applies to its own scripts, plus the baseline security headers.
// Visitors without a session cookie are sent to /login (FRONTEND.md §2).
export function proxy(request: NextRequest) {
  const { pathname, search } = request.nextUrl;
  if (!matches(pathname, PUBLIC_PATHS) && !request.cookies.has(SESSION_COOKIE)) {
    const login = new URL('/login', request.url);
    if (pathname !== '/' && pathname !== '/select-organisation')
      login.searchParams.set('next', `${pathname}${search}`);
    const redirect = NextResponse.redirect(login);
    for (const [name, value] of Object.entries(staticSecurityHeaders)) redirect.headers.set(name, value);
    return redirect;
  }

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
  if (matches(pathname, TOKEN_PATHS)) response.headers.set('Referrer-Policy', 'no-referrer');
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
