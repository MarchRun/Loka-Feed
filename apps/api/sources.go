package main

import (
	"context"
	"net/http"
	"slices"
)

type Category struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type SourceInfo struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Categories []Category `json:"categories"`
}

type articleSource interface {
	fetch(context.Context, string, string) ([]Article, *newsFailure)
	info() SourceInfo
}

func commonCategories() []Category {
	return []Category{
		{"business", "Bisnis"}, {"entertainment", "Hiburan"}, {"general", "Umum"},
		{"health", "Kesehatan"}, {"science", "Sains"}, {"sports", "Olahraga"}, {"technology", "Teknologi"},
	}
}

type demoSource struct{}

func (demoSource) info() SourceInfo {
	return SourceInfo{"demo", "Contoh LokaFeed", commonCategories()}
}

func (demoSource) fetch(_ context.Context, query, category string) ([]Article, *newsFailure) {
	return (&articleStore{articles: demoArticles()}).list(query, category), nil
}

func (*newsClient) info() SourceInfo {
	return SourceInfo{"newsapi", "NewsAPI", commonCategories()}
}

func (s *articleStore) activeSource() articleSource {
	if s.source == nil {
		return demoSource{}
	}
	return s.source
}

func (s *articleStore) sourceHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"source": s.activeSource().info()})
}

func supportsCategory(source articleSource, category string) bool {
	return category == "" || slices.ContainsFunc(source.info().Categories, func(c Category) bool {
		return c.Value == category
	})
}
