package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
)

type antaraClient struct {
	baseURL string
	http    *http.Client
}

func newAntaraClient() *antaraClient {
	return &antaraClient{
		baseURL: "https://www.antaranews.com",
		http: &http.Client{
			Timeout:       8 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

func (*antaraClient) info() SourceInfo {
	return SourceInfo{"antara", "ANTARA News", []Category{
		{"general", "Umum / Terkini"}, {"business", "Ekonomi"}, {"technology", "Teknologi"},
		{"sports", "Olahraga"}, {"entertainment", "Hiburan"},
	}}
}

var antaraFeeds = map[string]string{
	"": "terkini", "general": "terkini", "business": "ekonomi",
	"technology": "tekno", "sports": "olahraga", "entertainment": "hiburan",
}

var rssTags = regexp.MustCompile(`<[^>]*>`)

func rssText(value string) string {
	// Text extraction only; output is never rendered as HTML.
	return strings.Join(strings.Fields(html.UnescapeString(rssTags.ReplaceAllString(value, " "))), " ")
}

func rssDate(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC1123Z, time.RFC1123, "Mon, 2 Jan 2006 15:04:05 -0700", "Mon, 2 Jan 2006 15:04:05 MST"} {
		if date, err := time.Parse(layout, strings.TrimSpace(value)); err == nil {
			return date, nil
		}
	}
	return time.Time{}, errors.New("invalid RSS date")
}

func (c *antaraClient) fetch(ctx context.Context, query, category string) ([]Article, *newsFailure) {
	feed, exists := antaraFeeds[category]
	if !exists {
		return nil, &newsFailure{400, "invalid_query", "Parameter pencarian tidak valid."}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/rss/"+feed+".xml", nil)
	if err != nil {
		return nil, upstreamFailure()
	}
	res, err := c.http.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, &newsFailure{504, "news_timeout", "Layanan berita melewati batas waktu."}
		}
		return nil, upstreamFailure()
	}
	defer res.Body.Close()
	if res.StatusCode == 429 {
		return nil, &newsFailure{503, "news_unavailable", "Layanan berita sementara tidak tersedia."}
	}
	if res.StatusCode != 200 {
		return nil, upstreamFailure()
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, newsBodyLimit+1))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, &newsFailure{504, "news_timeout", "Layanan berita melewati batas waktu."}
		}
		return nil, upstreamFailure()
	}
	if len(body) > newsBodyLimit {
		return nil, upstreamFailure()
	}
	var rss struct {
		XMLName xml.Name `xml:"rss"`
		Version string   `xml:"version,attr"`
		Channel *struct {
			Items []struct {
				Title       string `xml:"title"`
				Link        string `xml:"link"`
				Date        string `xml:"pubDate"`
				Description string `xml:"description"`
				Content     string `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
				Author      string `xml:"author"`
				Creator     string `xml:"http://purl.org/dc/elements/1.1/ creator"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	if xml.Unmarshal(body, &rss) != nil || rss.Version != "2.0" || rss.Channel == nil {
		return nil, upstreamFailure()
	}
	articles := make([]Article, 0)
	seen := make(map[string]bool)
	for _, item := range rss.Channel.Items {
		link := validArticleURL(item.Link)
		parsed, err := url.Parse(link)
		if err != nil || parsed == nil || link == "" {
			continue
		}
		host := strings.ToLower(parsed.Hostname())
		if host != "antaranews.com" && !strings.HasSuffix(host, ".antaranews.com") {
			continue
		}
		date, err := rssDate(item.Date)
		title := rssText(item.Title)
		if err != nil || title == "" {
			continue
		}
		description := item.Content
		if strings.TrimSpace(description) == "" {
			description = item.Description
		}
		description = rssText(description)
		if query != "" && !strings.Contains(strings.ToLower(title), query) &&
			!strings.Contains(strings.ToLower(description), query) {
			continue
		}
		hash := sha256.Sum256([]byte(link))
		id := hex.EncodeToString(hash[:])
		if seen[id] {
			continue
		}
		seen[id] = true
		group := category
		if group == "" {
			group = "general"
		}
		author := item.Creator
		if author == "" {
			author = item.Author
		}
		articles = append(articles, Article{
			ID: id, Title: title, Description: description, URL: link,
			SourceName: "ANTARA News", Category: group, Author: rssText(author), PublishedAt: date,
		})
	}
	slices.SortFunc(articles, func(a, b Article) int {
		if order := b.PublishedAt.Compare(a.PublishedAt); order != 0 {
			return order
		}
		return strings.Compare(a.ID, b.ID)
	})
	if len(articles) > 100 {
		articles = articles[:100]
	}
	return articles, nil
}
