package translate

import (
	"context"
	"github.com/hyskr/easyread/backend/engines"
	"strings"
	"testing"
)

func TestFreshSinglePageOutputLimitRecovery(t *testing.T) {
	ws := workspace(t, Object{"meta": Object{"page_count": 1}, "blocks": []any{}, "translation": Object{}})
	old := Run
	defer func() { Run = old }()
	calls := 0
	Run = func(ctx context.Context, cfg Object, prompt, cwd string, images []string) (string, Object, error) {
		calls++
		switch calls {
		case 1:
			return "", nil, engines.ErrOutputLimit
		case 2:
			if !strings.Contains(prompt, "Do not translate") {
				t.Error("expected source-only stage")
			}
			return `{"blocks":[{"id":"p1","type":"para","page":1,"en":"Source"}]}`, nil, nil
		default:
			return `{"zh":{"p1":"译文"}}`, nil, nil
		}
	}
	failed, err := Pages(context.Background(), ws, Object{}, []int{1}, false, func(int, int, string, Object) {})
	if err != nil || len(failed) > 0 || calls != 3 {
		t.Fatal(failed, err, calls)
	}
	paper, _ := ws.Load("paper")
	if Obj(List(paper["blocks"])[0])["zh"] != "译文" {
		t.Fatal(paper)
	}
}
func TestWebReadNeedsNoPDFOrModel(t *testing.T) {
	ws := workspace(t, Object{"meta": Object{"source_type": "web", "page_count": 1}, "blocks": []any{Object{"id": "web-0001", "type": "para", "page": 1, "en": "Original"}}, "translation": Object{}})
	failed, err := Pages(context.Background(), ws, Object{}, []int{1}, true, func(int, int, string, Object) {})
	if err != nil || len(failed) > 0 {
		t.Fatal(failed, err)
	}
}
func TestWebTranslationRequiresEveryRunBeforeSaving(t *testing.T) {
	ws := workspace(t, Object{"meta": Object{"source_type": "web", "page_count": 1}, "blocks": []any{Object{"id": "web-0001", "type": "para", "page": 1, "en": "Hello world", "web_runs": []any{Object{"id": "r1", "en": "Hello "}, Object{"id": "r2", "en": "world"}}}}, "translation": Object{"en_pages": []any{1}}})
	old := Run
	defer func() { Run = old }()
	Run = func(context.Context, Object, string, string, []string) (string, Object, error) {
		return `{"r1":"你好"}`, nil, nil
	}
	failed, err := Pages(context.Background(), ws, Object{}, []int{1}, false, func(int, int, string, Object) {})
	if err != nil || len(failed) != 1 {
		t.Fatal(failed, err)
	}
	paper, _ := ws.Load("paper")
	if Obj(List(paper["blocks"])[0])["zh"] != nil {
		t.Fatal("partial translation saved")
	}
	Run = func(context.Context, Object, string, string, []string) (string, Object, error) {
		return `{"r1":"你好","r2":"世界"}`, nil, nil
	}
	failed, err = Pages(context.Background(), ws, Object{}, []int{1}, false, func(int, int, string, Object) {})
	if err != nil || len(failed) > 0 {
		t.Fatal(failed, err)
	}
	paper, _ = ws.Load("paper")
	if Obj(List(paper["blocks"])[0])["zh"] != "你好世界" || len(List(Obj(paper["translation"])["done_pages"])) != 1 {
		t.Fatal(paper)
	}
}
