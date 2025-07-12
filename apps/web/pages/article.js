import Head from 'next/head';
import Image from 'next/image';
import Link from 'next/link';
import { useRouter } from 'next/router';
import { useEffect } from 'react';
import { useSession, signOut } from 'next-auth/react';
import { getServerSession } from 'next-auth/next';
import { authOptions } from '../lib/auth';
import { articleQuery, fetchArticles } from '../lib/articles';
import { formatDate } from '../lib/format';

export async function getServerSideProps(context) {
  const session = await getServerSession(context.req, context.res, authOptions);
  if (!session) {
    return { redirect: { destination: '/login', permanent: false } };
  }
  const params = articleQuery(context.query, true);
  const result = params
    ? await fetchArticles(`/api/articles/${encodeURIComponent(params.id)}`)
    : { data: null, status: 404 };
  const article = result.data?.article || null;
  const error = result.status === 404 || result.status === 400
    ? 'Artikel tidak tersedia. Silakan kembali ke daftar berita.'
    : !article ? 'Layanan berita sementara tidak tersedia. Silakan coba lagi.' : null;
  context.res.statusCode = article ? 200 : result.status;
  return { props: { session, article, error } };
}

function Masthead() {
  return (
    <header className="bg-teal text-cream">
      <div className="mx-auto flex max-w-5xl items-center justify-between gap-4 px-6 py-4 sm:px-10">
        <Link
          href="/"
          className="font-serif text-2xl leading-none tracking-tight focus:outline-none focus-visible:ring-2 focus-visible:ring-lime"
        >
          LokaFeed
        </Link>
        <div className="flex items-center gap-4 text-sm">
          <Link
            href="/"
            className="hidden text-cream/80 underline underline-offset-4 hover:text-cream sm:inline focus:outline-none focus-visible:ring-2 focus-visible:ring-lime"
          >
            Kembali ke beranda
          </Link>
          <button
            onClick={() => signOut({ callbackUrl: '/login' })}
            className="min-h-[44px] rounded-sm border border-cream/50 px-4 text-cream transition hover:border-cream hover:bg-cream/10 focus:outline-none focus-visible:ring-2 focus-visible:ring-lime focus-visible:ring-offset-2 focus-visible:ring-offset-teal"
          >
            Keluar
          </button>
        </div>
      </div>
    </header>
  );
}

export default function ArticleDetail({ article, error }) {
  const router = useRouter();
  const { status } = useSession();
  const { title, description, content, urlToImage, url, sourceName, author, publishedAt } = article || {};

  useEffect(() => {
    if (status === 'unauthenticated') {
      router.push('/login');
    }
  }, [status, router]);

  if (status === 'loading') {
    return (
      <div className="flex min-h-screen items-center justify-center bg-white px-6">
        <p className="text-sm text-muted">Memuat sesi…</p>
      </div>
    );
  }

  if (status === 'unauthenticated') {
    return null;
  }

  if (!title) {
    return (
      <div className="min-h-screen bg-white font-sans text-ink">
        <Masthead />
        <main className="mx-auto max-w-[720px] px-6 py-16 sm:px-8">
          <p role="alert" className="font-serif text-2xl text-ink">
            {error || 'Artikel tidak tersedia.'}
          </p>
          <p className="mt-3 max-w-prose text-sm text-body">
            Artikel mungkin sudah tidak tersimpan setelah layanan dimulai ulang.
          </p>
          <Link
            href="/"
            className="mt-6 inline-flex min-h-[44px] items-center rounded-sm bg-teal px-5 text-sm font-semibold text-cream transition hover:bg-ink focus:outline-none focus-visible:ring-2 focus-visible:ring-teal focus-visible:ring-offset-2"
          >
            Kembali ke beranda
          </Link>
        </main>
      </div>
    );
  }

  const published = formatDate(publishedAt);
  const showContent = Boolean(content) && content !== description;
  const metaParts = [
    sourceName ? `Sumber: ${sourceName}` : 'Sumber tidak tercantum',
    author ? `Penulis: ${author}` : null,
    published,
  ].filter(Boolean);

  return (
    <div className="min-h-screen bg-white font-sans text-ink">
      <Head>
        <title>{title} — LokaFeed</title>
        <meta name="description" content={description || title} />
      </Head>

      <Masthead />

      <main className="mx-auto max-w-[760px] px-6 pb-16 pt-10 sm:px-8">
        <Link
          href="/"
          className="inline-flex min-h-[44px] items-center text-sm text-teal underline underline-offset-4 hover:text-ink focus:outline-none focus-visible:ring-2 focus-visible:ring-teal sm:hidden"
        >
          Kembali ke beranda
        </Link>

        <article>
          <header className="border-b border-rule pb-6">
            <h1 className="font-serif text-[2rem] font-bold leading-tight tracking-tight text-ink sm:text-[2.4rem]">
              {title}
            </h1>
            <p className="mt-4 text-xs leading-relaxed text-muted">{metaParts.join(' · ')}</p>
          </header>

          {urlToImage && (
            <div className="mt-8">
              <Image
                src={urlToImage}
                alt=""
                width={1200}
                height={675}
                unoptimized
                sizes="(min-width: 768px) 700px, 100vw"
                className="h-auto w-full border border-rule bg-cream object-contain"
              />
            </div>
          )}

          {description && (
            <p className="mt-8 max-w-[68ch] text-lg leading-relaxed text-body">{description}</p>
          )}

          {showContent && (
            <div className="mt-6 max-w-[68ch] text-[17px] leading-[1.75] text-ink">
              <p>{content}</p>
            </div>
          )}

          {url ? (
            <a
              href={url}
              target="_blank"
              rel="noopener noreferrer"
              className="mt-10 inline-flex min-h-[44px] items-center rounded-sm bg-teal px-5 text-sm font-semibold text-cream transition hover:bg-ink focus:outline-none focus-visible:ring-2 focus-visible:ring-teal focus-visible:ring-offset-2"
            >
              Baca artikel asli
              <span className="sr-only"> (membuka tab baru)</span>
            </a>
          ) : (
            <p className="mt-10 text-xs text-muted">Tautan sumber tidak tersedia.</p>
          )}
        </article>

        <Link
          href="/"
          className="mt-10 inline-flex min-h-[44px] items-center text-sm text-teal underline underline-offset-4 hover:text-ink focus:outline-none focus-visible:ring-2 focus-visible:ring-teal"
        >
          Kembali ke daftar berita
        </Link>
      </main>
    </div>
  );
}
