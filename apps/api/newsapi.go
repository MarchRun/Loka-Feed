package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

const newsBodyLimit = 2 << 20

type newsClient struct {
	key     string
	baseURL string
	http    *http.Client
}

type newsFailure struct {
	status  int
	code    string
	message string
}

func newNewsClient(key string) *newsClient {
	return &newsClient{
		key: key, baseURL: "https://newsapi.org",
		http: &http.Client{
			Timeout: 8 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func upstreamFailure() *newsFailure {
	return &newsFailure{http.StatusBadGateway, "news_upstream_error", "Layanan berita sedang bermasalah."}
}

func validArticleURL(raw string) string {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return ""
	}
	return raw
}

func (c *newsClient) fetch(ctx context.Context, query, category string) ([]Article, *newsFailure) {
	path := "/v2/top-headlines"
	params := url.Values{"pageSize": {"100"}}
	if query != "" && category == "" {
		path = "/v2/everything"
		params.Set("sortBy", "publishedAt")
	} else {
		params.Set("country", "us")
	}
	if query != "" {
		params.Set("q", query)
	}
	if category != "" {
		params.Set("category", category)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path+"?"+params.Encode(), nil)
	if err != nil {
		return nil, upstreamFailure()
	}
	req.Header.Set("X-Api-Key", c.key)
	res, err := c.http.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, &newsFailure{http.StatusGatewayTimeout, "news_timeout", "Layanan berita melewati batas waktu."}
		}
		return nil, upstreamFailure()
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests {
		return nil, &newsFailure{http.StatusServiceUnavailable, "news_unavailable", "Layanan berita sementara tidak tersedia."}
	}
	if res.StatusCode != http.StatusOK {
		return nil, upstreamFailure()
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, newsBodyLimit+1))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, &newsFailure{http.StatusGatewayTimeout, "news_timeout", "Layanan berita melewati batas waktu."}
		}
		return nil, upstreamFailure()
	}
	if len(body) > newsBodyLimit {
		return nil, upstreamFailure()
	}
	var data struct {
		Status   string `json:"status"`
		Code     string `json:"code"`
		Articles []struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			Content     string `json:"content"`
			URL         string `json:"url"`
			URLToImage  string `json:"urlToImage"`
			Author      string `json:"author"`
			PublishedAt string `json:"publishedAt"`
			Source      struct {
				Name string `json:"name"`
			} `json:"source"`
		} `json:"articles"`
	}
	if json.Unmarshal(body, &data) != nil {
		return nil, upstreamFailure()
	}
	if data.Status != "ok" {
		if data.Code == "rateLimited" || data.Code == "maximumResultsReached" {
			return nil, &newsFailure{http.StatusServiceUnavailable, "news_unavailable", "Layanan berita sementara tidak tersedia."}
		}
		return nil, upstreamFailure()
	}
	articles := make([]Article, 0)
	seen := make(map[string]bool)
	for _, raw := range data.Articles {
		sourceURL := validArticleURL(raw.URL)
		title := strings.TrimSpace(raw.Title)
		publishedAt, err := time.Parse(time.RFC3339, raw.PublishedAt)
		if sourceURL == "" || title == "" || err != nil {
			continue
		}
		hash := sha256.Sum256([]byte(sourceURL))
		id := hex.EncodeToString(hash[:])
		if seen[id] {
			continue
		}
		seen[id] = true
		articles = append(articles, Article{
			ID: id, Title: title, Description: raw.Description, Content: raw.Content,
			URL: sourceURL, URLToImage: validArticleURL(raw.URLToImage),
			Author: raw.Author, SourceName: raw.Source.Name, PublishedAt: publishedAt,
		})
		if len(articles) == 100 {
			break
		}
	}
	slices.SortFunc(articles, func(a, b Article) int {
		if order := b.PublishedAt.Compare(a.PublishedAt); order != 0 {
			return order
		}
		return strings.Compare(a.ID, b.ID)
	})
	return articles, nil
}
