package server

import (
	"context"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

var fontCache struct {
	sync.Mutex
	names []string
}

func localFonts(ctx context.Context, fresh bool) ([]string, error) {
	fontCache.Lock()
	defer fontCache.Unlock()
	if !fresh && fontCache.names != nil {
		return fontCache.names, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "/usr/bin/osascript", "-l", "JavaScript", "-e", `ObjC.import("AppKit"); ObjC.deepUnwrap($.NSFontManager.sharedFontManager.availableFontFamilies).join("\n")`)
	case "windows":
		cmd = exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", `[Console]::OutputEncoding=[Text.Encoding]::UTF8; Add-Type -AssemblyName System.Drawing; (New-Object System.Drawing.Text.InstalledFontCollection).Families.Name`)
	default:
		cmd = exec.CommandContext(ctx, "fc-list", "--format=%{family}\n")
	}
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	names := []string{}
	seen := map[string]bool{}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		parts := []string{line}
		if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
			parts = strings.Split(line, ",")
		}
		for _, name := range parts {
			name = strings.TrimSpace(name)
			key := strings.ToLower(name)
			if key != "" && !strings.HasPrefix(key, ".") && !strings.HasPrefix(key, "@") && !seen[key] {
				names = append(names, name)
				seen[key] = true
			}
		}
	}
	sort.Slice(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })
	fontCache.names = names
	return names, nil
}
