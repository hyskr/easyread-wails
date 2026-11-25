# EasyRead Wails

面向 Apple 芯片 Mac 的本地论文阅读器。导入 PDF 或网页文章后，可以对照原文与译文、划线批注、写富文本笔记，并用思维导图整理笔记。文献保存在本机；翻译或问答时，相关内容会发送给你选择的模型。

[下载 macOS Apple Silicon 正式版](https://github.com/hyskr/easyread-wails/releases/latest) · [使用指南](docs/guide.md) · [English](README.en.md)

打开 DMG，将 EasyRead Wails 拖入“应用程序”。在设置中选择已登录的模型 CLI 或配置 API，之后可在“软件更新”页升级。

<a id="run-from-source"></a>

## 本地开发

在 Apple 芯片 Mac 上安装 Xcode Command Line Tools、Go 1.23 或更高版本、Node.js 22 和 Poppler，然后在仓库根目录运行：

```sh
brew install poppler
npm ci --ignore-scripts
npm start
```

修改笔记编辑器源码后运行 `npm run build:editor`。运行 `npm run dist` 可在 `dist/wails/` 生成 DMG 和 ZIP。

| 目录 | 用途 |
| --- | --- |
| `desktop-wails/` | 原生窗口、菜单、深链接、更新与退出保存 |
| `backend/` | Go 服务，负责导入、翻译、问答、文献库和导出 |
| `easyread/web/` | 文献库、阅读器、Tiptap 笔记与 Markmap |
| `scripts/`、`tests/` | 构建、打包和回归测试 |
| `docs/` | 使用指南、数据格式、接口及上游同步记录 |

项目基于 [EasyRead](https://github.com/Edwardxlai/easyread)。仓库也保留上游 Python 和 Electron 实现，供同步、对照和对应平台的测试使用。

[桌面开发与发布](desktop-wails/README.md) · [手动同步上游](docs/upstream-sync.md) · [上游贡献者](docs/guide.md#贡献者) · [MIT 许可](LICENSE)
