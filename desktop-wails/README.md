# Wails 桌面端

EasyRead Wails 使用系统 WebKit 和随应用打包的 Go 后端。默认文献数据目录为 `~/EasyRead`。

- `main.go` 启动服务并连接阅读器与原生窗口。
- `desktop.go` 处理菜单、窗口状态、文件导出与深链接。
- `exit.go` 协调笔记保存和服务退出。
- `updates.go` 从本仓库的 Wails Release 下载更新，保存笔记后重启安装。

## 源码预览与打包

按[根目录说明](../README.md#run-from-source)安装依赖后运行 `npm start`。开发启动脚本通过 `EASYREAD_WAILS_DEV_ROOT` 指向当前仓库，刷新阅读页即可查看普通 JS、CSS 和 HTML 修改。修改编辑器源码后运行 `npm run build:editor`，修改 Go 代码后运行 `./scripts/build_wails.sh` 并重新启动。

本地构建应用位于 `desktop-wails/build/bin/EasyRead Wails.app`。运行 `npm run dist` 生成 `dist/wails/EasyRead-Wails-arm64.dmg` 与同名 ZIP。

## 发布与更新

推送 `wails-v*` 版本标签后，[Wails 工作流](../.github/workflows/wails-release.yml)测试、构建并发布 macOS arm64 安装包。签名使用仓库 Secrets `MAC_SIGN_P12` 与 `MAC_SIGN_P12_PASSWORD`。当前发布包使用自签名，尚无 Apple 公证。

软件更新页支持后台下载，缓存位于 `~/Library/Caches/EasyRead Wails/updates/`。Wails 导出的论文来源链接使用 `easyread-wails://open`。

[手动同步上游](../docs/upstream-sync.md)
