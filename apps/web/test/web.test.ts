import { isLinkActive, moduleFor, sidebarSections } from '@/components/shell/navigation';
import { parseSidebarWidth, parseTheme } from '@/lib/preferences';
import { buildContentSecurityPolicy, staticSecurityHeaders } from '@/lib/security-headers';

describe('preferences', () => {
  it('accepts only known themes and defaults to system', () => {
    expect(parseTheme('dark')).toBe('dark');
    expect(parseTheme('light')).toBe('light');
    expect(parseTheme('neon')).toBe('system');
    expect(parseTheme(undefined)).toBe('system');
  });

  it('clamps the sidebar width to 240–320 px', () => {
    expect(parseSidebarWidth('290')).toBe(290);
    expect(parseSidebarWidth('100')).toBe(240);
    expect(parseSidebarWidth('9999')).toBe(320);
    expect(parseSidebarWidth('abc')).toBe(274);
    expect(parseSidebarWidth(undefined)).toBe(274);
  });
});

describe('content security policy', () => {
  const production = buildContentSecurityPolicy({
    nonce: 'abc123',
    dev: false,
    publicUrl: 'https://collab.example.org',
    livekitUrl: 'wss://media.example.org',
    storageUrl: 'https://files.example.org/bucket',
  });

  it('restricts scripts to the per-request nonce', () => {
    expect(production).toContain("script-src 'self' 'nonce-abc123' 'strict-dynamic'");
    expect(production).not.toContain('unsafe-eval');
    expect(production).toContain("object-src 'none'");
    expect(production).toContain("frame-ancestors 'none'");
    expect(production).toContain("base-uri 'none'");
  });

  it('allows only the realtime, media and storage origins to connect', () => {
    expect(production).toContain(
      "connect-src 'self' wss://collab.example.org wss://media.example.org https://files.example.org",
    );
    expect(production).toContain('upgrade-insecure-requests');
  });

  it('relaxes only what local development needs', () => {
    const dev = buildContentSecurityPolicy({ nonce: 'n', dev: true, publicUrl: 'http://localhost:8080' });
    expect(dev).toContain("'unsafe-eval'");
    expect(dev).toContain("connect-src 'self' ws://localhost:8080");
    expect(dev).not.toContain('upgrade-insecure-requests');
    expect(dev).toContain("frame-src 'none'");
  });

  it('sends the baseline headers', () => {
    expect(staticSecurityHeaders['X-Frame-Options']).toBe('DENY');
    expect(staticSecurityHeaders['Cross-Origin-Opener-Policy']).toBe('same-origin');
    expect(staticSecurityHeaders['Permissions-Policy']).toContain('geolocation=()');
  });
});

describe('navigation', () => {
  it('maps paths to rail modules', () => {
    expect(moduleFor('/home')).toBe('home');
    expect(moduleFor('/saved')).toBe('home');
    expect(moduleFor('/chats')).toBe('chats');
    expect(moduleFor('/files')).toBe('files');
    expect(moduleFor('/settings/appearance')).toBe('settings');
    expect(moduleFor('/unknown')).toBe('home');
  });

  it('marks the default view active when no view is selected', () => {
    const [upcoming, past] = sidebarSections.meetings.links;
    expect(upcoming && isLinkActive(upcoming, '/meetings', new URLSearchParams())).toBe(true);
    expect(past && isLinkActive(past, '/meetings', new URLSearchParams())).toBe(false);
    expect(past && isLinkActive(past, '/meetings', new URLSearchParams('view=past'))).toBe(true);
  });
});
