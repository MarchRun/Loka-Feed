package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

func TestArticlesHTTP(t *testing.T) {
	server := httptest.NewServer(routes())
	defer server.Close()
	for _, tc := range []struct {
		name, method, path, code string
		status                   int
		count                    int
	}{
		{"list", "GET", "/api/articles", "", 200, 2},
		{"search", "GET", "/api/articles?q=WEB", "", 200, 1},
		{"description", "GET", "/api/articles?q=SAINS", "", 200, 1},
		{"category", "GET", "/api/articles?category=%20TECHNOLOGY%20", "", 200, 1},
		{"combined", "GET", "/api/articles?q=web&category=science", "", 200, 0},
		{"empty", "GET", "/api/articles?category=business", "", 200, 0},
		{"detail", "GET", "/api/articles/demo-technology", "", 200, -1},
		{"missing", "GET", "/api/articles/missing", "article_not_found", 404, -1},
		{"case-sensitive", "GET", "/api/articles/DEMO-technology", "article_not_found", 404, -1},
		{"unknown-query", "GET", "/api/articles?page=1", "invalid_query", 400, -1},
		{"duplicate-query", "GET", "/api/articles?q=a&q=b", "invalid_query", 400, -1},
		{"duplicate-category", "GET", "/api/articles?category=&category=", "invalid_query", 400, -1},
		{"bad-category", "GET", "/api/articles?category=unknown", "invalid_query", 400, -1},
		{"bad-utf8", "GET", "/api/articles?q=%FF", "invalid_query", 400, -1},
		{"bad-escape", "GET", "/api/articles?q=%ZZ", "invalid_query", 400, -1},
		{"bad-separator", "GET", "/api/articles?q=a;b", "invalid_query", 400, -1},
		{"unicode-limit", "GET", "/api/articles?q=" + url.QueryEscape(strings.Repeat("界", 200)), "", 200, 0},
		{"over-limit", "GET", "/api/articles?q=" + url.QueryEscape(strings.Repeat("界", 201)), "invalid_query", 400, -1},
		{"detail-query", "GET", "/api/articles/demo-technology?q=", "invalid_query", 400, -1},
		{"head-list", "HEAD", "/api/articles", "", 200, -1},
		{"head-detail", "HEAD", "/api/articles/demo-technology", "", 200, -1},
		{"method-list", "POST", "/api/articles", "", 405, -1},
		{"method-detail", "POST", "/api/articles/demo-technology", "", 405, -1},
		{"unknown-route", "GET", "/missing", "", 404, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, server.URL+tc.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			res, err := server.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			body, err := io.ReadAll(res.Body)
			if err != nil {
				t.Fatal(err)
			}
			if res.StatusCode != tc.status {
				t.Fatalf("status = %d, want %d: %s", res.StatusCode, tc.status, body)
			}
			if tc.method == "HEAD" {
				if len(body) != 0 {
					t.Fatalf("HEAD body: %s", body)
				}
				return
			}
			if tc.code != "" || tc.status == 200 {
				if res.Header.Get("Content-Type") != "application/json" {
					t.Fatal("expected JSON content type")
				}
				var data struct {
					Articles []Article `json:"articles"`
					Article  Article   `json:"article"`
					Error    struct {
						Code string `json:"code"`
					} `json:"error"`
				}
				if err := json.Unmarshal(body, &data); err != nil {
					t.Fatal(err)
				}
				if data.Error.Code != tc.code {
					t.Fatalf("error code = %q, want %q", data.Error.Code, tc.code)
				}
				if tc.count >= 0 && (data.Articles == nil || len(data.Articles) != tc.count) {
					t.Fatalf("unexpected articles: %s", body)
				}
				if tc.name == "detail" && data.Article.ID != "demo-technology" {
					t.Fatalf("unexpected article: %s", body)
				}
			}
		})
	}
}

func TestStoreOrderAndIsolation(t *testing.T) {
	data := demoArticles()
	older := data[1]
	sameTime := data[0]
	sameTime.ID = "aaa"
	store := &articleStore{articles: []Article{older, data[0], sameTime}}
	result := store.list("", "")
	if result[0].ID != "aaa" || result[1].ID != "demo-technology" || result[2].ID != older.ID {
		t.Fatalf("unexpected order: %+v", result)
	}
	result[0].Title = "changed"
	article, found := store.find("aaa")
	if !found || article.Title == "changed" {
		t.Fatal("list exposed internal data")
	}
	article.Title = "changed again"
	original, _ := store.find("aaa")
	if original.Title == article.Title {
		t.Fatal("detail exposed internal data")
	}
	if store.articles[0].ID != older.ID {
		t.Fatal("sorting modified internal order")
	}
	if got := (&articleStore{}).list("", ""); got == nil || len(got) != 0 {
		t.Fatal("empty store must return non-nil empty slice")
	}
}

func TestConcurrentArticleReads(t *testing.T) {
	store := &articleStore{articles: demoArticles()}
	var workers sync.WaitGroup
	for i := 0; i < 20; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for j := 0; j < 20; j++ {
				result := store.list("", "")
				result[0].Title = "local change"
				if article, found := store.find("demo-technology"); !found || article.Title == "local change" {
					t.Error("concurrent result changed store")
				}
			}
		}()
	}
	workers.Wait()
}
