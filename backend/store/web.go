package store

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"html"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// ImportWeb uses the upstream URL identity so repeated imports preserve reader data.
func (l *Library) ImportWeb(source, snapshot string, meta Object, blocks []any) (*Workspace, bool, error) {
	id, err := WebID(source)
	if err != nil {
		return nil, false, err
	}
	unlock, err := Lock(l.Root)
	if err != nil {
		return nil, false, err
	}
	defer unlock()
	if ws, err := l.Workspace(id); err == nil {
		return ws, false, nil
	}
	root := filepath.Join(l.Root, id)
	if err = os.MkdirAll(root, 0755); err != nil {
		return nil, false, err
	}
	if err = os.WriteFile(filepath.Join(root, "source.html"), []byte(snapshot), 0600); err != nil {
		return nil, false, err
	}
	cover := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="600" height="400"><rect width="600" height="400" fill="#f7f4ed"/><text x="35" y="100" font-size="28" fill="#292a2e">%s</text><text x="35" y="340" font-size="20" fill="#6b6b6b">%s</text></svg>`, html.EscapeString(str(meta["title_en"])), html.EscapeString(str(meta["venue"])))
	if err = os.WriteFile(filepath.Join(root, "cover.svg"), []byte(cover), 0600); err != nil {
		return nil, false, err
	}
	for _, v := range blocks {
		if b := obj(v); b["type"] == "figure" && strings.HasPrefix(str(b["src"]), "data:image/") {
			meta["cover"] = b["src"]
			break
		}
	}
	for _, v := range blocks {
		b := obj(v)
		src := str(b["src"])
		if !strings.HasPrefix(src, "data:image/") {
			continue
		}
		parts := strings.SplitN(src, ",", 2)
		if len(parts) != 2 {
			continue
		}
		ext := map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp", "image/gif": ".gif", "image/svg+xml": ".svg"}[strings.TrimSuffix(strings.TrimPrefix(parts[0], "data:"), ";base64")]
		if ext == "" {
			continue
		}
		data, e := base64.StdEncoding.DecodeString(parts[1])
		if e != nil {
			return nil, false, e
		}
		if e = os.MkdirAll(filepath.Join(root, "figures"), 0755); e != nil {
			return nil, false, e
		}
		rel := "figures/" + str(b["id"]) + ext
		if e = os.WriteFile(filepath.Join(root, rel), data, 0600); e != nil {
			return nil, false, e
		}
		b["src"] = rel
	}
	pages := []any{}
	for _, v := range array(meta["pages"]) {
		pages = append(pages, obj(v)["n"])
	}
	files := map[string]Object{"paper": {"schema": Schema, "meta": meta, "blocks": blocks, "references": []any{}, "glossary": []any{}, "translation": Object{"done_pages": pages, "en_pages": pages, "gaps_checked": true}}, "reader": EmptyReader(), "discussion": EmptyDiscussion(), "item": {"added": Now(), "tags": []any{}, "status": "unread", "starred": false}}
	for _, name := range []string{"reader", "discussion", "item", "paper"} {
		if err = WriteJSON(filepath.Join(root, name+".json"), files[name]); err != nil {
			return nil, false, err
		}
	}
	return NewWorkspace(root), true, nil
}

func WebID(source string) (string, error) {
	u, err := url.Parse(source)
	if err != nil {
		return "", err
	}
	u.Fragment = ""
	return fmt.Sprintf("%x", sha256.Sum256([]byte("web:"+u.String())))[:12], nil
}
