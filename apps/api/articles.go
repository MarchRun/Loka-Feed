package main

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type Article struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Content     string    `json:"content"`
	Category    string    `json:"category"`
	SourceName  string    `json:"sourceName"`
	URL         string    `json:"url"`
	URLToImage  string    `json:"urlToImage"`
	Author      string    `json:"author"`
	PublishedAt time.Time `json:"publishedAt"`
	Views       int       `json:"views"`
}

type articleStore struct {
	mu       sync.RWMutex
	articles []Article
	source   articleSource
}

func demoArticles() []Article {
	return []Article{
		{
			ID: "demo-technology", Title: "Mengenal teknologi web",
			Description: "Artikel contoh tentang teknologi, bukan berita aktual.",
			Content:     "Data demonstrasi LokaFeed untuk menguji API artikel.",
			Category:    "technology", SourceName: "Contoh LokaFeed",
			PublishedAt: time.Date(2025, 7, 15, 9, 0, 0, 0, time.UTC),
		},
		{
			ID: "demo-science", Title: "Mengenal sains",
			Description: "Artikel contoh tentang sains, bukan berita aktual.",
			Content:     "Data demonstrasi LokaFeed untuk menguji filter kategori.",
			Category:    "science", SourceName: "Contoh LokaFeed",
			PublishedAt: time.Date(2025, 7, 14, 9, 0, 0, 0, time.UTC),
		},
	}
}

func (s *articleStore) list(query, category string) []Article {
	s.mu.RLock()
	snapshot := slices.Clone(s.articles)
	s.mu.RUnlock()

	articles := make([]Article, 0)
	for _, article := range snapshot {
		if category != "" && article.Category != category {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(article.Title), query) &&
			!strings.Contains(strings.ToLower(article.Description), query) {
			continue
		}
		articles = append(articles, article)
	}
	slices.SortFunc(articles, func(a, b Article) int {
		if order := b.PublishedAt.Compare(a.PublishedAt); order != 0 {
			return order
		}
		return strings.Compare(a.ID, b.ID)
	})
	return articles
}

func (s *articleStore) find(id string) (Article, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, article := range s.articles {
		if article.ID == id {
			return article, true
		}
	}
	return Article{}, false
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode JSON response: %v", err)
	}
}

func articleError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

func articleQuery(raw string, detail bool) (string, string, bool) {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return "", "", false
	}
	for key, value := range values {
		if detail || (key != "q" && key != "category") || len(value) != 1 ||
			!utf8.ValidString(value[0]) {
			return "", "", false
		}
	}
	query := strings.TrimSpace(values.Get("q"))
	category := strings.ToLower(strings.TrimSpace(values.Get("category")))
	if utf8.RuneCountInString(query) > 200 {
		return "", "", false
	}
	return strings.ToLower(query), category, true
}

func (s *articleStore) listHandler(w http.ResponseWriter, r *http.Request) {
	query, category, valid := articleQuery(r.URL.RawQuery, false)
	source := s.activeSource()
	if !valid || !supportsCategory(source, category) {
		articleError(w, http.StatusBadRequest, "invalid_query", "Parameter pencarian tidak valid.")
		return
	}
	articles, failure := source.fetch(r.Context(), query, category)
	if failure != nil {
		articleError(w, failure.status, failure.code, failure.message)
		return
	}
	s.remember(articles)
	writeJSON(w, http.StatusOK, map[string]any{"articles": articles})
}

func (s *articleStore) remember(articles []Article) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, article := range articles {
		index := slices.IndexFunc(s.articles, func(stored Article) bool { return stored.ID == article.ID })
		if index >= 0 {
			s.articles[index] = article
		} else {
			s.articles = append(s.articles, article)
		}
	}
	if len(s.articles) > 500 {
		s.articles = slices.Clone(s.articles[len(s.articles)-500:])
	}
}

func (s *articleStore) detailHandler(w http.ResponseWriter, r *http.Request) {
	if _, _, valid := articleQuery(r.URL.RawQuery, true); !valid {
		articleError(w, http.StatusBadRequest, "invalid_query", "Parameter pencarian tidak valid.")
		return
	}
	article, found := s.find(r.PathValue("id"))
	if !found {
		articleError(w, http.StatusNotFound, "article_not_found", "Artikel tidak ditemukan.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"article": article})
}
