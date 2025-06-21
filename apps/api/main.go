package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func routes() http.Handler {
	return articleRoutes(&articleStore{articles: demoArticles()})
}

func articleRoutes(store *articleStore) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/source", store.sourceHandler)
	mux.HandleFunc("GET /api/articles", store.listHandler)
	mux.HandleFunc("GET /api/articles/{id}", store.detailHandler)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{\"status\":\"ok\"}\n"))
	})
	return mux
}

func configuredStore(source, key string) (*articleStore, error) {
	switch source {
	case "", "demo":
		return &articleStore{articles: demoArticles(), source: demoSource{}}, nil
	case "antara":
		return &articleStore{source: newAntaraClient()}, nil
	case "newsapi":
		if strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("NEWS_API_KEY wajib diisi untuk NEWS_SOURCE=newsapi")
		}
		return &articleStore{source: newNewsClient(strings.TrimSpace(key))}, nil
	default:
		return nil, fmt.Errorf("NEWS_SOURCE harus demo, antara, atau newsapi")
	}
}

func main() {
	store, err := configuredStore(os.Getenv("NEWS_SOURCE"), os.Getenv("NEWS_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	server := &http.Server{
		Addr:              "127.0.0.1:8080",
		Handler:           articleRoutes(store),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("LokaFeed API: http://%s", server.Addr)
	log.Fatal(server.ListenAndServe())
}
