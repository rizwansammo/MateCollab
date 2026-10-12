'use client';

import { IconButton, Input, cn } from '@matecollab/ui';
import { Eye, EyeOff } from 'lucide-react';
import { useState, type ComponentProps } from 'react';
import type { FieldValues, Path, UseFormReturn } from 'react-hook-form';
import { ApiError, errorMessage } from '@/lib/api';

/** A loose address check for instant feedback; the server decides what it accepts. */
export const EMAIL = /^[^\s@]+@[^\s@]+$/;

/**
 * Puts the server's field errors on the matching form fields and returns the message for the
 * rest (shown above the form). The API is authoritative; client validation is only a courtesy.
 */
export function applyProblem<T extends FieldValues>(
  form: UseFormReturn<T>,
  err: unknown,
  fields: Path<T>[],
): string | null {
  if (err instanceof ApiError && err.fields.length > 0) {
    let unmatched = false;
    for (const f of err.fields) {
      if ((fields as string[]).includes(f.field)) {
        form.setError(f.field as Path<T>, { type: 'server', message: f.message }, { shouldFocus: true });
      } else {
        unmatched = true;
      }
    }
    return unmatched ? errorMessage(err) : null;
  }
  return errorMessage(err);
}

/** Password input with a show/hide toggle. */
export function PasswordInput({ className, ...props }: Omit<ComponentProps<'input'>, 'type'>) {
  const [visible, setVisible] = useState(false);
  return (
    <div className="relative">
      <Input {...props} type={visible ? 'text' : 'password'} className={cn('pr-10', className)} />
      <IconButton
        label={visible ? 'Hide password' : 'Show password'}
        pressed={visible}
        tooltip={false}
        onClick={() => setVisible((v) => !v)}
        className="absolute top-1/2 right-1 -translate-y-1/2"
      >
        {visible ? <EyeOff aria-hidden /> : <Eye aria-hidden />}
      </IconButton>
    </div>
  );
}

/** Reads `token=` from the URL fragment, where one-time links carry it (never sent to servers). */
export function readFragmentToken(): string | null {
  const hash = window.location.hash.replace(/^#/, '');
  const token = new URLSearchParams(hash).get('token');
  return token && token.length <= 128 ? token : null;
}

/** Removes the token from the address bar and history once it has been read. */
export function forgetFragment() {
  window.history.replaceState(null, '', `${window.location.pathname}${window.location.search}`);
}
