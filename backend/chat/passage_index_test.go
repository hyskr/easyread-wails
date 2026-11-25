package chat

import (
	"fmt"
	"github.com/hyskr/easyread/backend/store"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func indexedLibrary(t testing.TB, count int) (*store.Library, []*store.Workspace) {
	t.Helper()
	lib, err := store.NewLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	spaces := []*store.Workspace{}
	for i := 0; i < count; i++ {
		ws, _, err := lib.ImportPDF([]byte(fmt.Sprintf("%%PDF-%d", i)), fmt.Sprintf("paper-%d", i), nil)
		if err != nil {
			t.Fatal(err)
		}
		blocks := []any{}
		for n := 0; n < 24; n++ {
			blocks = append(blocks, M{"id": fmt.Sprintf("p%d", n), "en": strings.Repeat("study result synthetic ", 24), "zh": "试验结果"})
		}
		if err := ws.Update("paper", func(p M) error { p["blocks"] = blocks; return nil }); err != nil {
			t.Fatal(err)
		}
		spaces = append(spaces, ws)
	}
	return lib, spaces
}
func TestPassageIndexRefreshAndRemoval(t *testing.T) {
	lib, spaces := indexedLibrary(t, 2)
	idx := new(PassageIndex)
	include := func(string, M) bool { return true }
	_, _, _, err := idx.snapshot(lib, include)
	if err != nil {
		t.Fatal(err)
	}
	old := idx.papers[spaces[1].Root]
	if err := spaces[0].Update("reader", func(r M) error { r["edits"] = M{"p0": M{"zh": "更新后的译文"}}; return nil }); err != nil {
		t.Fatal(err)
	}
	_, rows, _, err := idx.snapshot(lib, include)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range rows {
		if row.source["text"] == "更新后的译文" {
			found = true
		}
	}
	if !found || &old.rows[0] != &idx.papers[spaces[1].Root].rows[0] {
		t.Fatal("wrong papers reindexed")
	}
	if err := os.Rename(spaces[0].Root, filepath.Join(lib.Root, ".removed")); err != nil {
		t.Fatal(err)
	}
	_, _, _, err = idx.snapshot(lib, include)
	if err != nil || len(idx.papers) != 1 {
		t.Fatal("deleted paper retained", err)
	}
}
func BenchmarkScopedQuestion(b *testing.B) {
	lib, _ := indexedLibrary(b, 60)
	for _, warm := range []bool{false, true} {
		name := "cold"
		if warm {
			name = "warm"
		}
		b.Run(name, func(b *testing.B) {
			idx := new(PassageIndex)
			include := func(string, M) bool { return true }
			for i := 0; i < b.N+1; i++ {
				if !warm {
					idx = new(PassageIndex)
				}
				if i == 1 {
					b.ResetTimer()
				}
				catalog, rows, _, err := idx.snapshot(lib, include)
				if err != nil {
					b.Fatal(err)
				}
				selectSources(rows, catalog, "study result", false)
			}
		})
	}
}
