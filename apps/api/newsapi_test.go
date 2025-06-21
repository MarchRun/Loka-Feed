package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewsMappingAndNormalization(t *testing.T) {
	for _, tc := range []struct{ query, category, path string }{
		{"", "", "/v2/top-headlines"},
		{"Go & web", "", "/v2/everything"},
		{"Go", "technology", "/v2/top-headlines"},
		{"", "science", "/v2/top-headlines"},
	} {
		t.Run(tc.path+tc.query+tc.category, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.path || r.URL.Query().Get("q") != tc.query ||
					r.URL.Query().Get("category") != tc.category || r.URL.Query().Get("pageSize") != "100" {
					t.Errorf("unexpected request: %s", r.URL)
				}
				if r.Header.Get("X-Api-Key") != "test-secret" || r.URL.Query().Get("apiKey") != "" {
					t.Error("key must only appear in header")
				}
				if tc.path == "/v2/everything" {
					if r.URL.Query().Get("sortBy") != "publishedAt" || r.URL.Query().Get("country") != "" {
						t.Error("invalid everything parameters")
					}
				} else if r.URL.Query().Get("country") != "us" {
					t.Error("country missing")
				}
				fmt.Fprint(w, `{"status":"ok","articles":[
					{"title":" Old ","url":" https://example.org/old ","publishedAt":"2025-07-01T00:00:00Z","author":null,"source":{"name":null},"urlToImage":"javascript:alert(1)"},
					{"title":"New","url":"https://example.org/new","publishedAt":"2025-07-02T00:00:00Z"},
					{"title":"Duplicate","url":"https://example.org/new","publishedAt":"2025-07-02T00:00:00Z"},
					{"title":"Bad date","url":"https://example.org/bad","publishedAt":"invalid"},
					{"title":"Bad URL","url":"javascript:alert(1)","publishedAt":"2025-07-02T00:00:00Z"},
					{"title":"","url":"https://example.org/empty","publishedAt":"2025-07-02T00:00:00Z"}
				]}`)
			}))
			defer server.Close()
			client := newNewsClient("test-secret")
			client.baseURL = server.URL
			articles, failure := client.fetch(context.Background(), tc.query, tc.category)
			if failure != nil || len(articles) != 2 {
				t.Fatalf("articles=%+v failure=%+v", articles, failure)
			}
			hash := sha256.Sum256([]byte("https://example.org/new"))
			if articles[0].ID != hex.EncodeToString(hash[:]) || articles[1].Title != "Old" ||
				articles[1].URLToImage != "" || articles[0].Category != "" || articles[1].Author != "" {
				t.Fatalf("unexpected normalized data: %+v", articles)
			}
		})
	}
}

func TestNewsFailures(t *testing.T) {
	for _, tc := range []struct {
		name, body, code string
		status, want     int
	}{
		{"quota", "", "news_unavailable", 429, 503},
		{"unauthorized", "secret provider details", "news_upstream_error", 401, 502},
		{"redirect", "", "news_upstream_error", 302, 502},
		{"bad-json", "{", "news_upstream_error", 200, 502},
		{"trailing-json", `{"status":"ok"}{}`, "news_upstream_error", 200, 502},
		{"too-large", strings.Repeat(" ", newsBodyLimit+1), "news_upstream_error", 200, 502},
		{"provider-error", `{"status":"error","code":"apiKeyInvalid"}`, "news_upstream_error", 200, 502},
		{"provider-quota", `{"status":"error","code":"rateLimited"}`, "news_unavailable", 200, 503},
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
			client := newNewsClient("test-secret")
			client.baseURL = server.URL
			handler := articleRoutes(&articleStore{source: client})
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, httptest.NewRequest("GET", "/api/articles", nil))
			if res.Code != tc.want || !strings.Contains(res.Body.String(), tc.code) || requests != 1 {
				t.Fatalf("unexpected response %d: %s, requests=%d", res.Code, res.Body, requests)
			}
			if strings.Contains(res.Body.String(), "test-secret") || strings.Contains(res.Body.String(), "secret provider details") {
				t.Fatal("response leaked secret")
			}
		})
	}
}

func TestNewsTimeoutAndCancellation(t *testing.T) {
	cancelled := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		cancelled <- struct{}{}
	}))
	defer server.Close()
	client := newNewsClient("test-secret")
	client.baseURL = server.URL
	client.http.Timeout = 30 * time.Millisecond
	_, failure := client.fetch(context.Background(), "", "")
	if failure == nil || failure.status != 504 {
		t.Fatalf("expected timeout, got %+v", failure)
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("upstream request was not cancelled")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, failure := client.fetch(ctx, "", ""); failure == nil {
		t.Fatal("cancelled request unexpectedly succeeded")
	}
}

func TestNewsStoreAndListIsolation(t *testing.T) {
	store := &articleStore{}
	for i := 0; i < 501; i++ {
		store.remember([]Article{{ID: fmt.Sprint(i), Title: "original"}})
	}
	if len(store.articles) != 500 {
		t.Fatal("store exceeded capacity")
	}
	if _, found := store.find("0"); found {
		t.Fatal("oldest ID not evicted")
	}
	store.remember([]Article{{ID: "1", Title: "updated"}, {ID: "502"}})
	if _, found := store.find("1"); found {
		t.Fatal("update incorrectly changed insertion order")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"status":"ok","articles":[]}`)
	}))
	defer server.Close()
	client := newNewsClient("test-secret")
	client.baseURL = server.URL
	store.source = client
	res := httptest.NewRecorder()
	articleRoutes(store).ServeHTTP(res, httptest.NewRequest("GET", "/api/articles", nil))
	var data struct{ Articles []Article }
	if json.Unmarshal(res.Body.Bytes(), &data) != nil || data.Articles == nil || len(data.Articles) != 0 {
		t.Fatalf("list included previously stored results: %s", res.Body)
	}
}

func TestNewsConfiguration(t *testing.T) {
	for _, source := range []string{"", "demo"} {
		store, err := configuredStore(source, "")
		if err != nil || store.source.info().ID != "demo" || len(store.articles) == 0 {
			t.Fatal("demo requires no key")
		}
	}
	for _, source := range []string{"newsapi", "unknown"} {
		if _, err := configuredStore(source, ""); err == nil {
			t.Fatalf("%s must fail without valid configuration", source)
		}
	}
	store, err := configuredStore("newsapi", "test-secret")
	if err != nil || store.source.info().ID != "newsapi" || len(store.articles) != 0 {
		t.Fatal("live mode must not include demo articles")
	}
}
