# 手动同步上游

Go/Wails 主分支：`main`，历史开发分支：`feat/wails-desktop`，仓库：`hyskr/easyread-wails`。
上游：`Edwardxlai/easyread`。本项目不配置定时同步。

## 同步步骤

1. 先保存当前改动，确认 `git status` 干净。
2. `git fetch origin main`，其中 origin 指向 Edwardxlai/easyread；fork 指向 hyskr/easyread-wails。
3. `git log --oneline HEAD..origin/main` 查看上游新增提交。
4. 在 Wails 分支执行 `git merge origin/main`，保留 Wails 原生桥接与打包配置；对桌面协议、退出保存、导出和后端启动的冲突逐项处理。
5. 运行 `node --test tests/web/test_*.cjs tests/desktop/test_*.cjs`，以及本次变化涉及的 Python 测试。
6. 重新构建编辑器、Go 后端和 Wails 应用；实际验证导入 PDF、阅读、笔记保存、导出、退出重开。
7. `git push fork HEAD:main`。推送会触发 Wails Actions 构建；只有明确发布时创建 `wails-v*` 标签，发布稳定版本。不会自动替换本机安装。

可在本聊天中发送“同步 EasyRead 上游并更新 Wails”，手动触发上述流程。

## Go 后端分支

Wails 当前构建入口为 `node scripts/build_backend.cjs`，后端位于 `backend/`；前端通过 `easyread/web/assets.go` 嵌入。同步上游时保留这些文件，并将 Python HTTP 接口或存储格式变化对应移植到 Go；不要仅覆盖 Python 代码后认为 Go 后端已同步。验证使用 `cd backend && go test ./...` 和 `EASYREAD_PDF_BIN=/opt/homebrew/bin node tests/backend/native-smoke.cjs`。

## 2026-11-04 同步状态

已合并上游 `19fdc96`，保留 Go 后端与 Wails 桌面入口。Go 已接入引用关系图、引用 URL/PDF 导入和关联校验，参考文献按文本及印刷键合并并保留编号冲突。模型输入通过 Poppler 提取粗体、斜体和内部链接提示，保留抽取原文用于定位。已补参考文献标题有条目但模型未返回条目的检测，以及正文链接定位清理。

PDF 引擎仍有差异：Poppler 内部链接提供页面目标及可见标签，不能完整复现 PDFium 的命名目标、逐字符字体权重及脚注几何识别。模型通过可见标签补齐语义引用键；复杂脚注与数学字体的识别仍需实文验证。

## 2026-11-11 同步状态

合并上游 `3cd21f5`（v1.4.7 发布记录）。前端及保留的 Python 实现同步了批注范围、数学划线、主题恢复、返回位置、文档附件和翻译检查改动。Go 本轮移植 PDF/TXT/Markdown 附件读取、术语有效项过滤、流式正式完成事件校验。合并后 304 项前端/桌面测试与完整 Go 测试通过。

待继续对齐的 Go 行为包括每次 API 请求的细粒度诊断日志、按问题块局部修复、PDF 字体/脚注的几何语义，以及上游新增的用量计时阶段划分。这些差异并未通过本次代码合并自动消除。


## 2026-11-16 同步状态

合并上游 `a08c0d2`（v1.5.0）。阅读器同步多标签面板、左右／上下分屏、正文分批加载、网页批注、模型设置布局和跟随系统主题修复。笔记保留 Tiptap、全屏工作区、公式渲染、来源引用和 XMind／SVG 导出；侧栏导图改用上游独立标签，可与笔记分屏。

Go 已移植网页 URL 导入、隔离浏览器加载动态文章、静态快照、文字片段翻译、网页离线导出和带段落出处的文献库问答。重复导入按上游 URL 标识复用条目，保留笔记；网页阅读不调用模型。WebKit 的网页事件桥接允许可信父页监听器运行，快照仍使用 `script-src 'none'` CSP 禁止导入脚本执行；引用会过滤隐藏的另一语言文本。翻译补充单页截断后分步整理／翻译、编号代码图遗漏检查、算法标记检查和表格缩进提示；OpenCode 显式设置会话标题，支持 `EASYREAD_TARGET` 默认语言。

Go 与上游仍有差异：网页封面优先使用已归档的正文图片，否则使用标题封面，不执行 Python 旧封面回填工具；搜索当前按请求读取本地段落，没有上游的增量内存索引；模型默认值展示只采用当前 CLI 配置，不记录所有第三方 CLI 的“上次实际型号”。Poppler 对编号代码图的精确裁剪、脚注几何和局部修复差异继续保留。网页资源归档失败仍可能依赖网络；需要登录的网页不支持。模型翻译与问答的保存／失败分支使用模拟响应验证，没有调用付费模型。


本轮验证：309 项前端／桌面测试、完整 Go 后端测试、Wails 原生测试、21 项相关 Python 测试通过；Wails 本机构建与无 Python 的 PDF 导入／笔记保存／离线导出检查通过。Chromium 与 WebKit 已检查笔记全屏退出、导图分屏，以及网页选文引用后的粗体、来源链接和服务端保存。

## 2026-11-20 同步状态

合并上游 `23b44d5`。网页文章在原网页排版内渲染公式，既有划线可按渲染后的文字重新定位。Wails 开发预览也能加载带查询参数的 KaTeX 样式。本轮沿用现有 Go 数据格式与导出接口。已在 Go 服务和 WebKit 中验证切换语言、刷新及离线 HTML 重开后的公式和既有高亮。
