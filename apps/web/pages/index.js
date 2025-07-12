import Head from 'next/head';
import Link from 'next/link';
import { getServerSession } from 'next-auth/next';
import { authOptions } from '../lib/auth';
import { articleQuery, fetchArticles, fetchSource } from '../lib/articles';
import { formatDate } from '../lib/format';
import { useSession, signIn, signOut } from 'next-auth/react';
import { useEffect, useState } from 'react';
import { useRouter } from 'next/router';

export async function getServerSideProps(context) {
  const session = await getServerSession(context.req, context.res, authOptions);
  if (!session) {
    return { redirect: { destination: '/login', permanent: false } };
  }
  const source = await fetchSource();
  const params = source ? articleQuery(context.query, false, source.categories) : null;
  const result = !source ? { data: null, status: 503 } : params
    ? await fetchArticles(`/api/articles?${new URLSearchParams(params)}`)
    : { data: null, status: 400 };
  const newsData = Array.isArray(result.data?.articles) ? result.data.articles : [];
  const error = result.status === 400
    ? 'Parameter pencarian tidak valid.'
    : !result.data ? 'Layanan berita sementara tidak tersedia. Silakan coba lagi.' : null;
  return {
    props: {
      session,
      newsData,
      error,
      source,
      initialQuery: params?.q || '',
      initialCategory: params?.category || '',
    },
  };
}

export default function Home({ newsData, error, source, initialQuery, initialCategory }) {
  const { data: session, status } = useSession();
  const router = useRouter();
  const [searchQuery, setSearchQuery] = useState(initialQuery || '');
  const [selectedCategory, setSelectedCategory] = useState(initialCategory || '');

  useEffect(() => {
    setSearchQuery(initialQuery);
  }, [initialQuery]);

  useEffect(() => {
    setSelectedCategory(initialCategory);
  }, [initialCategory]);

  useEffect(() => {
    if (status === 'unauthenticated') {
      router.push('/login');
    }
  }, [status, router]);

  const categoryLabel = (value) => {
    const match = source?.categories.find((item) => item.value === value);
    if (match) return match.label;
    return value ? value.charAt(0).toUpperCase() + value.slice(1) : 'Umum';
  };

  const categoryHref = (value) => {
    const query = {};
    if (searchQuery.trim()) query.q = searchQuery.trim();
    if (value) query.category = value;
    return { pathname: '/', query };
  };

  const handleSearch = (event) => {
    event.preventDefault();
    const query = {};
    if (searchQuery.trim()) query.q = searchQuery.trim();
    if (selectedCategory) query.category = selectedCategory;
    router.push({ pathname: '/', query });
  };

  if (status === 'loading') {
    return (
      <div className="flex min-h-screen items-center justify-center bg-white px-6">
        <p className="text-sm text-muted">Memuat sesi…</p>
      </div>
    );
  }

  if (status === 'unauthenticated') {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center gap-5 bg-white px-6 text-center">
        <p className="text-lg text-ink">Anda perlu masuk untuk melihat berita.</p>
        <button
          onClick={() => signIn('google')}
          className="min-h-[44px] rounded-sm bg-teal px-6 text-sm font-semibold text-cream transition hover:bg-ink focus:outline-none focus-visible:ring-2 focus-visible:ring-teal focus-visible:ring-offset-2"
        >
          Masuk dengan Google
        </button>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-white font-sans text-ink">
      <Head>
        <title>LokaFeed — Berita ringkas dari satu API</title>
        <meta name="description" content="Ringkasan berita dari API sumber aktif." />
      </Head>

      <header className="bg-teal text-cream">
        <div className="mx-auto flex max-w-7xl flex-col gap-5 px-6 py-6 sm:flex-row sm:items-end sm:justify-between sm:px-10">
          <div>
            <p className="font-serif text-[2.875rem] leading-none tracking-tight">LokaFeed</p>
            <p className="mt-3 text-[0.65rem] uppercase tracking-[0.35em] text-lime">
              Berita ringkas dari satu API
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-4 text-sm">
            {session?.user?.name && <span className="text-cream/80">{session.user.name}</span>}
            <button
              onClick={() => signOut({ callbackUrl: '/login' })}
              className="min-h-[44px] rounded-sm border border-cream/50 px-5 text-sm text-cream transition hover:border-cream hover:bg-cream/10 focus:outline-none focus-visible:ring-2 focus-visible:ring-lime focus-visible:ring-offset-2 focus-visible:ring-offset-teal"
            >
              Keluar
            </button>
          </div>
        </div>
      </header>

      <section aria-label="Pencarian berita" className="bg-lime">
        <form
          onSubmit={handleSearch}
          className="mx-auto flex max-w-7xl flex-col gap-3 px-6 py-4 sm:flex-row sm:items-center sm:gap-4 sm:px-10"
        >
          <label
            htmlFor="search"
            className="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-teal sm:w-24 sm:shrink-0"
          >
            Cari berita
          </label>
          <div className="flex w-full max-w-xl">
            <input
              id="search"
              type="search"
              value={searchQuery}
              onChange={(event) => setSearchQuery(event.target.value)}
              placeholder="contoh: ekonomi digital"
              className="h-12 min-w-0 flex-1 rounded-sm border-2 border-r-0 border-teal bg-white px-4 text-[15px] text-ink outline-none placeholder:text-muted/80 focus-visible:ring-2 focus-visible:ring-teal focus-visible:ring-offset-2"
            />
            <button
              type="submit"
              className="h-12 shrink-0 rounded-sm bg-teal px-7 text-[15px] font-semibold text-cream transition hover:bg-ink focus:outline-none focus-visible:ring-2 focus-visible:ring-teal focus-visible:ring-offset-2"
            >
              Cari
            </button>
          </div>
          <p className="text-xs text-teal sm:ml-auto">
            Sumber aktif: <strong className="font-semibold">{source?.name || 'tidak tersedia'}</strong>
          </p>
        </form>
      </section>

      <nav aria-label="Kategori" className="border-b border-rule bg-white">
        {source ? (
          <ul className="mx-auto flex max-w-7xl gap-1 overflow-x-auto px-4 sm:px-9">
            <li className="shrink-0">
              <Link
                href={categoryHref('')}
                aria-current={initialCategory === '' ? 'page' : undefined}
                className={`inline-flex min-h-[48px] items-center border-b-[3px] px-4 text-sm transition ${
                  initialCategory === ''
                    ? 'border-teal font-bold text-ink'
                    : 'border-transparent text-muted hover:text-ink'
                } focus:outline-none focus-visible:ring-2 focus-visible:ring-teal focus-visible:ring-offset-2`}
              >
                Semua
              </Link>
            </li>
            {source.categories.map((category) => (
              <li key={category.value} className="shrink-0">
                <Link
                  href={categoryHref(category.value)}
                  aria-current={initialCategory === category.value ? 'page' : undefined}
                  className={`inline-flex min-h-[48px] items-center border-b-[3px] px-4 text-sm transition ${
                    initialCategory === category.value
                      ? 'border-teal font-bold text-ink'
                      : 'border-transparent text-muted hover:text-ink'
                  } focus:outline-none focus-visible:ring-2 focus-visible:ring-teal focus-visible:ring-offset-2`}
                >
                  {category.label}
                </Link>
              </li>
            ))}
          </ul>
        ) : (
          <p className="mx-auto max-w-7xl px-6 py-4 text-sm text-muted sm:px-10">
            Kategori tidak tersedia karena metadata sumber gagal dimuat.
          </p>
        )}
      </nav>

      <main className="mx-auto max-w-[900px] px-6 pb-12 pt-10 sm:px-8">
        <div className="mb-1 flex items-center gap-5">
          <h1 className="text-[0.7rem] font-bold uppercase tracking-[0.2em] text-muted">Berita terbaru</h1>
          <span aria-hidden="true" className="h-px flex-1 bg-rule" />
        </div>

        {(initialQuery || initialCategory) && (
          <div className="mt-4 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-muted">
            {initialQuery && (
              <span>
                Hasil untuk <strong className="font-semibold text-ink">“{initialQuery}”</strong>
              </span>
            )}
            {initialCategory && (
              <span>
                Kategori <strong className="font-semibold text-ink">{categoryLabel(initialCategory)}</strong>
              </span>
            )}
            <Link
              href="/"
              className="text-teal underline underline-offset-2 hover:text-ink focus:outline-none focus-visible:ring-2 focus-visible:ring-teal"
            >
              Tampilkan semua
            </Link>
          </div>
        )}

        {error && (
          <p
            role="alert"
            className="mt-6 border-l-4 border-teal bg-lime-tint px-4 py-3 text-sm text-ink"
          >
            {error}
          </p>
        )}

        {newsData.length > 0 ? (
          <ol className="mt-2">
            {newsData.map((article) => {
              const published = formatDate(article.publishedAt);
              return (
                <li
                  key={article.id}
                  className="grid gap-2 border-b border-rule py-6 sm:grid-cols-[132px_minmax(0,1fr)] sm:gap-6"
                >
                  <div className="text-[0.68rem] uppercase tracking-[0.12em] text-muted">
                    <p className="font-bold text-teal">{categoryLabel(article.category)}</p>
                    {published && <p className="mt-1 normal-case tracking-normal">{published}</p>}
                  </div>
                  <div className="min-w-0">
                    {article.id ? (
                      <Link
                        href={{ pathname: '/article', query: { id: article.id } }}
                        className="font-serif text-2xl font-bold leading-snug text-ink underline-offset-4 transition hover:underline focus:outline-none focus-visible:ring-2 focus-visible:ring-teal focus-visible:ring-offset-4"
                      >
                        {article.title}
                      </Link>
                    ) : (
                      <p className="font-serif text-2xl font-bold leading-snug text-ink">{article.title}</p>
                    )}
                    {article.description && (
                      <p className="mt-2 line-clamp-2 max-w-[66ch] text-[15px] leading-relaxed text-body">
                        {article.description}
                      </p>
                    )}
                    <p className="mt-3 text-xs text-muted">
                      {article.sourceName ? `Sumber: ${article.sourceName}` : 'Sumber tidak tercantum'}
                      {article.id && (
                        <>
                          {' · '}
                          <Link
                            href={{ pathname: '/article', query: { id: article.id } }}
                            className="text-teal underline underline-offset-2 hover:text-ink focus:outline-none focus-visible:ring-2 focus-visible:ring-teal"
                          >
                            Baca artikel
                          </Link>
                        </>
                      )}
                    </p>
                  </div>
                </li>
              );
            })}
          </ol>
        ) : error ? null : (
          <div className="mt-8 border-l-4 border-teal bg-lime-tint px-5 py-6">
            <p className="font-serif text-xl font-bold text-ink">Tidak ada berita yang cocok.</p>
            <p className="mt-2 max-w-prose text-sm text-body">
              Coba kata kunci lain atau pilih kategori yang berbeda.
            </p>
            {(initialQuery || initialCategory) && (
              <Link
                href="/"
                className="mt-4 inline-flex min-h-[44px] items-center rounded-sm bg-teal px-5 text-sm font-semibold text-cream transition hover:bg-ink focus:outline-none focus-visible:ring-2 focus-visible:ring-teal focus-visible:ring-offset-2"
              >
                Tampilkan semua berita
              </Link>
            )}
          </div>
        )}
      </main>
    </div>
  );
}
