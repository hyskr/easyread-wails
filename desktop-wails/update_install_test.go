package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInstallationRestoresPreviousBundle(t *testing.T) {
	for _, failure := range []string{"copy", "launch", "none"} {
		t.Run(failure, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "EasyRead Wails.app")
			if err := os.Mkdir(target, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(target, "original"), nil, 0600); err != nil {
				t.Fatal(err)
			}
			commands := map[string]string{"ditto": "mkdir -p \"$2\"; touch \"$2/new\"; test \"$FAIL\" != copy", "open": "test \"$FAIL\" != launch || test -f \"$1/original\""}
			for name, body := range commands {
				if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body), 0700); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command("/bin/sh", "-c", installScript, "install", "2147483647", "fixture", target)
			cmd.Env = append(os.Environ(), "FAIL="+failure, "PATH="+dir+":/bin:/usr/bin")
			out, err := cmd.CombinedOutput()
			if (err == nil) != (failure == "none") {
				t.Fatal(err, string(out))
			}
			contents, err := os.ReadDir(target)
			if err != nil || len(contents) != 1 {
				t.Fatal(contents, err)
			}
			expected := "original"
			if failure == "none" {
				expected = "new"
			}
			if contents[0].Name() != expected {
				t.Fatal(contents)
			}
		})
	}
}
func TestInvalidUpdateCanBeDownloadedAgain(t *testing.T) {
	dir := t.TempDir()
	zip := filepath.Join(dir, "update.zip")
	if err := os.WriteFile(zip, []byte("incomplete"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := unpackUpdate(zip, dir); err == nil {
		t.Fatal("invalid package accepted")
	}
	if _, err := os.Stat(zip); !os.IsNotExist(err) {
		t.Fatal("bad cache still blocks retry")
	}
}
