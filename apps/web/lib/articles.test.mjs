import assert from 'node:assert/strict';
import test from 'node:test';
import { articleQuery, fetchArticles, fetchSource } from './articles.js';

test('article query validation', () => {
  assert.deepEqual(articleQuery({ q: ' Web ', category: ' TECHNOLOGY ' }, false, [{ value: 'technology', label: 'Teknologi' }]), { q: 'Web', category: 'technology' });
  assert.deepEqual(articleQuery({}), { q: '', category: '' });
  assert.equal(articleQuery({ q: ['a', 'b'] }), null);
  assert.equal(articleQuery({ category: 'invalid' }), null);
  assert.equal(articleQuery({ other: 'value' }), null);
  assert.equal(articleQuery({ q: '界'.repeat(201) }), null);
  assert.ok(articleQuery({ q: '界'.repeat(200) }));
  assert.deepEqual(articleQuery({ id: 'demo-technology' }, true), { id: 'demo-technology' });
  assert.equal(articleQuery({ id: '../private' }, true), null);
  assert.equal(articleQuery({ id: ['a', 'b'] }, true), null);
});

test('categories follow source metadata and malformed sources fail safely', async () => {
  const categories = [{ value: 'business', label: 'Ekonomi' }];
  assert.ok(articleQuery({ category: 'business' }, false, categories));
  assert.equal(articleQuery({ category: 'science' }, false, categories), null);
  const original = globalThis.fetch;
  try {
    globalThis.fetch = async () => ({
      ok: true, status: 200,
      json: async () => ({ source: { id: 'antara', name: 'ANTARA News', categories } }),
    });
    assert.deepEqual(await fetchSource(), { id: 'antara', name: 'ANTARA News', categories });
    for (const source of [null, {}, { id: 'x', name: 'X', categories: null },
      { id: 'x', name: 'X', categories: [null] },
      { id: 'x', name: 'X', categories: [...categories, ...categories] }]) {
      globalThis.fetch = async () => ({ ok: true, status: 200, json: async () => ({ source }) });
      assert.equal(await fetchSource(), null);
    }
    globalThis.fetch = async () => { throw new Error('private configuration'); };
    assert.equal(await fetchSource(), null);
  } finally {
    globalThis.fetch = original;
  }
});

test('server fetch returns safe errors and applies timeout', async () => {
  const original = globalThis.fetch;
  try {
    globalThis.fetch = async (url, options) => {
      assert.equal(new URL(url).pathname, '/api/articles');
      assert.equal(options.cache, 'no-store');
      assert.ok(options.signal instanceof AbortSignal);
      return { ok: true, status: 200, json: async () => ({ articles: [] }) };
    };
    assert.deepEqual(await fetchArticles('/api/articles'), { data: { articles: [] }, status: 200 });
    globalThis.fetch = async () => ({ ok: false, status: 504 });
    assert.deepEqual(await fetchArticles('/api/articles'), { data: null, status: 504 });
    globalThis.fetch = async () => { throw new Error('private upstream error'); };
    assert.deepEqual(await fetchArticles('/api/articles'), { data: null, status: 503 });
  } finally {
    globalThis.fetch = original;
  }
});
