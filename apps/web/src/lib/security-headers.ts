// HTML security headers (docs/security/SECURITY-BASELINE.md §11), applied by proxy.ts. The API
// sets its own (stricter) headers; Nginx adds HSTS in production.

export interface CspOptions {
  nonce: string;
  /** Development needs 'unsafe-eval' for React Refresh. */
  dev: boolean;
  publicUrl?: string;
  livekitUrl?: string;
  storageUrl?: string;
}

function origin(url: string | undefined): string | null {
  if (!url) return null;
  try {
    return new URL(url).origin;
  } catch {
    return null;
  }
}

export function buildContentSecurityPolicy({
  nonce,
  dev,
  publicUrl,
  livekitUrl,
  storageUrl,
}: CspOptions): string {
  const self = origin(publicUrl);
  const realtime = self ? self.replace(/^http/, 'ws') : null;
  const livekit = origin(livekitUrl);
  const storage = origin(storageUrl);
  const list = (...values: (string | null | false)[]) => values.filter(Boolean).join(' ');

  const directives: [string, string][] = [
    ['default-src', "'self'"],
    ['script-src', list("'self'", `'nonce-${nonce}'`, "'strict-dynamic'", dev && "'unsafe-eval'")],
    // Radix positions popovers with inline style attributes (accepted in the baseline).
    ['style-src', "'self' 'unsafe-inline'"],
    ['img-src', list("'self'", 'data:', 'blob:', storage)],
    ['media-src', list("'self'", 'blob:', storage)],
    ['font-src', "'self'"],
    ['connect-src', list("'self'", realtime, livekit, storage)],
    ['frame-src', storage ?? "'none'"],
    ['worker-src', "'self' blob:"],
    ['object-src', "'none'"],
    ['base-uri', "'none'"],
    ['form-action', "'self'"],
    ['frame-ancestors', "'none'"],
  ];
  const policy = directives.map(([name, value]) => `${name} ${value}`);
  if (self?.startsWith('https://')) policy.push('upgrade-insecure-requests');
  return policy.join('; ');
}

export const staticSecurityHeaders: Record<string, string> = {
  'Referrer-Policy': 'strict-origin-when-cross-origin',
  'Permissions-Policy':
    'camera=(self), microphone=(self), display-capture=(self), fullscreen=(self), geolocation=(), payment=(), usb=(), bluetooth=()',
  'Cross-Origin-Opener-Policy': 'same-origin',
  'Cross-Origin-Resource-Policy': 'same-origin',
  'X-Frame-Options': 'DENY',
  'X-Content-Type-Options': 'nosniff',
};
