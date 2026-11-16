package main

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDownloadedUpdateSurvivesNewAppInstance(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	dir := filepath.Join(updateCache(), "99.0.0")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ready"), []byte("99.0.0"), 0600); err != nil {
		t.Fatal(err)
	}
	first := &App{}
	if first.cachedUpdate() != "99.0.0" {
		t.Fatal("cache not detected")
	}
	second := &App{}
	if second.UpdateStatus()["cached"] != "99.0.0" {
		t.Fatal("cache lost after restart")
	}
}

// A settings page may disappear while the download request is still running.
func TestDownloadRunsOutsideSettingsBinding(t *testing.T) {
	previous := http.DefaultTransport
	entered, release := make(chan struct{}), make(chan struct{})
	http.DefaultTransport = updateRoundTrip(func(*http.Request) (*http.Response, error) {
		close(entered)
		<-release
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("[]")), Header: make(http.Header)}, nil
	})
	defer func() { http.DefaultTransport = previous }()
	a := &App{}
	returned := make(chan struct{})
	go func() { a.DownloadUpdate(); close(returned) }()
	select {
	case <-returned:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("download blocked the page binding")
	}
	<-entered
	if !a.updating.Load() {
		t.Fatal("download was not marked busy")
	}
	a.DownloadUpdate() // A second click must not start another request.
	close(release)
	deadline := time.Now().Add(time.Second)
	for a.updating.Load() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if a.updating.Load() {
		t.Fatal("download did not finish")
	}
}

type updateRoundTrip func(*http.Request) (*http.Response, error)

func (f updateRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDownloadedUpdatePreparesAutomaticInstall(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	dir := filepath.Join(updateCache(), "99.0.0")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ready"), []byte("99.0.0"), 0600); err != nil {
		t.Fatal(err)
	}
	// An existing save request prevents this unit test from invoking the native window.
	a := &App{quitPhase: "waiting"}
	a.installDownloadedUpdate()
	if a.installDir != dir || a.installSource != filepath.Join(dir, "EasyRead Wails.app") {
		t.Fatal("download completion did not prepare installation")
	}
	if _, err := os.Stat(filepath.Join(dir, "ready")); err != nil {
		t.Fatal("download cache removed", err)
	}
}
