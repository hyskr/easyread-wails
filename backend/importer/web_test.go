package importer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebSnapshotPreservesRunsAndRemovesActiveContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/photo.png" {
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte("image"))
			return
		}
		w.Write([]byte(`<html><head><title>Article</title></head><body><article><h1>Article</h1><p>Hello <strong>bold</strong> ` + strings.Repeat("paragraph ", 25) + `</p><img src="/photo.png"><a href="javascript:alert(1)" onclick="attack()">bad</a><script>attack()</script><iframe src="/evil"></iframe></article></body></html>`))
	}))
	defer server.Close()
	snapshot, meta, blocks, err := FetchWeb(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"<script", "<iframe", "onclick=", "javascript:"} {
		if strings.Contains(snapshot, bad) {
			t.Fatalf("active content %s", bad)
		}
	}
	if !strings.Contains(snapshot, "data:image/png;base64,") || !strings.Contains(snapshot, "data-easyread-run=") || !strings.Contains(snapshot, "<strong>") {
		t.Fatal(snapshot)
	}
	if meta["source_type"] != "web" || len(blocks) < 3 {
		t.Fatalf("%v %v", meta, blocks)
	}
}
