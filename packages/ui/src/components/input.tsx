'use client';

import type { ComponentProps } from 'react';
import { cn } from '../lib/cn';
import { useFieldControl } from './field';

/** Shared look of text-entry controls (inputs, textareas, select triggers). */
export const controlClasses = [
  'w-full min-w-0 rounded-md border border-border-control bg-bg px-2.75 text-14 text-fg',
  'outline-hidden transition-[border-color,box-shadow] duration-140 placeholder:text-fg-muted',
  'focus-visible:border-primary focus-visible:ring-3 focus-visible:ring-primary/15',
  'disabled:cursor-not-allowed disabled:opacity-50',
  'aria-invalid:border-danger aria-invalid:focus-visible:ring-danger/15',
].join(' ');

/** Single-line text input, 38 px tall. Use inside <Field> for its label and messages. */
export function Input({ className, type = 'text', ...props }: ComponentProps<'input'>) {
  return <input type={type} className={cn(controlClasses, 'h-9.5', className)} {...useFieldControl(props)} />;
}

/** Multi-line text input; grows vertically only. */
export function Textarea({ className, rows = 4, ...props }: ComponentProps<'textarea'>) {
  return (
    <textarea
      rows={rows}
      className={cn(controlClasses, 'min-h-20 resize-y py-2.25 leading-[1.6]', className)}
      {...useFieldControl(props)}
    />
  );
}
