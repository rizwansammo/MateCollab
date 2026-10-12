import { cn } from '../lib/cn';

export type Presence = 'available' | 'busy' | 'away' | 'dnd' | 'offline';

export const presenceLabels: Record<Presence, string> = {
  available: 'Available',
  busy: 'Busy',
  away: 'Away',
  dnd: 'Do not disturb',
  offline: 'Offline',
};

const presenceClasses: Record<Presence, string> = {
  available: 'bg-presence-available',
  busy: 'bg-presence-busy',
  away: 'bg-presence-away',
  dnd: 'bg-presence-dnd',
  offline: 'bg-presence-offline',
};

const toneClasses = [
  'bg-tone-1 text-tone-1-fg',
  'bg-tone-2 text-tone-2-fg',
  'bg-tone-3 text-tone-3-fg',
  'bg-tone-4 text-tone-4-fg',
  'bg-tone-5 text-tone-5-fg',
] as const;

/** Up to two initials: first and last word ("Ayesha Rahman" → "AR"). */
export function initialsOf(name: string): string {
  const words = name.trim().split(/\s+/u).filter(Boolean);
  const first = words[0];
  if (!first) return '?';
  const last = words.length > 1 ? words[words.length - 1] : undefined;
  const firstChar = Array.from(first)[0] ?? '';
  const lastChar = last ? (Array.from(last)[0] ?? '') : '';
  return (firstChar + lastChar).toLocaleUpperCase();
}

/** Stable tone (1–5) derived from a seed such as the user ID, so colours never shuffle. */
export function toneOf(seed: string): 1 | 2 | 3 | 4 | 5 {
  let hash = 0;
  for (const ch of seed) hash = (hash * 31 + (ch.codePointAt(0) ?? 0)) >>> 0;
  return ((hash % 5) + 1) as 1 | 2 | 3 | 4 | 5;
}

export interface PresenceDotProps {
  presence: Presence;
  /** Ring colour matching the surface behind the avatar. */
  ringClassName?: string;
  className?: string;
}

/** Presence colour dot. Decorative: pair it with text (the avatar label does this). */
export function PresenceDot({ presence, ringClassName = 'border-bg', className }: PresenceDotProps) {
  return (
    <span
      aria-hidden
      data-presence={presence}
      className={cn(
        'absolute right-0 bottom-0 size-2.25 rounded-full border-2',
        presenceClasses[presence],
        ringClassName,
        className,
      )}
    />
  );
}

export interface AvatarProps {
  /** Person or organisation name; used for initials and the accessible label. */
  name: string;
  /** Seed for the tone (defaults to the name); pass the user ID for stability. */
  toneSeed?: string;
  /** Image URL (served from the storage origin). Initials show while it is absent. */
  src?: string;
  /** Diameter in px. */
  size?: number;
  presence?: Presence;
  /** Class for the presence ring when the avatar sits on a non-default surface. */
  presenceRingClassName?: string;
  /** Hide from assistive technology when the name is already visible next to it. */
  decorative?: boolean;
  className?: string;
}

/** Circular avatar with initials or a photo and an optional presence dot. */
export function Avatar({
  name,
  toneSeed,
  src,
  size = 32,
  presence,
  presenceRingClassName,
  decorative = false,
  className,
}: AvatarProps) {
  const label = presence ? `${name}, ${presenceLabels[presence]}` : name;
  return (
    <span
      role={decorative ? undefined : 'img'}
      aria-label={decorative ? undefined : label}
      aria-hidden={decorative || undefined}
      className={cn(
        'relative inline-flex shrink-0 select-none items-center justify-center rounded-full align-middle font-heading tracking-[0.2px]',
        toneClasses[toneOf(toneSeed ?? name) - 1],
        className,
      )}
      style={{ width: size, height: size, fontSize: Math.max(11, Math.round(size * 0.36)) }}
    >
      {src ? (
        <img src={src} alt="" className="size-full rounded-full object-cover" draggable={false} />
      ) : (
        <span aria-hidden>{initialsOf(name)}</span>
      )}
      {presence ? <PresenceDot presence={presence} ringClassName={presenceRingClassName} /> : null}
    </span>
  );
}

export interface AvatarGroupProps {
  people: { name: string; toneSeed?: string; src?: string }[];
  /** Visible avatars before "+N". */
  max?: number;
  size?: number;
  className?: string;
}

/** Overlapping avatars with an overflow count; announced as a single list. */
export function AvatarGroup({ people, max = 3, size = 27, className }: AvatarGroupProps) {
  const shown = people.slice(0, max);
  const extra = people.length - shown.length;
  const names = people.map((p) => p.name).join(', ');
  return (
    <span role="img" aria-label={names} className={cn('inline-flex items-center pl-1', className)}>
      {shown.map((person) => (
        <Avatar
          key={person.toneSeed ?? person.name}
          {...person}
          size={size}
          decorative
          className="-ml-1 border-2 border-bg"
        />
      ))}
      {extra > 0 ? (
        <span
          aria-hidden
          className="-ml-1 inline-flex items-center justify-center rounded-full border-2 border-bg bg-muted text-11 font-heading text-fg-muted"
          style={{ width: size, height: size }}
        >
          +{extra}
        </span>
      ) : null}
    </span>
  );
}
