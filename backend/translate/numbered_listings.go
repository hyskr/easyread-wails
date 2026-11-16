package translate

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var listingLine = regexp.MustCompile(`^\s*(\d{1,3})[ \t]+\S`)
var listingCaption = regexp.MustCompile(`(?i)^\s*(?:Figure|Fig\.)\s+(\d+(?:\.\d+)*)[.:]\s+\S`)
var listingCode = regexp.MustCompile(`[{}]|:=|=>|\w+\([^)]*\)\s*;`)

// Code with a Figure caption must survive as a figure, not disappear into prose.
func numberedListings(root string, pages []int) map[string]string {
	out := map[string]string{}
	for _, page := range pages {
		body := []string{}
		last := 0
		for _, line := range strings.Split(sourceText(root, page), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			if caption := listingCaption.FindStringSubmatch(line); caption != nil {
				count := 0
				for _, row := range body {
					if listingCode.MatchString(row) {
						count++
					}
				}
				if last >= 3 && len(body) > 0 && (listingLine.MatchString(body[len(body)-1]) || listingCode.MatchString(body[len(body)-1])) && count >= 2 {
					out[fmt.Sprintf("p.%d Fig. %s", page, caption[1])] = strings.TrimSpace(line)
				}
				body = nil
				last = 0
				continue
			}
			if m := listingLine.FindStringSubmatch(line); m != nil {
				n, _ := strconv.Atoi(m[1])
				if n == 1 {
					body = []string{line}
					last = 1
				} else if len(body) > 0 && n == last+1 {
					body = append(body, line)
					last = n
				} else {
					body = nil
					last = 0
				}
			} else if len(body) > 0 {
				body = append(body, line)
				if len(body) > last*6 {
					body = nil
					last = 0
				}
			}
		}
	}
	return out
}
func missingListings(data Object, expected map[string]string) []string {
	present := map[string]bool{}
	for _, v := range List(data["blocks"]) {
		b := Obj(v)
		if b["type"] != "figure" {
			continue
		}
		num := Str(b["num"])
		if m := listingCaption.FindStringSubmatch(Str(b["caption_en"])); m != nil {
			num = m[1]
		}
		present[fmt.Sprintf("p.%d Fig. %s", Num(b["page"]), num)] = true
	}
	missing := []string{}
	for key := range expected {
		if !present[key] {
			missing = append(missing, key)
		}
	}
	return missing
}
