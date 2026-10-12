'use client';

import { zodResolver } from '@hookform/resolvers/zod';
import type { SessionDocument } from '@matecollab/contracts';
import { Button, Field, Input } from '@matecollab/ui';
import { useMutation } from '@tanstack/react-query';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import { api, unwrap } from '@/lib/api';
import { FormNotice } from './auth-frame';
import { EMAIL, PasswordInput, applyProblem } from './form-helpers';

const schema = z.object({
  email: z.string().trim().min(1, 'Enter your work email.').regex(EMAIL, 'Enter a valid email address.'),
  password: z.string().min(1, 'Enter your password.'),
});
type Values = z.infer<typeof schema>;

export interface SignInFormProps {
  /** Pre-filled (and locked) when signing in to accept an invitation. */
  email?: string;
  submitLabel?: string;
  onSignedIn: (session: SessionDocument) => void;
  onMFARequired: () => void;
}

/** Email + password step. The server answers wrong passwords and unknown accounts identically. */
export function SignInForm({ email, submitLabel = 'Continue', onSignedIn, onMFARequired }: SignInFormProps) {
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { email: email ?? '', password: '' },
  });
  const [formError, setFormError] = useState<string | null>(null);
  const login = useMutation({
    mutationFn: (values: Values) => unwrap(api.POST('/auth/login', { body: values })),
    meta: { allowAnonymous: true },
  });

  const submit = form.handleSubmit((values) => {
    setFormError(null);
    login.mutate(values, {
      onSuccess: (res) => {
        if (res.mfa_required) onMFARequired();
        else if (res.session) onSignedIn(res.session);
      },
      onError: (err) => {
        form.setValue('password', '');
        // Field errors go on the fields; wrong credentials and rate limits above the form.
        setFormError(applyProblem(form, err, ['email', 'password']));
      },
    });
  });

  const { errors } = form.formState;
  return (
    <form onSubmit={submit} noValidate>
      {formError ? <FormNotice>{formError}</FormNotice> : null}
      <Field label="Work email" error={errors.email?.message}>
        <Input
          type="email"
          autoComplete="username"
          inputMode="email"
          readOnly={Boolean(email)}
          autoFocus={!email}
          {...form.register('email')}
        />
      </Field>
      <Field label="Password" error={errors.password?.message}>
        <PasswordInput
          autoComplete="current-password"
          autoFocus={Boolean(email)}
          {...form.register('password')}
        />
      </Field>
      <Button type="submit" variant="primary" loading={login.isPending} className="mt-2 h-10 w-full">
        {submitLabel}
      </Button>
    </form>
  );
}
