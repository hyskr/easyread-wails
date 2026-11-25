# EasyRead Wails

A local paper reader for Apple Silicon Macs. Import a PDF or web article, compare the original with its translation, annotate passages, and organize rich-text notes in a mind map. Your library stays on your Mac. Translation and chat send relevant excerpts to the model you configure.

[Download the macOS release](https://github.com/hyskr/easyread-wails/releases/latest) · [User guide](docs/guide.en.md) · [中文](README.md)

Open the DMG and drag EasyRead Wails into Applications. Choose a model CLI or API in Settings. Future updates are available in Software Updates.

<a id="run-from-source"></a>

## Development

Install Xcode Command Line Tools, Go 1.23 or newer, Node.js 22 and Poppler on an Apple Silicon Mac. From the repository root:

```sh
brew install poppler
npm ci --ignore-scripts
npm start
```

Run `npm run build:editor` after changing editor sources. Run `npm run dist` to create the DMG and ZIP under `dist/wails/`.

The native shell lives in `desktop-wails/`, the Go service in `backend/`, and the shared reader in `easyread/web/`. Build scripts and regression tests live in `scripts/` and `tests/`. The Python and Electron implementations are retained for upstream integration and testing.

[Upstream EasyRead](https://github.com/Edwardxlai/easyread) · [Desktop development](desktop-wails/README.md) · [Upstream sync](docs/upstream-sync.md) · [MIT license](LICENSE)
