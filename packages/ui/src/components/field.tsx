'use client';

import { CircleAlert } from 'lucide-react';
import { createContext, useContext, useId, type ReactNode } from 'react';
import { cn } from '../lib/cn';

interface FieldState {
  id: string;
  describedBy: string | undefined;
  invalid: boolean;
  required: boolean;
}

const FieldContext = createContext<FieldState | null>(null);

/**
 * Props a form control should receive inside a <Field>: its id (for the label), the ids of the
 * hint and error text, and its validity. Explicit props on the control win.
 */
export function useFieldControl<
  P extends { id?: string; 'aria-describedby'?: string; 'aria-invalid'?: unknown; required?: boolean },
>(props: P): P {
  const field = useContext(FieldContext);
  if (!field) return props;
  const describedBy = [field.describedBy, props['aria-describedby']].filter(Boolean).join(' ') || undefined;
  return {
    ...props,
    id: props.id ?? field.id,
    'aria-describedby': describedBy,
    'aria-invalid': props['aria-invalid'] ?? (field.invalid || undefined),
    required: props.required ?? (field.required || undefined),
  };
}

export interface FieldProps {
  label: ReactNode;
  /** Help text below the control, announced with it. */
  description?: ReactNode;
  /** Validation message; marks the control invalid and is announced with it. */
  error?: ReactNode;
  required?: boolean;
  /** Keeps the label for assistive technology only. */
  hideLabel?: boolean;
  className?: string;
  children: ReactNode;
}

/** Label + control + hint/error, wired together for assistive technology. */
export function Field({
  label,
  description,
  error,
  required = false,
  hideLabel,
  className,
  children,
}: FieldProps) {
  const id = useId();
  const descriptionId = description ? `${id}-description` : undefined;
  const errorId = error ? `${id}-error` : undefined;
  const state: FieldState = {
    id,
    describedBy: [errorId, descriptionId].filter(Boolean).join(' ') || undefined,
    invalid: Boolean(error),
    required,
  };
  return (
    <FieldContext value={state}>
      <div className={cn('mb-4.25 flex flex-col gap-1.75', className)}>
        <label htmlFor={id} className={cn('text-13 font-strong text-fg', hideLabel && 'sr-only')}>
          {label}
          {required ? (
            <span className="text-danger" aria-hidden>
              {' '}
              *
            </span>
          ) : null}
        </label>
        {children}
        {error ? (
          <p id={errorId} className="flex items-start gap-1.5 text-12 font-medium text-danger">
            <CircleAlert className="mt-0.5 size-3.5 shrink-0" aria-hidden />
            <span>{error}</span>
          </p>
        ) : null}
        {description ? (
          <p id={descriptionId} className="text-12 text-fg-muted">
            {description}
          </p>
        ) : null}
      </div>
    </FieldContext>
  );
}
