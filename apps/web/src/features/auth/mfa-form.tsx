'use client';

import { zodResolver } from '@hookform/resolvers/zod';
import type { SessionDocument } from '@matecollab/contracts';
import { Button, Field, Input } from '@matecollab/ui';
import { useMutation } from '@tanstack/react-query';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import { ApiError, api, errorMessage, unwrap } from '@/lib/api';
import { FormNotice } from './auth-frame';

const codeSchema = z.object({
  code: z
    .string()
    .transform((v) => v.replace(/\s+/g, ''))
    .pipe(z.string().regex(/^\d{6}$/, 'Enter the 6-digit code from your authenticator app.')),
});
const recoverySchema = z.object({
  code: z
    .string()
    .trim()
    .min(10, 'Enter one of your recovery codes.')
    .max(32, 'That doesn’t look like a recovery code.'),
});
type Values = { code: string };

export interface MFAFormProps {
  mode: 'code' | 'recovery';
  onSignedIn: (session: SessionDocument) => void;
  /** The challenge expired or used up its attempts: the user must sign in again. */
  onRestart: () => void;
}

/** Second sign-in step: an authenticator code or a single-use recovery code. */
export function MFAForm({ mode, onSignedIn, onRestart }: MFAFormProps) {
  const form = useForm<Values>({
    resolver: zodResolver(mode === 'code' ? codeSchema : recoverySchema) as never,
    defaultValues: { code: '' },
  });
  const [expired, setExpired] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const verify = useMutation({
    mutationFn: (code: string) =>
      unwrap(api.POST('/auth/login/mfa', { body: mode === 'code' ? { code } : { recovery_code: code } })),
    meta: { allowAnonymous: true },
  });

  const submit = form.handleSubmit(({ code }) => {
    setFormError(null);
    verify.mutate(code, {
      onSuccess: (res) => res.session && onSignedIn(res.session),
      onError: (err) => {
        form.setValue('code', '');
        if (err instanceof ApiError && err.code === 'mfa_challenge_expired') setExpired(true);
        else if (err instanceof ApiError && err.code === 'invalid_code')
          form.setError(
            'code',
            {
              message:
                mode === 'code'
                  ? 'That code didn’t work. Check your authenticator app and try again.'
                  : 'That recovery code didn’t work. Each code can be used once.',
            },
            { shouldFocus: true },
          );
        else setFormError(errorMessage(err));
      },
    });
  });

  if (expired) {
    return (
      <div>
        <FormNotice>For your security, this sign-in attempt has ended. Sign in again to continue.</FormNotice>
        <Button variant="primary" className="h-10 w-full" onClick={onRestart}>
          Sign in again
        </Button>
      </div>
    );
  }

  const error = form.formState.errors.code?.message;
  return (
    <form onSubmit={submit} noValidate>
      {formError ? <FormNotice>{formError}</FormNotice> : null}
      {mode === 'code' ? (
        <Field
          label="Authentication code"
          error={error}
          description="Open your authenticator app to see the current code."
        >
          <Input
            autoComplete="one-time-code"
            inputMode="numeric"
            maxLength={7}
            autoFocus
            className="font-mono tracking-[0.3em]"
            {...form.register('code')}
          />
        </Field>
      ) : (
        <Field label="Recovery code" error={error} description="Each recovery code works once.">
          <Input
            autoComplete="off"
            spellCheck={false}
            autoFocus
            className="font-mono"
            {...form.register('code')}
          />
        </Field>
      )}
      <Button type="submit" variant="primary" loading={verify.isPending} className="mt-2 h-10 w-full">
        Verify and sign in
      </Button>
    </form>
  );
}
