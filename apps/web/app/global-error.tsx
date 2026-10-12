'use client';

import './globals.css';

// Last-resort boundary when the root layout itself fails. Details stay in the console only.
export default function GlobalError({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return (
    <html lang="en-GB">
      <body className="flex min-h-dvh items-center justify-center bg-bg p-6 text-center text-fg">
        <div className="max-w-sm">
          <h1 className="text-18 font-semibold">Something went wrong</h1>
          <p className="mt-2 text-14 text-fg-muted">MateCollab could not load. Try again in a moment.</p>
          <button
            type="button"
            onClick={reset}
            className="mt-5 h-8.75 rounded-md border border-primary bg-primary px-3.25 text-13 font-strong text-white"
          >
            Try again
          </button>
        </div>
      </body>
    </html>
  );
}
