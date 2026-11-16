package server

import (
	"github.com/hyskr/easyread/backend/importer"
	"net/http"
)

func (a *App) importWeb(w http.ResponseWriter, r *http.Request, b Object) error {
	snapshot, meta, blocks, err := importer.FetchWeb(r.Context(), text(b["ref"]))
	if err != nil {
		return err
	}
	ws, fresh, err := a.Library.ImportWeb(text(b["ref"]), snapshot, meta, blocks)
	if err != nil {
		return err
	}
	// PDF page limits do not apply to logical article batches.
	delete(b, "pages")
	delete(b, "page_cap")
	b["scope"] = "all"
	return a.importResult(w, ws, fresh, b)
}
