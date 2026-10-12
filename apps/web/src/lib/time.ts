// Date formatting with Intl in the en-GB locale (FRONTEND.md §1). Times are shown in the
// browser's zone until profile time zones are applied app-wide.

const dateTime = new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeStyle: 'short' });
const date = new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium' });
const relative = new Intl.RelativeTimeFormat('en-GB', { numeric: 'auto' });

export function formatDateTime(value: string | Date): string {
  return dateTime.format(new Date(value));
}

export function formatDate(value: string | Date): string {
  return date.format(new Date(value));
}

const units: [Intl.RelativeTimeFormatUnit, number][] = [
  ['year', 365 * 24 * 3600],
  ['month', 30 * 24 * 3600],
  ['week', 7 * 24 * 3600],
  ['day', 24 * 3600],
  ['hour', 3600],
  ['minute', 60],
];

/** "just now", "5 minutes ago", "in 3 days". */
export function formatRelative(value: string | Date, now: Date = new Date()): string {
  const seconds = Math.round((new Date(value).getTime() - now.getTime()) / 1000);
  if (Math.abs(seconds) < 45) return 'just now';
  for (const [unit, size] of units) {
    if (Math.abs(seconds) >= size || unit === 'minute')
      return relative.format(Math.round(seconds / size), unit);
  }
  return 'just now';
}
