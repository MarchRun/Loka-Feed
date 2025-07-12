import Head from 'next/head';
import { signIn, useSession } from 'next-auth/react';
import { useRouter } from 'next/router';
import { useEffect } from 'react';

export default function Login() {
  const { status } = useSession();
  const router = useRouter();
  const authError = router.isReady && typeof router.query.error === 'string' && router.query.error;

  useEffect(() => {
    if (status === 'authenticated') {
      router.push('/');
    }
  }, [status, router]);

  if (status === 'loading') {
    return (
      <div className="flex min-h-screen items-center justify-center bg-cream px-6">
        <p className="text-sm text-muted">Memuat…</p>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-cream px-6 font-sans text-ink">
      <Head>
        <title>Masuk — LokaFeed</title>
        <meta name="description" content="Masuk ke LokaFeed." />
      </Head>

      <main className="w-full max-w-sm text-center">
        <p className="font-serif text-[2.875rem] leading-none tracking-tight text-teal">LokaFeed</p>
        <p className="mt-3 text-[0.65rem] uppercase tracking-[0.35em] text-muted">
          Berita ringkas dari satu API
        </p>

        {authError && (
          <p
            role="alert"
            className="mt-8 border-l-4 border-teal bg-lime-tint px-4 py-3 text-left text-sm text-ink"
          >
            Masuk gagal. Silakan coba lagi.
          </p>
        )}

        <button
          onClick={() => signIn('google')}
          className="mt-10 flex min-h-[48px] w-full items-center justify-center rounded-sm bg-teal px-5 text-sm font-semibold text-cream transition hover:bg-ink focus:outline-none focus-visible:ring-2 focus-visible:ring-teal focus-visible:ring-offset-2"
        >
          Lanjutkan dengan Google
        </button>
      </main>
    </div>
  );
}
