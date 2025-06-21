package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

const sampleRSS = `<?xml version="1.0"?>
<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/" xmlns:dc="http://purl.org/dc/elements/1.1/">
<channel><title>Contoh pengujian</title>
<item><title>Contoh teknologi &amp; sains</title><link>https://www.antaranews.com/berita/1/contoh</link>
<pubDate>Tue, 1 Jul 2025 09:00:00 +0700</pubDate>
<description><![CDATA[<img src="https://example.org/image.jpg">fallback]]></description>
<content:encoded><![CDATA[<b>Ringkasan</b> contoh &amp; pengujian]]></content:encoded><dc:creator>Penulis Contoh</dc:creator></item>
<item><title>Duplikat</title><link>https://www.antaranews.com/berita/1/contoh</link><pubDate>Tue, 01 Jul 2025 09:00:00 +0700</pubDate></item>
<item><title>Contoh terbaru</title><link>https://otomotif.antaranews.com/berita/2/contoh</link><pubDate>Wed, 02 Jul 2025 09:00:00 +0700</pubDate><description><![CDATA[<p>Contoh&nbsp;lain</p>]]></description></item>
<item><title>Tanggal buruk</title><link>https://www.antaranews.com/berita/3</link><pubDate>invalid</pubDate></item>
<item><title>Domain palsu</title><link>https://antaranews.com.evil.example/berita/4</link><pubDate>Tue, 01 Jul 2025 09:00:00 +0700</pubDate></item>
<item><title>URL buruk</title><link>javascript:alert(1)</link><pubDate>Tue, 01 Jul 2025 09:00:00 +0700</pubDate></item>
</channel></rss>`

func TestAntaraLive(t *testing.T) {
	if os.Getenv("LOKAFEED_TEST_ANTARA") != "1" {
		t.Skip("set LOKAFEED_TEST_ANTARA=1 for one live RSS request")
	}
	client := newAntaraClient()
	articles, failure := client.fetch(context.Background(), "", "")
	if failure != nil || len(articles) == 0 {
		t.Fatalf("live RSS unavailable: count=%d failure=%+v", len(articles), failure)
	}
	store := &articleStore{source: client}
	store.remember(articles)
	res := httptest.NewRecorder()
	articleRoutes(store).ServeHTTP(res, httptest.NewRequest("GET", "/api/articles/"+articles[0].ID, nil))
	if res.Code != 200 {
		t.Fatalf("live detail status=%d", res.Code)
	}
	t.Logf("live RSS parsed; %d articles, local detail HTTP 200; no feed content saved", len(articles))
}

func TestAntaraMappingAndNormalization(t *testing.T) {
	for category, feed := range antaraFeeds {
		t.Run(category, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.URL.Path != "/rss/"+feed+".xml" || r.URL.RawQuery != "" || r.Header.Get("X-Api-Key") != "" {
					t.Errorf("unexpected RSS request: %s", r.URL)
				}
				fmt.Fprint(w, sampleRSS)
			}))
			defer server.Close()
			client := newAntaraClient()
			client.baseURL = server.URL
			articles, failure := client.fetch(context.Background(), "", category)
			if failure != nil || len(articles) != 2 || requests != 1 {
				t.Fatalf("articles=%+v failure=%+v requests=%d", articles, failure, requests)
			}
			if articles[0].Title != "Contoh terbaru" || articles[1].Description != "Ringkasan contoh & pengujian" ||
				articles[1].Author != "Penulis Contoh" || articles[1].URLToImage != "" || articles[1].Content != "" {
				t.Fatalf("bad normalization: %+v", articles)
			}
			handler := articleRoutes(&articleStore{source: client})
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, httptest.NewRequest("GET", "/api/articles?q=ringkasan&category="+category, nil))
			var data struct{ Articles []Article }
			if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &data) != nil || len(data.Articles) != 1 {
				t.Fatalf("filter failed: %s", res.Body)
			}
			detail := httptest.NewRecorder()
			handler.ServeHTTP(detail, httptest.NewRequest("GET", "/api/articles/"+data.Articles[0].ID, nil))
			if detail.Code != 200 || requests != 2 {
				t.Fatal("detail must use local store, not RSS")
			}
		})
	}
}

func TestAntaraFailures(t *testing.T) {
	for _, tc := range []struct {
		name, body   string
		status, want int
	}{
		{"broken", "<rss", 200, 502},
		{"html", "<html></html>", 200, 502},
		{"wrong-version", `<rss version="1.0"><channel/></rss>`, 200, 502},
		{"missing-channel", `<rss version="2.0"/>`, 200, 502},
		{"external-entity", `<!DOCTYPE rss [<!ENTITY external SYSTEM "http://localhost/private">]><rss version="2.0"><channel><title>&external;</title></channel></rss>`, 200, 502},
		{"oversized", strings.Repeat(" ", newsBodyLimit+1), 200, 502},
		{"quota", "", 429, 503},
		{"redirect", "", 302, 502},
		{"upstream", "private response", 500, 502},
		{"empty", `<rss version="2.0"><channel/></rss>`, 200, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.Header().Set("Location", "/redirected")
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			client := newAntaraClient()
			client.baseURL = server.URL
			res := httptest.NewRecorder()
			articleRoutes(&articleStore{source: client}).ServeHTTP(res, httptest.NewRequest("GET", "/api/articles", nil))
			if res.Code != tc.want || requests != 1 || strings.Contains(res.Body.String(), "private response") {
				t.Fatalf("status=%d body=%s requests=%d", res.Code, res.Body, requests)
			}
			if tc.want == 200 && !strings.Contains(res.Body.String(), `"articles":[]`) {
				t.Fatal("empty feed must return []")
			}
		})
	}
}

func TestAntaraTimeoutAndResultLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()
	client := newAntaraClient()
	client.baseURL = server.URL
	client.http.Timeout = 30 * time.Millisecond
	if _, failure := client.fetch(context.Background(), "", ""); failure == nil || failure.status != 504 {
		t.Fatalf("expected timeout, got %+v", failure)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, failure := client.fetch(ctx, "", ""); failure == nil {
		t.Fatal("cancelled request succeeded")
	}
	many := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<rss version="2.0"><channel>`)
		for i := 0; i < 110; i++ {
			fmt.Fprintf(w, `<item><title>Contoh</title><link>https://www.antaranews.com/berita/%d</link><pubDate>Tue, 01 Jul 2025 09:00:00 +0700</pubDate></item>`, i)
		}
		fmt.Fprint(w, `</channel></rss>`)
	}))
	defer many.Close()
	client.baseURL = many.URL
	client.http.Timeout = time.Second
	articles, failure := client.fetch(context.Background(), "", "")
	if failure != nil || len(articles) != 100 {
		t.Fatalf("limit failed: count=%d failure=%+v", len(articles), failure)
	}
	articles, failure = client.fetch(context.Background(), "no-match", "")
	if failure != nil || articles == nil || len(articles) != 0 {
		t.Fatal("no match must return empty slice")
	}
}

func TestSourceMetadataAndValidation(t *testing.T) {
	for _, name := range []string{"demo", "antara", "newsapi"} {
		t.Run(name, func(t *testing.T) {
			store, err := configuredStore(name, "test-secret")
			if err != nil {
				t.Fatal(err)
			}
			if name == "antara" {
				store.source.(*antaraClient).baseURL = "http://127.0.0.1:1"
			}
			if name == "newsapi" {
				store.source.(*newsClient).baseURL = "http://127.0.0.1:1"
			}
			handler := articleRoutes(store)
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, httptest.NewRequest("GET", "/api/source", nil))
			var data struct{ Source SourceInfo }
			if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &data) != nil || data.Source.ID != name ||
				strings.Contains(res.Body.String(), "test-secret") || strings.Contains(res.Body.String(), "127.0.0.1") {
				t.Fatalf("bad metadata: %s", res.Body)
			}
			want := 7
			if name == "antara" {
				want = 5
			}
			if len(data.Source.Categories) != want {
				t.Fatal("wrong categories")
			}
			invalid := httptest.NewRecorder()
			category := "unknown"
			if name == "antara" {
				category = "science"
			}
			handler.ServeHTTP(invalid, httptest.NewRequest("GET", "/api/articles?category="+category, nil))
			if invalid.Code != 400 {
				t.Fatal("unsupported category must fail before network")
			}
			method := httptest.NewRecorder()
			handler.ServeHTTP(method, httptest.NewRequest("POST", "/api/source", nil))
			if method.Code != 405 {
				t.Fatal("source must be read only")
			}
			server := httptest.NewServer(handler)
			defer server.Close()
			head, err := server.Client().Head(server.URL + "/api/source")
			if err != nil {
				t.Fatal(err)
			}
			head.Body.Close()
			if head.StatusCode != 200 {
				t.Fatal("HEAD source failed")
			}
		})
	}
}
