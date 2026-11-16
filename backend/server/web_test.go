package server

import (
	"github.com/hyskr/easyread/backend/exporter"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestHTTPWebImportAndOfflineExport(t *testing.T) {
	a, s := testApp(t)
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head><title>Web sample</title></head><body><article><p>` + strings.Repeat("Static article with bold words. ", 15) + `</p></article></body></html>`))
	}))
	defer source.Close()
	code, result := call(t, s, a, "POST", "/api/import-url", Object{"ref": source.URL, "kind": "web", "translate": false}, true)
	if code != 200 {
		t.Fatal(code, result)
	}
	id := text(result["id"])
	resp, err := s.Client().Get(s.URL + "/p/" + id + "/source.html")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), "data-easyread-run") {
		t.Fatal(resp.StatusCode, string(body))
	}
	ws, err := a.Library.Workspace(id)
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../../easyread/web")
	if err != nil {
		t.Fatal(err)
	}
	output, _, err := exporter.HTML(ws, root, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "web_snapshot") {
		t.Fatal("snapshot missing from offline document")
	}
}
