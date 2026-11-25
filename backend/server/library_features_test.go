package server

import (
	"encoding/json"
	"fmt"
	"github.com/hyskr/easyread/backend/store"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFolderWatchOnlyImportsCompleteStableFiles(t *testing.T) {
	a, s := testApp(t)
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	code, res := call(t, s, a, "POST", "/api/migrate/watch", Object{"path": dir, "enabled": true, "keep_filename": true}, true)
	if code != 200 {
		t.Fatal(code, res)
	}
	path := filepath.Join(dir, "paper.pdf")
	put := func(data []byte) {
		t.Helper()
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	count := func(want int) {
		t.Helper()
		rows, err := a.Library.All()
		if err != nil || len(rows) != want {
			t.Fatalf("expected %d documents, got %d (%v)", want, len(rows), err)
		}
	}
	put([]byte("%PDF-1.4 incomplete"))
	a.pollFolders()
	a.pollFolders()
	count(0)
	put(tinyPDF())
	a.pollFolders()
	count(0)
	a.pollFolders()
	count(1)
	rows, err := a.Library.List()
	if err != nil || rows[0]["display_title"] != "paper.pdf" {
		t.Fatal(rows, err)
	}
	tags := array(rows[0]["tags"])
	if len(tags) != 1 || tags[0] != filepath.Base(dir) {
		t.Fatal(tags)
	}
	code, res = call(t, s, a, "POST", "/api/migrate/watch", Object{"path": dir, "enabled": false}, true)
	if code != 200 {
		t.Fatal(code, res)
	}
	if err = os.WriteFile(filepath.Join(dir, "another.pdf"), append(tinyPDF(), byte(' ')), 0600); err != nil {
		t.Fatal(err)
	}
	a.pollFolders()
	a.pollFolders()
	count(1)
}

func TestScopedChatAndClassification(t *testing.T) {
	a, s := testApp(t)
	makePaper := func(name, original string) *store.Workspace {
		t.Helper()
		ws, _, err := a.Library.ImportPDF([]byte("%PDF-"+name), name, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := ws.Update("paper", func(p Object) error {
			p["blocks"] = []any{Object{"id": "p1", "type": "para", "en": original, "page": 1}}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		return ws
	}
	good := makePaper("selected", "The trial doubled accuracy.")
	other := makePaper("other", "OUTSIDE_THE_SELECTION.")
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			return
		}
		raw, _ := io.ReadAll(r.Body)
		prompt := string(raw)
		answer := "Evidence [1]."
		if strings.Contains(prompt, "OUTSIDE_THE_SELECTION") {
			t.Error("unselected text sent to the model")
		}
		switch {
		case strings.Contains(prompt, "Suggest up to three categories"):
			answer = `{"suggestions":[{"name":"Trials","reason":"Original trial."}]}`
		case strings.Contains(prompt, "filing criterion"):
			answer = `{"match":true,"source":1,"quote":"doubled accuracy"}`
			if strings.Contains(prompt, "fabricated") {
				answer = `{"match":true,"source":1,"quote":"unsupported quote"}`
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Object{"choices": []any{Object{"message": Object{"content": answer}, "finish_reason": "stop"}}})
	}))
	defer model.Close()
	_, err := a.Config.Save(Object{"engine": "openai", "openai": Object{"base_url": model.URL, "model": "local", "api": "chat"}, "chat": Object{"default": "local", "models": []any{Object{"id": "local", "engine": "openai", "model": "local", "base_url": model.URL}}}})
	if err != nil {
		t.Fatal(err)
	}
	post := func(action string, b Object) Object {
		t.Helper()
		code, x := call(t, s, a, "POST", "/api/p/"+good.ID()+"/"+action, b, true)
		if code != 200 {
			t.Fatal(action, code, x)
		}
		return x
	}
	if _, err = other.PatchItem(Object{"ai_indexed": false}); err != nil {
		t.Fatal(err)
	}
	body := strings.NewReader(fmt.Sprintf(`{"text":"accuracy","library_scope":{"papers":[%q]} }`, good.ID()))
	req, _ := http.NewRequest("POST", s.URL+"/api/p/"+other.ID()+"/chat", body)
	req.Header.Set("X-Token", a.Token)
	resp, err := s.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	stream, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(stream), `"sources"`) || !strings.Contains(string(stream), `"done":true`) {
		t.Fatal(string(stream))
	}
	conv, err := other.Load("chat")
	if err != nil {
		t.Fatal(err)
	}
	messages := array(object(array(conv["threads"])[0])["messages"])
	if len(messages) != 2 || len(array(object(messages[1])["sources"])) != 1 || object(messages[0])["library_scope"] == nil {
		t.Fatal(messages)
	}
	if m := post("match-category", Object{"rule": "trial evidence"}); m["status"] != "match" || m["anchor"] != "p1" {
		t.Fatal(m)
	}
	if code, _ := call(t, s, a, "POST", "/api/p/"+good.ID()+"/match-category", Object{"rule": "fabricated"}, true); code == 200 {
		t.Fatal("accepted an invented quote")
	}
	review := post("suggest-categories", Object{})
	if review["state"] != "pending" {
		t.Fatal(review)
	}
	item, _ := good.Load("item")
	if len(array(item["tags"])) != 0 {
		t.Fatal("suggestion changed tags")
	}
	if _, err := good.PatchItem(Object{"add_category": "Existing"}); err != nil {
		t.Fatal(err)
	}
	result := post("review-categories", Object{"id": review["id"], "categories": []any{"Trials"}})
	if fmt.Sprint(result["tags"]) != "[Existing Trials]" {
		t.Fatal(result)
	}
}
