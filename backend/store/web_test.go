package store

import "testing"

func TestWebReimportRetainsNotes(t *testing.T) {
	lib, err := NewLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	meta := Object{"source_type": "web", "pages": []any{Object{"n": 1}}, "title_en": "Web"}
	ws, fresh, err := lib.ImportWeb("https://example.org/article#one", "<html>first</html>", meta, []any{Object{"id": "web-0001", "type": "para", "page": 1, "en": "Original"}})
	if err != nil || !fresh {
		t.Fatalf("%v %v", fresh, err)
	}
	err = ws.Update("reader", func(r Object) error { r["paper_note"] = Object{"body": "My note"}; return nil })
	if err != nil {
		t.Fatal(err)
	}
	again, fresh, err := lib.ImportWeb("https://example.org/article#two", "<html>new</html>", meta, nil)
	if err != nil || fresh || again.ID() != ws.ID() {
		t.Fatalf("%v %v", fresh, err)
	}
	reader, err := again.Load("reader")
	if err != nil || obj(reader["paper_note"])["body"] != "My note" {
		t.Fatal(reader, err)
	}
}
