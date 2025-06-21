package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealth(t *testing.T) {
	server := httptest.NewServer(routes())
	defer server.Close()

	for _, tc := range []struct {
		method string
		path   string
		status int
	}{
		{http.MethodGet, "/health", http.StatusOK},
		{http.MethodHead, "/health", http.StatusOK},
		{http.MethodPost, "/health", http.StatusMethodNotAllowed},
		{http.MethodGet, "/unknown", http.StatusNotFound},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
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
				t.Fatalf("status = %d, want %d", res.StatusCode, tc.status)
			}
			if tc.status == http.StatusOK {
				if res.Header.Get("Content-Type") != "application/json" {
					t.Fatal("expected JSON content type")
				}
				if tc.method == http.MethodGet && string(body) != "{\"status\":\"ok\"}\n" {
					t.Fatalf("unexpected body: %q", body)
				}
				if tc.method == http.MethodHead && len(body) != 0 {
					t.Fatal("HEAD must have no body")
				}
			}
		})
	}
}
