'use client';

import { zodResolver } from '@hookform/resolvers/zod';
import type { SessionDocument } from '@matecollab/contracts';
import { Button, Field, Input, Spinner } from '@matecollab/ui';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { CircleCheck, KeyRound, Lock, MailCheck, ShieldCheck } from 'lucide-react';
import type { Route } from 'next';
import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import { useEffect, useRef, useState } from 'react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import { ApiError, api, errorMessage, unwrap } from '@/lib/api';
import { AuthFootnote, AuthHeading, AuthLink, FormNotice } from './auth-frame';
import { EMAIL, PasswordInput, applyProblem, forgetFragment, readFragmentToken } from './form-helpers';
import { MFAForm } from './mfa-form';
import { acceptSession, safeNext, useSessionQuery } from './session';
import { SignInForm } from './sign-in-form';

/** Where to go after signing in: a safe `next` path, else organisation selection. */
function useAfterSignIn() {
  const params = useSearchParams();
  const router = useRouter();
  const client = useQueryClient();
  const next = safeNext(params.get('next'));
  const withNext = (path: string) => (next ? `${path}?next=${encodeURIComponent(next)}` : path);
  return {
    next,
    withNext,
    done: (doc: SessionDocument) => {
      acceptSession(client, doc);
      router.replace((next ?? '/select-organisation') as Route);
    },
  };
}

// ---- sign in ----------------------------------------------------------------------------------

export function SignInView() {
  const params = useSearchParams();
  const router = useRouter();
  const after = useAfterSignIn();
  const session = useSessionQuery({ allowAnonymous: true });

  // Already signed in (e.g. a bookmarked /login): continue instead of asking again.
  useEffect(() => {
    if (session.data) router.replace((after.next ?? '/select-organisation') as Route);
  }, [session.data, after.next, router]);

  return (
    <>
      <AuthHeading title="Welcome to MateCollab">Sign in to your workspace.</AuthHeading>
      {params.get('reason') === 'expired' ? (
        <FormNotice tone="info">Your session has ended. Sign in again to continue.</FormNotice>
      ) : null}
      {params.get('reason') === 'password-reset' ? (
        <FormNotice tone="success">
          Your password has been changed. Sign in with your new password.
        </FormNotice>
      ) : null}
      <SignInForm
        onSignedIn={after.done}
        onMFARequired={() => router.push(after.withNext('/login/mfa') as Route)}
      />
      <AuthLink href="/forgot-password">Forgot password?</AuthLink>
      <AuthFootnote icon={<Lock aria-hidden />}>
        MateCollab never asks for your password by email or chat.
      </AuthFootnote>
    </>
  );
}

// ---- second step ------------------------------------------------------------------------------

export function MFAView({ mode }: { mode: 'code' | 'recovery' }) {
  const router = useRouter();
  const after = useAfterSignIn();
  return (
    <>
      {mode === 'code' ? (
        <AuthHeading title="Two-step verification">
          Enter the 6-digit code from your authenticator app.
        </AuthHeading>
      ) : (
        <AuthHeading title="Use a recovery code">
          Enter one of the recovery codes you saved when you turned on two-step verification.
        </AuthHeading>
      )}
      <MFAForm
        mode={mode}
        onSignedIn={after.done}
        onRestart={() => router.replace(after.withNext('/login') as Route)}
      />
      {mode === 'code' ? (
        <AuthLink href={after.withNext('/login/recovery')}>Use a recovery code instead</AuthLink>
      ) : (
        <AuthLink href={after.withNext('/login/mfa')}>Use your authenticator app instead</AuthLink>
      )}
      <AuthFootnote icon={<ShieldCheck aria-hidden />}>
        <Link href={after.withNext('/login') as Route} className="underline-offset-4 hover:underline">
          Sign in with a different account
        </Link>
      </AuthFootnote>
    </>
  );
}

// ---- forgot password --------------------------------------------------------------------------

const forgotSchema = z.object({
  email: z.string().trim().min(1, 'Enter your work email.').regex(EMAIL, 'Enter a valid email address.'),
});

export function ForgotPasswordView() {
  const form = useForm<{ email: string }>({
    resolver: zodResolver(forgotSchema),
    defaultValues: { email: '' },
  });
  const [sentTo, setSentTo] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const request = useMutation({
    mutationFn: (email: string) => unwrap(api.POST('/auth/password/forgot', { body: { email } })),
    meta: { allowAnonymous: true },
  });

  if (sentTo) {
    return (
      <>
        <AuthHeading title="Check your email">
          If <strong className="font-strong text-fg">{sentTo}</strong> has a MateCollab account, we’ve sent a
          link to reset the password. The link works once and expires in 60 minutes.
        </AuthHeading>
        <Button asChild variant="secondary" className="h-10 w-full">
          <Link href="/login">Back to sign in</Link>
        </Button>
        <AuthFootnote icon={<MailCheck aria-hidden />}>
          No email? Check your spam folder or ask your administrator.
        </AuthFootnote>
      </>
    );
  }

  const submit = form.handleSubmit(({ email }) => {
    setFormError(null);
    request.mutate(email, {
      onSuccess: () => setSentTo(email),
      onError: (err) => setFormError(applyProblem(form, err, ['email'])),
    });
  });
  return (
    <>
      <AuthHeading title="Reset your password">
        Enter your work email and we’ll send you a reset link.
      </AuthHeading>
      <form onSubmit={submit} noValidate>
        {formError ? <FormNotice>{formError}</FormNotice> : null}
        <Field label="Work email" error={form.formState.errors.email?.message}>
          <Input
            type="email"
            autoComplete="username"
            inputMode="email"
            autoFocus
            {...form.register('email')}
          />
        </Field>
        <Button type="submit" variant="primary" loading={request.isPending} className="mt-2 h-10 w-full">
          Send reset link
        </Button>
      </form>
      <AuthLink href="/login">Back to sign in</AuthLink>
    </>
  );
}

// ---- reset password ---------------------------------------------------------------------------

const resetSchema = z
  .object({
    password: z.string().min(12, 'Use at least 12 characters.').max(128, 'Use 128 characters or fewer.'),
    confirm: z.string(),
  })
  .refine((v) => v.password === v.confirm, { path: ['confirm'], message: 'The passwords don’t match.' });
type ResetValues = z.infer<typeof resetSchema>;

/** Reads the one-time token from the fragment once, then removes it from the address bar. */
function useFragmentToken() {
  const [token, setToken] = useState<string | null | undefined>(undefined);
  const read = useRef(false);
  useEffect(() => {
    if (read.current) return;
    read.current = true;
    const t = readFragmentToken();
    forgetFragment();
    setToken(t);
  }, []);
  return token;
}

function InvalidLink({ title, action }: { title: string; action: { href: string; label: string } }) {
  return (
    <>
      <AuthHeading title={title}>This link is invalid, has expired or has already been used.</AuthHeading>
      <Button asChild variant="primary" className="h-10 w-full">
        <Link href={action.href as Route}>{action.label}</Link>
      </Button>
      <AuthLink href="/login">Back to sign in</AuthLink>
    </>
  );
}

export function ResetPasswordView() {
  const token = useFragmentToken();
  const form = useForm<ResetValues>({
    resolver: zodResolver(resetSchema),
    defaultValues: { password: '', confirm: '' },
  });
  const [invalid, setInvalid] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const router = useRouter();
  const reset = useMutation({
    mutationFn: (password: string) =>
      unwrap(api.POST('/auth/password/reset', { body: { token: token ?? '', new_password: password } })),
    meta: { allowAnonymous: true },
  });

  if (token === undefined) return <Spinner label="Checking your link" />;
  if (token === null || invalid) {
    return (
      <InvalidLink
        title="This link can’t be used"
        action={{ href: '/forgot-password', label: 'Request a new link' }}
      />
    );
  }

  const submit = form.handleSubmit(({ password }) => {
    setFormError(null);
    reset.mutate(password, {
      onSuccess: () => router.replace('/login?reason=password-reset' as Route),
      onError: (err) => {
        if (err instanceof ApiError && err.field('token')) return setInvalid(true);
        const pw = err instanceof ApiError ? err.field('new_password') : undefined;
        if (pw) form.setError('password', { message: pw }, { shouldFocus: true });
        else setFormError(errorMessage(err));
      },
    });
  });
  const { errors } = form.formState;
  return (
    <>
      <AuthHeading title="Choose a new password">
        Use at least 12 characters. Avoid common passwords and your email address.
      </AuthHeading>
      <form onSubmit={submit} noValidate>
        {formError ? <FormNotice>{formError}</FormNotice> : null}
        <Field label="New password" error={errors.password?.message}>
          <PasswordInput autoComplete="new-password" autoFocus {...form.register('password')} />
        </Field>
        <Field label="Confirm new password" error={errors.confirm?.message}>
          <PasswordInput autoComplete="new-password" {...form.register('confirm')} />
        </Field>
        <Button type="submit" variant="primary" loading={reset.isPending} className="mt-2 h-10 w-full">
          Save password
        </Button>
      </form>
      <AuthFootnote icon={<KeyRound aria-hidden />}>Saving signs you out on every device.</AuthFootnote>
    </>
  );
}

// ---- verify email -----------------------------------------------------------------------------

export function VerifyEmailView() {
  const token = useFragmentToken();
  const verify = useMutation({
    mutationFn: (t: string) => unwrap(api.POST('/auth/email/verify', { body: { token: t } })),
    meta: { allowAnonymous: true },
  });
  const started = useRef(false);
  useEffect(() => {
    if (token && !started.current) {
      started.current = true;
      verify.mutate(token);
    }
  }, [token, verify]);

  if (token === null) {
    return (
      <InvalidLink title="This link can’t be used" action={{ href: '/login', label: 'Go to sign in' }} />
    );
  }
  if (verify.isError) {
    const err = verify.error;
    if (err instanceof ApiError && err.code === 'email_taken') {
      return (
        <>
          <AuthHeading title="Address already in use">{err.message}</AuthHeading>
          <AuthLink href="/login">Back to sign in</AuthLink>
        </>
      );
    }
    if (err instanceof ApiError && (err.field('token') || err.status === 422)) {
      return (
        <InvalidLink title="This link can’t be used" action={{ href: '/login', label: 'Go to sign in' }} />
      );
    }
    return (
      <>
        <AuthHeading title="We couldn’t confirm your email">{errorMessage(err)}</AuthHeading>
        <Button variant="primary" className="h-10 w-full" onClick={() => token && verify.mutate(token)}>
          Try again
        </Button>
      </>
    );
  }
  if (verify.isSuccess) {
    return (
      <>
        <div aria-hidden className="mb-5 text-success [&_svg]:size-8">
          <CircleCheck />
        </div>
        <AuthHeading title="Email address confirmed">
          You can now use this address with MateCollab.
        </AuthHeading>
        <Button asChild variant="primary" className="h-10 w-full">
          <Link href="/select-organisation">Continue to MateCollab</Link>
        </Button>
      </>
    );
  }
  return (
    <div className="flex items-center gap-3 text-14 text-fg-muted">
      <Spinner label="Confirming your email address" /> Confirming your email address…
    </div>
  );
}
