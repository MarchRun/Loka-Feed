export function articleQuery(query, detail = false, categories = []) {
  const allowed = detail ? ['id'] : ['q', 'category'];
  for (const [key, value] of Object.entries(query)) {
    if (!allowed.includes(key) || typeof value !== 'string') return null;
  }
  if (detail) {
    const id = query.id || '';
    return /^[a-zA-Z0-9-]{1,100}$/.test(id) ? { id } : null;
  }
  const q = (query.q || '').trim();
  const category = (query.category || '').trim().toLowerCase();
  if ([...q].length > 200 || (category !== '' && !categories.some((item) => item.value === category))) {
    return null;
  }
  return { q, category };
}

export async function fetchSource() {
  const result = await fetchArticles('/api/source');
  const source = result.data?.source;
  if (!source || typeof source.id !== 'string' || !source.id ||
      typeof source.name !== 'string' || !source.name ||
      !Array.isArray(source.categories) || source.categories.length === 0 ||
      !source.categories.every((item) => item && typeof item.value === 'string' &&
        /^[a-z][a-z0-9-]*$/.test(item.value) && typeof item.label === 'string' && item.label.trim()) ||
      new Set(source.categories.map((item) => item.value)).size !== source.categories.length) {
    return null;
  }
  return { id: source.id, name: source.name, categories: source.categories };
}

export async function fetchArticles(path) {
  try {
    const base = process.env.API_BASE_URL || 'http://127.0.0.1:8080';
    const response = await fetch(new URL(path, base), {
      signal: AbortSignal.timeout(10000),
      cache: 'no-store',
    });
    if (!response.ok) {
      return { data: null, status: response.status };
    }
    return { data: await response.json(), status: response.status };
  } catch {
    return { data: null, status: 503 };
  }
}
