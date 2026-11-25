package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hyskr/easyread/backend/store"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type watchOptions struct {
	Path         string             `json:"path"`
	Link         bool               `json:"link"`
	KeepFilename bool               `json:"keep_filename"`
	Done         map[string][]int64 `json:"done"`
}
type folderWatch struct {
	mu       sync.Mutex
	path     string
	roots    map[string]*watchOptions
	previous map[string]map[string][]int64
}

func (a *App) startFolderWatch() error {
	w := &a.watch
	w.path = filepath.Join(a.Config.Home, "folder-watch", fmt.Sprintf("%x.json", sha256.Sum256([]byte(a.Library.Root))))
	w.roots = map[string]*watchOptions{}
	w.previous = map[string]map[string][]int64{}
	b, err := os.ReadFile(w.path)
	if err == nil {
		if err = json.Unmarshal(b, &w.roots); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	a.prepareWG.Add(1)
	go func() {
		defer a.prepareWG.Done()
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-a.background.Done():
				return
			case <-ticker.C:
				a.pollFolders()
			}
		}
	}()
	return nil
}
func (w *folderWatch) options(path string) any {
	root, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if opt := w.roots[root]; opt != nil {
		return Object{"link": opt.Link, "keep_filename": opt.KeepFilename}
	}
	return nil
}
func (a *App) configureWatch(b Object) error {
	path := text(b["path"])
	if !filepath.IsAbs(path) {
		return fmt.Errorf("请选择绝对目录路径")
	}
	enabled, ok := b["enabled"].(bool)
	if !ok {
		return fmt.Errorf("enabled must be boolean")
	}
	path = filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	} else if enabled {
		return err
	}
	w := &a.watch
	w.mu.Lock()
	defer w.mu.Unlock()
	roots := map[string]*watchOptions{}
	for key, opts := range w.roots {
		roots[key] = opts
	}
	if enabled {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("请选择文件夹")
		}
		overlaps := func(other string) bool { return within(other, path) || within(path, other) }
		if overlaps(a.Library.Root) {
			return fmt.Errorf("自动导入目录不能与文献库重叠")
		}
		for key := range roots {
			if key != path && overlaps(key) {
				return fmt.Errorf("自动导入目录不能互相包含")
			}
		}
		done := map[string][]int64{}
		if old := roots[path]; old != nil {
			done = old.Done
		}
		roots[path] = &watchOptions{path, b["link"] == true, b["keep_filename"] == true, done}
	} else {
		delete(roots, path)
	}
	if err := os.MkdirAll(filepath.Dir(w.path), 0700); err != nil {
		return err
	}
	if err := store.WriteJSON(w.path, roots); err != nil {
		return err
	}
	w.roots = roots
	delete(w.previous, path)
	return nil
}
func (a *App) pollFolders() {
	w := &a.watch
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.roots) == 0 {
		return
	}
	a.mu.Lock()
	if a.status != "idle" {
		a.mu.Unlock()
		return
	}
	a.active++
	a.mu.Unlock()
	defer func() { a.mu.Lock(); a.active--; a.mu.Unlock() }()
	for path, opt := range w.roots {
		if a.background.Err() != nil {
			return
		}
		if err := a.scanFolder(path, opt); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("automatic folder import: %v", err)
		}
	}
}
func sameStamp(a, b []int64) bool { return len(a) >= 2 && len(b) >= 2 && a[0] == b[0] && a[1] == b[1] }
func (a *App) scanFolder(root string, opt *watchOptions) error {
	real, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	if real != root {
		return fmt.Errorf("watched path now resolves elsewhere")
	}
	w := &a.watch
	found := map[string][]int64{}
	changed := false
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if a.background.Err() != nil {
			return a.background.Err()
		}
		if path != root && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || !strings.EqualFold(filepath.Ext(path), ".pdf") {
			return nil
		}
		if len(found) >= 5000 {
			return fmt.Errorf("自动导入目录超过 5000 个 PDF")
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		stamp := []int64{info.Size(), info.ModTime().UnixNano()}
		found[rel] = stamp
		if sameStamp(opt.Done[rel], stamp) || !sameStamp(w.previous[root][rel], stamp) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		after, err := os.Lstat(path)
		if err != nil || !after.Mode().IsRegular() || !os.SameFile(info, after) || !info.ModTime().Equal(after.ModTime()) || !bytes.HasPrefix(data, []byte("%PDF")) || !bytes.Contains(data[max(0, len(data)-65536):], []byte("%%EOF")) {
			return nil
		}
		var ws *store.Workspace
		if opt.Link {
			ws, _, err = a.Library.ImportPath(path, nil, true, opt.KeepFilename)
		} else {
			ws, _, err = a.Library.ImportPDF(data, d.Name(), nil, opt.KeepFilename)
		}
		if err != nil {
			return err
		}
		category := filepath.ToSlash(filepath.Join(filepath.Base(root), filepath.Dir(rel)))
		if _, err = ws.PatchItem(Object{"add_category": category}); err != nil {
			return err
		}
		p, err := ws.Load("paper")
		if err != nil {
			return err
		}
		j, err := ws.Load("job")
		if err != nil {
			return err
		}
		if len(array(object(p["meta"])["pages"])) == 0 && j["state"] != "queued" && j["state"] != "running" {
			if _, err = a.Jobs.Prepare(ws); err != nil {
				return err
			}
		}
		if opt.Done == nil {
			opt.Done = map[string][]int64{}
		}
		opt.Done[rel] = stamp
		changed = true
		return nil
	})
	if err == nil {
		w.previous[root] = found
		for rel := range opt.Done {
			if _, ok := found[rel]; !ok {
				delete(opt.Done, rel)
				changed = true
			}
		}
	}
	if changed {
		if saveErr := store.WriteJSON(w.path, w.roots); saveErr != nil {
			return saveErr
		}
	}
	return err
}
