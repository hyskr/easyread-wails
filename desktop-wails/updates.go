package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

var version = "1.5.2"

type downloadProgress struct {
	app             *App
	total, received int64
	last            time.Time
}

func (p *downloadProgress) Write(b []byte) (int, error) {
	p.received += int64(len(b))
	if time.Since(p.last) > 200*time.Millisecond || p.received == p.total {
		p.last = time.Now()
		p.app.updateProgress("正在下载更新", p.received, p.total)
	}
	return len(b), nil
}
func (a *App) UpdateStatus() map[string]any {
	a.updateMu.Lock()
	defer a.updateMu.Unlock()
	result := map[string]any{"current": version, "busy": a.updating.Load(), "supported": false, "message": "尚未检查更新", "cached": a.cachedUpdate()}
	for k, v := range a.updateState {
		result[k] = v
	}
	return result
}
func (a *App) updateProgress(message string, received, total int64) {
	a.updateMu.Lock()
	if a.updateState == nil {
		a.updateState = map[string]any{}
	}
	a.updateState["message"] = message
	a.updateState["received"] = received
	a.updateState["total"] = total
	a.updateMu.Unlock()

}

func updateCache() string {
	home, _ := os.UserCacheDir()
	return filepath.Join(home, "EasyRead Wails", "updates")
}
func (a *App) cachedUpdate() string {
	entries, _ := os.ReadDir(updateCache())
	best := ""
	for _, entry := range entries {
		v := entry.Name()
		if !entry.IsDir() || !semver.IsValid("v"+v) || semver.Compare("v"+v, "v"+version) <= 0 {
			continue
		}
		if _, err := os.Stat(filepath.Join(updateCache(), v, "ready")); err == nil && semver.Compare("v"+v, "v"+best) > 0 {
			best = v
		}
	}
	return best
}
func (a *App) DownloadUpdate() {
	if !a.updating.CompareAndSwap(false, true) {
		return
	}
	a.updateProgress("正在后台下载，完成后自动重启", 0, 0)
	go a.runUpdate(true, true)
}
func (a *App) InstallUpdate() error {
	v := a.cachedUpdate()
	if v == "" {
		return fmt.Errorf("没有已下载的更新")
	}
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	a.installMu.Lock()
	a.installDir = filepath.Join(updateCache(), v)
	a.installSource = filepath.Join(a.installDir, "EasyRead Wails.app")
	a.installTarget = filepath.Clean(filepath.Join(filepath.Dir(binary), "../.."))
	a.installMu.Unlock()
	a.updateProgress("正在保存笔记，准备安装", 0, 0)
	a.requestQuit()
	return nil
}
func (a *App) CheckUpdate(manual bool) { a.checkUpdate(manual, false) }
func (a *App) checkUpdate(manual, download bool) {
	if !a.updating.CompareAndSwap(false, true) {
		return
	}
	a.runUpdate(manual, download)
}
func (a *App) runUpdate(manual, download bool) {
	defer a.updating.Store(false)
	if manual {
		a.updateProgress("正在检查更新", 0, 0)
	}
	fail := func(e error) {
		a.updateProgress("更新失败："+e.Error(), 0, 0)

	}
	client := &http.Client{Timeout: 3 * time.Minute}
	response, err := client.Get("https://api.github.com/repos/hyskr/easyread-wails/releases?per_page=30")
	if err != nil {
		if manual {
			fail(err)
		}
		return
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		if manual {
			fail(fmt.Errorf("GitHub HTTP %d", response.StatusCode))
		}
		return
	}
	var releases []struct {
		Tag    string `json:"tag_name"`
		Draft  bool   `json:"draft"`
		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err = json.NewDecoder(response.Body).Decode(&releases); err != nil {
		if manual {
			fail(err)
		}
		return
	}
	asset, newVersion := "", ""
	for _, release := range releases {
		if release.Draft || !strings.HasPrefix(release.Tag, "wails-v") {
			continue
		}
		v := strings.TrimPrefix(release.Tag, "wails-")
		if !semver.IsValid(v) || semver.Compare(v, "v"+version) <= 0 {
			continue
		}
		for _, file := range release.Assets {
			if file.Name == "EasyRead-Wails-arm64.zip" && semver.Compare(v, "v"+newVersion) > 0 {
				asset = file.URL
				newVersion = strings.TrimPrefix(v, "v")
			}
		}
	}
	if asset == "" {
		if manual {
			a.updateProgress("当前已是最新版本", 0, 0)
		}

		return
	}
	a.updateMu.Lock()
	a.updateState = map[string]any{"message": "发现新版本 " + newVersion, "latest": newVersion}
	a.updateMu.Unlock()
	if !download {
		return
	}
	if a.cachedUpdate() == newVersion {
		a.installDownloadedUpdate()
		return
	}
	dir := filepath.Join(updateCache(), newVersion)
	if err := os.MkdirAll(dir, 0700); err != nil {
		fail(err)
		return
	}
	zipPath := filepath.Join(dir, "update.zip")
	if _, statErr := os.Stat(zipPath); os.IsNotExist(statErr) {
		a.updateProgress("正在连接下载服务器", 0, 0)
		client.Timeout = 30 * time.Minute
		r, err := client.Get(asset)
		if err != nil {
			fail(err)
			return
		}
		defer r.Body.Close()
		if r.StatusCode != 200 {
			fail(fmt.Errorf("下载 HTTP %d", r.StatusCode))
			return
		}
		file, err := os.Create(zipPath + ".part")
		if err != nil {
			fail(err)
			return
		}
		progress := &downloadProgress{app: a, total: r.ContentLength}
		a.updateProgress("正在下载更新", 0, r.ContentLength)
		_, err = io.Copy(file, io.TeeReader(r.Body, progress))
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			fail(err)
			return
		}
		if err = os.Rename(zipPath+".part", zipPath); err != nil {
			fail(err)
			return
		}

	}
	a.updateProgress("正在解压更新", 0, 0)
	if err = unpackUpdate(zipPath, dir); err != nil {
		fail(err)
		return
	}
	if err = os.WriteFile(filepath.Join(dir, "ready"), []byte(newVersion), 0600); err != nil {
		fail(err)
		return
	}
	a.installDownloadedUpdate()

}

// Extract into a fresh directory so a partial extraction is never marked ready.
func unpackUpdate(zipPath, dir string) error {
	stage, err := os.MkdirTemp(dir, ".unpack-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	output, err := exec.Command("ditto", "-x", "-k", zipPath, stage).CombinedOutput()
	app := "EasyRead Wails.app"
	if err == nil {
		var info os.FileInfo
		info, err = os.Stat(filepath.Join(stage, app, "Contents", "MacOS", "EasyRead Wails"))
		if err == nil && (!info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0) {
			err = fmt.Errorf("更新包缺少可执行文件")
		}
	}
	if err != nil {
		// Keep a usable archive when extraction failed for a local reason (such as a full disk).
		if readableUpdateZIP(zipPath) {
			return fmt.Errorf("解压失败，已保留下载包：%w %s", err, output)
		}
		if removeErr := os.Remove(zipPath); removeErr != nil {
			return fmt.Errorf("清理损坏的更新包：%w", removeErr)
		}
		return fmt.Errorf("解压失败，请重新下载：%w %s", err, output)
	}
	target := filepath.Join(dir, app)
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	return os.Rename(filepath.Join(stage, app), target)
}

func readableUpdateZIP(path string) bool {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return false
	}
	defer archive.Close()
	appBinary := false
	for _, file := range archive.File {
		if file.Name == "EasyRead Wails.app/Contents/MacOS/EasyRead Wails" && file.Mode().IsRegular() && file.Mode().Perm()&0111 != 0 {
			appBinary = true
		}
		r, err := file.Open()
		if err != nil {
			return false
		}
		_, err = io.Copy(io.Discard, r)
		r.Close()
		if err != nil {
			return false
		}
	}
	return appBinary
}

// Both renames stay on the installed application's volume. Keep the old app
// until the new bundle is in place and Launch Services accepts it.
const installScript = `while kill -0 "$1" 2>/dev/null; do sleep 1; done
stage=$(mktemp -d "$(dirname "$3")/.easyread-update.XXXXXX") || { open "$3"; exit 1; }
if ! ditto "$2" "$stage/new.app"; then rm -rf "$stage"; open "$3"; exit 1; fi
if ! mv "$3" "$stage/previous.app"; then rm -rf "$stage"; open "$3"; exit 1; fi
if mv "$stage/new.app" "$3"; then
    if open "$3"; then rm -rf "$stage"; exit 0; fi
    mv "$3" "$stage/rejected.app" || exit 1
fi
mv "$stage/previous.app" "$3" || exit 1
rm -rf "$stage"
open "$3"
exit 1`

func (a *App) installDownloadedUpdate() {
	if err := a.InstallUpdate(); err != nil {
		a.updateProgress("安装更新失败："+err.Error(), 0, 0)
	}
}
func (a *App) launchInstaller() error {
	a.installMu.Lock()
	defer a.installMu.Unlock()
	if a.installDir == "" {
		return nil
	}
	return exec.Command("/bin/sh", "-c", installScript, "easyread-update", fmt.Sprint(os.Getpid()), a.installSource, a.installTarget).Start()
}
