import { clsx, type ClassValue } from 'clsx';
import { extendTailwindMerge } from 'tailwind-merge';

// tailwind-merge must know the custom type scale (text-11 … text-32) is font size, not colour,
// otherwise `text-13 text-fg-muted` would be treated as two conflicting colours.
const twMerge = extendTailwindMerge({
  extend: {
    classGroups: {
      'font-size': [{ text: ['11', '12', '13', '14', '15', '16', '18', '19', '24', '28', '32'] }],
      'font-weight': [{ font: ['book', 'strong', 'heading', 'heavy'] }],
    },
  },
});

/** Joins class names and resolves Tailwind conflicts (later classes win). */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
