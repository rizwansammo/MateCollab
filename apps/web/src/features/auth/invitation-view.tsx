'use client';

import { zodResolver } from '@hookform/resolvers/zod';
import type { components, InvitationPreview, SessionDocument } from '@matecollab/contracts';
import { Button, Field, Input, OrgBadge, Spinner } from '@matecollab/ui';
import { useMutation, useQuery, useQueryClient, type UseMutationResult } from '@tanstack/react-query';
import { Clock, MailX, UserRoundCheck } from 'lucide-react';
import type { Route } from 'next';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useEffect, useRef, useState } from 'react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import { ApiError, api, errorMessage, setCsrfToken, unwrap } from '@/lib/api';
import { roleLabel } from '@/features/org/roles';
import { AuthFootnote, AuthHeading, AuthLink, FormNotice } from './auth-frame';
import { PasswordInput, applyProblem, forgetFragment, readFragmentToken } from './form-helpers';
import { MFAForm } from './mfa-form';
import { acceptSession, isSignedOut, sessionKey, useSessionQuery } from './session';
import { SignInForm } from './sign-in-form';

const accountSchema = z
  .object({
    displayName: z.string().trim().min(1, 'Enter your name.').max(80, 'Use 80 characters or fewer.'),
    password: z.string().min(12, 'Use at least 12 characters.').max(128, 'Use 128 characters or fewer.'),
    confirm: z.string(),
  })
  .refine((v) => v.password === v.confirm, { path: ['confirm'], message: 'The passwords don’t match.' });
type AccountValues = z.infer<typeof accountSchema>;
type AcceptMutation = UseMutationResult<
  components['schemas']['AcceptInvitationResponse'],
  Error,
  { display_name?: string; password?: string }
>;

/**
 * Invitation acceptance. The token is read from the URL fragment into memory and removed from
 * the address bar; it is only ever sent in request bodies. The organisation and role always come
 * from the server's copy of the invitation.
 */
export function InvitationView() {
  const [token, setToken] = useState<string | null | undefined>(undefined);
  const read = useRef(false);
  useEffect(() => {
    if (read.current) return;
    read.current = true;
    const t = readFragmentToken();
    forgetFragment();
    setToken(t);
  }, []);

  const preview = useQuery({
    queryKey: ['invitation', token],
    queryFn: () => unwrap(api.POST('/invitations/preview', { body: { token: token ?? '' } })),
    enabled: Boolean(token),
    staleTime: Infinity,
    meta: { allowAnonymous: true },
  });

  if (token === undefined || (token && preview.isPending)) {
    return (
      <div className="flex items-center gap-3 text-14 text-fg-muted">
        <Spinner label="Opening your invitation" /> Opening your invitation…
      </div>
    );
  }
  if (!token || (preview.error instanceof ApiError && preview.error.status === 404)) {
    return (
      <Closed
        icon={<MailX aria-hidden />}
        title="This invitation can’t be used"
        text="The link is invalid or the invitation was withdrawn. Ask your organisation administrator for a new invitation."
      />
    );
  }
  if (preview.isError) {
    return (
      <>
        <AuthHeading title="We couldn’t open this invitation">{errorMessage(preview.error)}</AuthHeading>
        <Button variant="primary" className="h-10 w-full" onClick={() => void preview.refetch()}>
          Try again
        </Button>
      </>
    );
  }
  if (!preview.isSuccess) return null;
  return <Invitation token={token} invitation={preview.data} />;
}

function Closed({ icon, title, text }: { icon: React.ReactNode; title: string; text: string }) {
  return (
    <>
      <div aria-hidden className="mb-5 text-fg-muted [&_svg]:size-8">
        {icon}
      </div>
      <AuthHeading title={title}>{text}</AuthHeading>
      <Button asChild variant="secondary" className="h-10 w-full">
        <Link href="/login">Go to sign in</Link>
      </Button>
    </>
  );
}

function OrgIdentity({ invitation }: { invitation: InvitationPreview }) {
  return (
    <div className="mb-2.5 flex items-center gap-5 border-b border-border pt-1.5 pb-7.25">
      <OrgBadge name={invitation.organisation.name} className="size-13 text-24" />
      <div className="min-w-0 flex-1">
        <strong className="block truncate text-16 font-heading">{invitation.organisation.name}</strong>
        <p className="mt-1.25 text-13 text-fg-muted">
          {invitation.inviter_name ? `${invitation.inviter_name} invited you` : 'You’re invited'} to join as{' '}
          {roleLabel(invitation.role, { article: true })}.
        </p>
      </div>
    </div>
  );
}

type Step = 'auto' | 'sign-in' | 'mfa' | 'recovery';

function Invitation({ token, invitation }: { token: string; invitation: InvitationPreview }) {
  const router = useRouter();
  const client = useQueryClient();
  const session = useSessionQuery({ allowAnonymous: true });
  const [step, setStep] = useState<Step>('auto');
  const [formError, setFormError] = useState<string | null>(null);

  const accept = useMutation({
    mutationFn: (body: { display_name?: string; password?: string }) =>
      unwrap(api.POST('/invitations/accept', { body: { token, ...body } })),
    meta: { allowAnonymous: true },
    onSuccess: async (res) => {
      await client.invalidateQueries({ queryKey: sessionKey });
      router.replace(`/${res.organisation.slug}/home` as Route);
    },
  });

  // Signing out here keeps the token in memory, so the invitation can be accepted with another account.
  const signOut = useMutation({
    mutationFn: () => unwrap(api.POST('/auth/logout')),
    meta: { allowAnonymous: true },
    onSettled: () => {
      setCsrfToken(null);
      client.setQueryData(sessionKey, undefined);
      void client.invalidateQueries({ queryKey: sessionKey });
      accept.reset();
      setFormError(null);
    },
  });

  const signedIn = (doc: SessionDocument) => {
    acceptSession(client, doc);
    accept.mutate({}, { onError: (err) => setFormError(errorMessage(err)) });
  };

  if (invitation.status === 'accepted') {
    return (
      <>
        <OrgIdentity invitation={invitation} />
        <Closed
          icon={<UserRoundCheck aria-hidden />}
          title="Invitation already used"
          text="This invitation has already been accepted. Sign in to open the organisation."
        />
      </>
    );
  }
  if (invitation.status === 'expired') {
    return (
      <>
        <OrgIdentity invitation={invitation} />
        <Closed
          icon={<Clock aria-hidden />}
          title="This invitation has expired"
          text={`Invitations are valid for 7 days. Ask ${invitation.inviter_name || 'your organisation administrator'} to send a new one.`}
        />
      </>
    );
  }

  if (session.isPending) return <Spinner label="Checking your sign-in" />;

  const acceptError = accept.error instanceof ApiError ? accept.error : null;

  // Signed in: accept with this account (the server checks that it owns the invited address).
  if (session.data && step === 'auto') {
    const user = session.data.user;
    return (
      <>
        <AuthHeading title={`Join ${invitation.organisation.name}`}>
          You’re signed in as <strong className="font-strong text-fg">{user.primary_email}</strong>.
        </AuthHeading>
        <OrgIdentity invitation={invitation} />
        {acceptError?.code === 'invitation_email_mismatch' ? (
          <>
            <FormNotice>
              This invitation was sent to <strong>{invitation.email}</strong>. Sign out, then sign in with
              that address to accept it.
            </FormNotice>
            <Button
              variant="primary"
              className="h-10 w-full"
              loading={signOut.isPending}
              onClick={() => signOut.mutate()}
            >
              Sign out and continue
            </Button>
          </>
        ) : (
          <>
            {acceptError ? <FormNotice>{errorMessage(acceptError)}</FormNotice> : null}
            <Button
              variant="primary"
              className="mt-5 h-10 w-full"
              loading={accept.isPending}
              onClick={() => accept.mutate({})}
            >
              Accept invitation
            </Button>
            <Button variant="ghost" className="mt-2 h-10 w-full" onClick={() => signOut.mutate()}>
              Use a different account
            </Button>
          </>
        )}
      </>
    );
  }

  if (session.isError && !isSignedOut(session.error)) {
    return <FormNotice>{errorMessage(session.error)}</FormNotice>;
  }

  // Existing account, signed out: sign in here (with two-step verification if enabled).
  if (invitation.account_exists || step !== 'auto') {
    return (
      <>
        <AuthHeading title={`Join ${invitation.organisation.name}`}>
          Sign in as <strong className="font-strong text-fg">{invitation.email}</strong> to accept.
        </AuthHeading>
        <OrgIdentity invitation={invitation} />
        <div className="mt-5">
          {formError ? <FormNotice>{formError}</FormNotice> : null}
          {step === 'mfa' || step === 'recovery' ? (
            <>
              <MFAForm
                mode={step === 'mfa' ? 'code' : 'recovery'}
                onSignedIn={signedIn}
                onRestart={() => setStep('sign-in')}
              />
              <button
                type="button"
                className="mx-auto my-5.5 block text-13 font-strong text-primary-ink underline-offset-4 hover:underline"
                onClick={() => setStep(step === 'mfa' ? 'recovery' : 'mfa')}
              >
                {step === 'mfa' ? 'Use a recovery code instead' : 'Use your authenticator app instead'}
              </button>
            </>
          ) : (
            <SignInForm
              email={invitation.email}
              submitLabel="Sign in and accept"
              onSignedIn={signedIn}
              onMFARequired={() => setStep('mfa')}
            />
          )}
        </div>
        {step === 'auto' || step === 'sign-in' ? (
          <AuthLink href="/forgot-password">Forgot password?</AuthLink>
        ) : null}
      </>
    );
  }

  return <CreateAccount invitation={invitation} accept={accept} />;
}

function CreateAccount({ invitation, accept }: { invitation: InvitationPreview; accept: AcceptMutation }) {
  const form = useForm<AccountValues>({
    resolver: zodResolver(accountSchema),
    defaultValues: { displayName: '', password: '', confirm: '' },
  });
  const [formError, setFormError] = useState<string | null>(null);
  const submit = form.handleSubmit((v) => {
    setFormError(null);
    accept.mutate(
      { display_name: v.displayName, password: v.password },
      {
        onError: (err) => {
          const pw = err instanceof ApiError ? err.field('password') : undefined;
          if (pw) form.setError('password', { message: pw }, { shouldFocus: true });
          const name = err instanceof ApiError ? err.field('display_name') : undefined;
          if (name) form.setError('displayName', { message: name }, { shouldFocus: true });
          if (!pw && !name) setFormError(applyProblem(form, err, []));
        },
      },
    );
  });
  const { errors } = form.formState;
  return (
    <>
      <AuthHeading title={`Join ${invitation.organisation.name}`}>
        Create your MateCollab account for <strong className="font-strong text-fg">{invitation.email}</strong>
        .
      </AuthHeading>
      <OrgIdentity invitation={invitation} />
      <form onSubmit={submit} noValidate className="mt-5">
        {formError ? <FormNotice>{formError}</FormNotice> : null}
        <Field
          label="Your name"
          error={errors.displayName?.message}
          description="Shown to people in your organisation."
        >
          <Input autoComplete="name" autoFocus {...form.register('displayName')} />
        </Field>
        <Field label="Password" error={errors.password?.message} description="At least 12 characters.">
          <PasswordInput autoComplete="new-password" {...form.register('password')} />
        </Field>
        <Field label="Confirm password" error={errors.confirm?.message}>
          <PasswordInput autoComplete="new-password" {...form.register('confirm')} />
        </Field>
        <Button type="submit" variant="primary" loading={accept.isPending} className="mt-2 h-10 w-full">
          Create account and join
        </Button>
      </form>
      <AuthFootnote>
        Already have an account with another address? Ask for an invitation to that address.
      </AuthFootnote>
    </>
  );
}
