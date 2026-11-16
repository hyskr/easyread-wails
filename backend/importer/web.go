package importer

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"golang.org/x/net/html"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"time"
)

const snapshotCSP = "default-src 'none'; style-src 'unsafe-inline' https: http:; img-src data: https: http:; font-src data: https: http:; media-src data: https: http:; base-uri 'none'; form-action 'none'; frame-src 'none'; script-src 'none'"

func attr(n *html.Node, k string) string {
	for _, a := range n.Attr {
		if a.Key == k {
			return a.Val
		}
	}
	return ""
}
func setAttr(n *html.Node, k, v string) {
	for i := range n.Attr {
		if n.Attr[i].Key == k {
			n.Attr[i].Val = v
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: k, Val: v})
}
func textNode(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(textNode(c))
	}
	return b.String()
}
func walk(n *html.Node, f func(*html.Node)) {
	f(n)
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		walk(c, f)
		c = next
	}
}
func resolve(base, ref string) string {
	u, e := url.Parse(ref)
	if e != nil {
		return ""
	}
	b, e := url.Parse(base)
	if e != nil {
		return ""
	}
	return b.ResolveReference(u).String()
}

// FetchWeb freezes scripts and executable navigation before exposing a same-origin snapshot.
func FetchWeb(ctx context.Context, address string) (string, M, []any, error) {
	data, final, err := get(ctx, address, 8<<20)
	if err != nil {
		return "", nil, nil, err
	}
	if !strings.Contains(strings.ToLower(string(data[:min(512, len(data))])), "<") {
		return "", nil, nil, fmt.Errorf("这个链接不是 HTML 网页")
	}
	snapshot, meta, blocks, err := freezeWeb(ctx, string(data), final)
	if err != nil && strings.Contains(strings.ToLower(string(data)), "<script") {
		rendered, renderErr := renderWeb(ctx, final)
		if renderErr != nil {
			return "", nil, nil, renderErr
		}
		return freezeWeb(ctx, rendered, final)
	}
	return snapshot, meta, blocks, err
}
func freezeWeb(ctx context.Context, source, base string) (string, M, []any, error) {
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return "", nil, nil, err
	}
	var head, body, scope *html.Node
	title := ""
	author := ""
	date := ""
	walk(doc, func(n *html.Node) {
		switch n.Data {
		case "head":
			head = n
		case "body":
			body = n
		case "article":
			if scope == nil || scope.Data != "article" {
				scope = n
			}
		case "main":
			if scope == nil {
				scope = n
			}
		case "title":
			title = textNode(n)
		case "base":
			if attr(n, "href") != "" {
				base = resolve(base, attr(n, "href"))
			}
		case "meta":
			if attr(n, "name") == "author" {
				author = attr(n, "content")
			}
			if attr(n, "property") == "article:published_time" {
				date = attr(n, "content")
			}
		}
	})
	if body == nil {
		return "", nil, nil, fmt.Errorf("网页没有正文")
	}
	if scope == nil {
		scope = body
	}
	remaining := 32 << 20
	requests := 0
	cache := map[string]string{}
	missing := []any{}
	asset := func(address string) string {
		if strings.HasPrefix(address, "data:") {
			return address
		}
		if v, ok := cache[address]; ok {
			return v
		}
		cache[address] = address
		if requests >= 80 || remaining <= 0 {
			missing = append(missing, address)
			return address
		}
		requests++
		b, _, e := get(ctx, address, int64(min(8<<20, remaining)))
		if e != nil {
			missing = append(missing, address)
			return address
		}
		remaining -= len(b)
		typ := mime.TypeByExtension(path.Ext(strings.Split(address, "?")[0]))
		if typ == "" {
			typ = http.DetectContentType(b)
		}
		cache[address] = "data:" + typ + ";base64," + base64.StdEncoding.EncodeToString(b)
		return cache[address]
	}
	cssURLs := regexp.MustCompile(`url\(\s*["']?([^\s)'";]+)["']?\s*\)`)
	css := func(value, base string) string {
		return cssURLs.ReplaceAllStringFunc(value, func(s string) string {
			m := cssURLs.FindStringSubmatch(s)
			if strings.HasPrefix(m[1], "#") {
				return s
			}
			return `url("` + asset(resolve(base, m[1])) + `")`
		})
	}
	walk(doc, func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		switch n.Data {
		case "script", "noscript", "base", "iframe", "object", "embed", "form", "template":
			if n.Parent != nil {
				n.Parent.RemoveChild(n)
			}
			return
		case "meta":
			if attr(n, "http-equiv") != "" {
				n.Parent.RemoveChild(n)
				return
			}
		case "link":
			if strings.Contains(attr(n, "rel"), "stylesheet") {
				address := resolve(base, attr(n, "href"))
				b, final, e := get(ctx, address, 8<<20)
				if e == nil {
					n.Data = "style"
					n.Attr = nil
					n.AppendChild(&html.Node{Type: html.TextNode, Data: css(string(b), final)})
				} else {
					setAttr(n, "href", address)
				}
			} else {
				n.Parent.RemoveChild(n)
				return
			}
		}
		attrs := []html.Attribute{}
		for _, a := range n.Attr {
			k := strings.ToLower(a.Key)
			if strings.HasPrefix(k, "on") || strings.HasPrefix(k, "data-easyread") || k == "srcdoc" || k == "nonce" || k == "integrity" || k == "autofocus" || k == "contenteditable" || k == "srcset" || k == "loading" {
				continue
			}
			if k == "href" || k == "src" || k == "poster" || k == "href" && a.Namespace == "xlink" {
				raw := strings.Map(func(r rune) rune {
					if r <= 32 {
						return -1
					}
					return r
				}, a.Val)
				if !strings.HasPrefix(raw, "#") {
					v := resolve(base, raw)
					u, _ := url.Parse(v)
					if u == nil || !(u.Scheme == "https" || u.Scheme == "http" || u.Scheme == "mailto" || u.Scheme == "tel" || (k == "src" || k == "poster") && strings.HasPrefix(strings.ToLower(raw), "data:image/")) {
						continue
					}
					a.Val = v
				}
			}
			if k == "style" {
				a.Val = css(a.Val, base)
			}
			attrs = append(attrs, a)
		}
		n.Attr = attrs
		if n.Data == "img" || n.Data == "source" {
			src := first(attr(n, "src"), attr(n, "data-src"))
			if src != "" {
				setAttr(n, "src", asset(resolve(base, src)))
			}
		}
		if n.Data == "style" && n.FirstChild != nil {
			n.FirstChild.Data = css(n.FirstChild.Data, base)
		}
		if n.Data == "a" {
			setAttr(n, "target", "_blank")
			setAttr(n, "rel", "noopener noreferrer")
		}
	})
	blocks := []any{}
	groups := map[*html.Node]M{}
	var visit func(*html.Node, bool)
	visit = func(n *html.Node, skip bool) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "code", "svg", "math", "nav", "footer", "button", "textarea", "select":
				skip = true
			}
			if attr(n, "hidden") != "" || attr(n, "aria-hidden") == "true" || attr(n, "translate") == "no" || strings.Contains(attr(n, "class"), "katex") || strings.Contains(attr(n, "style"), "display:none") {
				skip = true
			}
			if !skip && n.Data == "img" && attr(n, "src") != "" {
				blocks = append(blocks, M{"id": fmt.Sprintf("web-%04d", len(blocks)+1), "type": "figure", "src": attr(n, "src"), "caption_en": attr(n, "alt"), "page": 1})
			}
			if !skip && n.Data == "pre" {
				blocks = append(blocks, M{"id": fmt.Sprintf("web-%04d", len(blocks)+1), "type": "para", "en": "```\n" + textNode(n) + "\n```", "page": 1})
				return
			}
		}
		if !skip && n.Type == html.TextNode && strings.TrimSpace(n.Data) != "" {
			parent := n.Parent
			for p := n.Parent; p != nil && p != scope.Parent; p = p.Parent {
				if strings.Contains("|p|h1|h2|h3|h4|h5|h6|li|figcaption|td|th|blockquote|", "|"+p.Data+"|") {
					parent = p
					break
				}
			}
			b := groups[parent]
			if b == nil {
				b = M{"id": fmt.Sprintf("web-%04d", len(blocks)+1), "type": "para", "en": "", "web_runs": []any{}, "page": 1}
				if len(parent.Data) == 2 && parent.Data[0] == 'h' && parent.Data[1] >= '1' && parent.Data[1] <= '6' {
					b["type"] = "heading"
					b["level"] = min(2, int(parent.Data[1]-'0'))
				}
				blocks = append(blocks, b)
				groups[parent] = b
			}
			runs := arr(b["web_runs"])
			rid := fmt.Sprintf("%s-r%d", b["id"], len(runs)+1)
			b["web_runs"] = append(runs, M{"id": rid, "en": n.Data})
			b["en"] = str(b["en"]) + n.Data
			span := &html.Node{Type: html.ElementNode, Data: "span", Attr: []html.Attribute{{Key: "data-easyread-run", Val: rid}, {Key: "style", Val: "display:contents"}}}
			n.Parent.InsertBefore(span, n)
			n.Parent.RemoveChild(n)
			span.AppendChild(n)
			return
		}
		for c := n.FirstChild; c != nil; {
			next := c.NextSibling
			visit(c, skip)
			c = next
		}
	}
	visit(scope, false)
	page, size, total := 1, 0, 0
	for _, v := range blocks {
		b := obj(v)
		n := len([]rune(str(b["en"])))
		if size > 0 && size+n > 6000 {
			page++
			size = 0
		}
		b["page"] = page
		size += n
		total += n
	}
	if total < 160 {
		return "", nil, nil, fmt.Errorf("没有找到足够的网页正文；请使用静态文章链接")
	}
	pages := []any{}
	for i := 1; i <= page; i++ {
		pages = append(pages, M{"n": i})
	}
	u, _ := url.Parse(base)
	meta := M{"title_en": first(title, u.Hostname()), "url": base, "source": base, "source_type": "web", "authors": author, "date": date, "venue": u.Hostname(), "pages": pages, "page_count": page, "snapshot_external_assets": missing}
	if head != nil {
		head.InsertBefore(&html.Node{Type: html.ElementNode, Data: "meta", Attr: []html.Attribute{{Key: "http-equiv", Val: "Content-Security-Policy"}, {Key: "content", Val: snapshotCSP}}}, head.FirstChild)
		head.InsertBefore(&html.Node{Type: html.ElementNode, Data: "meta", Attr: []html.Attribute{{Key: "charset", Val: "utf-8"}}}, head.FirstChild)
	}
	var out bytes.Buffer
	err = html.Render(&out, doc)
	return out.String(), meta, blocks, err
}

// Dynamic articles use an isolated browser profile, never the user's cookies.
func renderWeb(ctx context.Context, address string) (string, error) {
	candidates := []string{"google-chrome", "chromium", "chromium-browser", "msedge", "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge", "/Applications/Chromium.app/Contents/MacOS/Chromium"}
	browser := ""
	for _, candidate := range candidates {
		if p, e := exec.LookPath(candidate); e == nil {
			browser = p
			break
		}
	}
	if browser == "" {
		return "", fmt.Errorf("这个网页需要 JavaScript 加载正文。请安装 Chrome 或 Edge 后重新导入。")
	}
	profile, e := os.MkdirTemp("", "easyread-web-")
	if e != nil {
		return "", e
	}
	defer os.RemoveAll(profile)
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	data, e := exec.CommandContext(ctx, browser, "--headless", "--disable-gpu", "--no-first-run", "--no-default-browser-check", "--user-data-dir="+profile, "--dump-dom", "--timeout=25000", "--virtual-time-budget=12000", address).Output()
	if e != nil {
		return "", fmt.Errorf("网页加载失败：%w", e)
	}
	if len(data) > 8<<20 {
		return "", fmt.Errorf("网页过大")
	}
	return string(data), nil
}

func WebFallbackAllowed(ref string) bool {
	u, err := url.Parse(ref)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || strings.HasSuffix(strings.ToLower(u.Path), ".pdf") {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "arxiv.org", "www.arxiv.org", "doi.org", "openreview.net", "aclanthology.org":
		return false
	}
	return true
}
