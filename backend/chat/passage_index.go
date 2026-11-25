package chat

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/hyskr/easyread/backend/store"
)

// PassageIndex belongs to one running library. Published rows are immutable.
type PassageIndex struct {
	mu     sync.Mutex
	papers map[string]indexedPaper
}
type indexedPaper struct {
	paper, reader string
	card          M
	rows          []passage
}

func versionOf(path string) string {
	st, err := os.Stat(path)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d-%d", st.ModTime().UnixNano(), st.Size())
}
func (index *PassageIndex) snapshot(lib *store.Library, include func(string, M) bool) (map[string]M, []passage, int, error) {
	index.mu.Lock()
	defer index.mu.Unlock()
	spaces, err := lib.All()
	if err != nil {
		return nil, nil, 0, err
	}
	if index.papers == nil {
		index.papers = map[string]indexedPaper{}
	}
	live := map[string]bool{}
	catalog := map[string]M{}
	rows := []passage{}
	empty := 0
	for _, ws := range spaces {
		live[ws.Root] = true
		item, err := ws.Load("item")
		if err != nil {
			continue
		}
		if !include(ws.ID(), item) {
			continue
		}
		pv, rv := versionOf(filepath.Join(ws.Root, "paper.json")), versionOf(filepath.Join(ws.Root, "reader.json"))
		entry, ok := index.papers[ws.Root]
		if !ok || pv != entry.paper || rv != entry.reader {
			p, err := ws.Load("paper")
			if err != nil {
				delete(index.papers, ws.Root)
				continue
			}
			r, err := ws.Load("reader")
			if err != nil {
				delete(index.papers, ws.Root)
				continue
			}
			meta := obj(p["meta"])
			abstract := str(meta["abstract_en"])
			for _, v := range arr(p["blocks"]) {
				b := obj(v)
				if b["role"] == "abstract" {
					abstract = firstString(str(b["zh"]), str(b["en"]), abstract)
					break
				}
			}
			entry = indexedPaper{pv, rv, M{"title_en": meta["title_en"], "title_zh": meta["title_zh"], "display_title": meta["display_title"], "abstract": abstract}, paperPassages(ws.ID(), p, r)}
			index.papers[ws.Root] = entry
		}
		card := M{}
		for k, v := range entry.card {
			card[k] = v
		}
		for k, v := range obj(item["meta_override"]) {
			if str(v) != "" {
				card[k] = v
			}
		}
		catalog[ws.ID()] = card
		rows = append(rows, entry.rows...)
		if len(entry.rows) == 0 {
			empty++
		}
	}
	for root := range index.papers {
		if !live[root] {
			delete(index.papers, root)
		}
	}
	return catalog, rows, empty, nil
}
