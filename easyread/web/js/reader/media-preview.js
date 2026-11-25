/* One on-demand renderer for figure/table hovers and note references. */
(function (PR) {
  "use strict";
  const isMedia = (b) => b && (b.type === "figure" || b.type === "table");
  const blocks = () => (PR.state.paper.blocks || []).filter(isMedia);
  const caption = (b) => PR.plain(PR.textFor(b.id + "#caption") || b.caption_en || b.caption_zh || "");
  const label = (b) => (b.type === "table" ? PR.t("表 {n}", { n: b.num || "" }) : PR.t("图 {n}", { n: b.num || "" })).trim();
  // Store identity only, never pixels or a duplicate table in reader.json.
  const reference = (b) => ({ id: b.id, type: b.type, num: String(b.num || ""), page: b.page, caption: caption(b).slice(0, 160) });
  function resolve(ref) {
    if (!ref) return null;
    const same = (b) => isMedia(b) && b.type === ref.type && String(b.num || "") === String(ref.num || "");
    const exact = PR.blockById[ref.id];
    if (same(exact)) return exact;
    // A retranslation can change IDs. Only accept an unambiguous match on the same page.
    const matches = ref.num ? blocks().filter((b) => same(b) && b.page === ref.page) : [];
    return matches.length === 1 ? matches[0] : null;
  }
  function tableHtml(b, compact = false) {
    const {head, rows, notes} = PR.tableParts(b);
    const cells = (rs, tag) => rs.map((r) => "<tr>" + r.map((c) => "<" + tag + ">" + PR.md(String(c ?? ""), { cite: false, xref: false }) + "</" + tag + ">").join("") + "</tr>").join("");
    return '<div class="media-table-scroll"' + (compact ? '' : ' tabindex="0"') + '><table><thead>' + cells(head, "th") + '</thead><tbody>' + cells(compact ? rows.slice(0, 4) : rows, "td") + "</tbody></table></div>" + (compact ? "" : PR.tableNotesHtml(notes, b));
  }
  function thumbnail(b) {
    if (!b) return '<span class="media-empty">' + PR.t("图表已变化或移除") + '</span>';
    if (b.type === "table") return tableHtml(b, true);
    if (!b.src) return '<span class="media-empty">' + PR.t("尚无图像，可查看 PDF 原页") + '</span>';
    return '<span class="media-image"><img src="' + PR.esc(PR.imageUrl(b.src)) + '" alt="' + PR.esc(label(b)) + '" loading="lazy" decoding="async"><span class="media-image-error" hidden>' + PR.t("图片未能加载，请查看原页") + '</span></span>';
  }
  function html(ref) {
    const b = resolve(ref);
    if (!b) return '<p class="media-empty">' + PR.t("这张图表已变化或移除，请重新关联") + '</p>' + (ref.page ? '<button class="btn sm" data-media-page="' + Number(ref.page) + '">' + PR.t("查看原页") + '</button>' : "");
    let content;
    if (b.type === "table") content = tableHtml(b);
    else if (b.src) content = '<div class="media-image"><img src="' + PR.esc(PR.imageUrl(b.src)) + '" alt="' + PR.esc(label(b)) + '" decoding="async"><span class="media-image-error" hidden>' + PR.t("图片未能加载，请查看原页") + '</span></div>';
    else content = '<p class="media-empty">' + PR.t("尚无图像，可查看 PDF 原页") + '</p>';
    return '<div class="media-preview" data-media-block="' + PR.esc(b.id) + '">' + content + '<div class="media-caption">' + PR.esc(caption(b)) + '</div><div class="media-footer"><span>' + PR.t("第 {page} 页", { page: b.page || "?" }) + '</span><button class="btn sm" data-media-go="' + PR.esc(b.id) + '">' + PR.t("定位正文") + '</button>' + (b.page ? '<button class="btn sm" data-media-page="' + Number(b.page) + '" data-media-block="' + PR.esc(b.id) + '">PDF</button>' : "") + transferActions(b).map((a,i) => '<button class="btn sm" data-media-transfer="' + i + '">' + PR.esc(a.label) + '</button>').join("") + '</div></div>';
  }
  function hover(anchor, b) {
    PR.popover(anchor, '<div class="media-hover"><div class="media-title">' + PR.esc(label(b)) + '</div>' + html(reference(b)) + '<button class="btn sm" data-dock-pin="' + PR.esc(b.id) + '">' + PR.t("在侧栏打开") + '</button></div>', { wide: true });
  }
  document.addEventListener("click", (e) => {
    const command = e.target.closest("[data-media-transfer]");
    if (command) {
      e.preventDefault(); e.stopPropagation();
      const block = PR.blockById[command.closest(".media-preview").dataset.mediaBlock];
      transferActions(block)[Number(command.dataset.mediaTransfer)]?.fn(); return;
    }
    const pin = e.target.closest("[data-dock-pin]");
    if (pin) { e.preventDefault(); PR.pinDockFigure(pin.dataset.dockPin); PR.hidePopover(); return; }
    const go = e.target.closest("[data-media-go]"), page = e.target.closest("[data-media-page]");
    if (!go && !page) return;
    e.preventDefault(); e.stopPropagation(); PR.hidePopover();
    if (go) PR.jumpTo("b-" + go.dataset.mediaGo);
    else PR.openPage(Number(page.dataset.mediaPage), page.dataset.mediaBlock);
  }, true);
  document.addEventListener("error", (e) => {
    if (!e.target.matches || !e.target.matches(".media-image img")) return;
    e.target.hidden = true;
    e.target.nextElementSibling.hidden = false;
  }, true);
  async function originalImage(b) {
    const res = await fetch(PR.imageUrl(b.src));
    if (!res.ok) throw Error(PR.t("图片未能加载，请查看原页"));
    return res.blob();
  }
  async function pngImage(b) {
    const blob = await originalImage(b);
    if (blob.type === "image/png") return blob;
    const url = URL.createObjectURL(blob);
    try {
      const image = new Image(); image.src = url; await image.decode();
      const canvas = document.createElement("canvas");
      canvas.width = image.naturalWidth; canvas.height = image.naturalHeight;
      canvas.getContext("2d").drawImage(image,0,0);
      return await new Promise((resolve,reject) => canvas.toBlob(value => value ? resolve(value) : reject(Error(PR.t("无法复制图片"))), "image/png"));
    } finally { URL.revokeObjectURL(url); }
  }
  function transferActions(b) {
    if (!isMedia(b)) return [];
    const action = (label, icon, run) => ({label, icon, fn:async () => {
      try { await run(); } catch (err) { PR.toast(PR.esc(err.message)); }
    }});
    if (b.type === "table") return [action(PR.t("复制表格"), "copy", async () => {
      const {head, rows} = PR.tableParts(b);
      const cell = value => PR.plain(String(value ?? "")).replace(/[\t\r\n]+/g, " ");
      const text = [...head,...rows].map(row => row.map(cell).join("\t")).join("\n");
      if (window.easyreadDesktop?.copyTable) await window.easyreadDesktop.copyTable(text);
      else await navigator.clipboard.writeText(text);
      PR.toast(PR.t("已复制"));
    })];
    if (!b.src) return [];
    return [action(PR.t("复制图片"),"copy",async () => {
      if (window.easyreadDesktop?.copyImage) {
        const png = await pngImage(b);
        const data = await new Promise((resolve,reject) => {
          const file = new FileReader(); file.onload = () => resolve(file.result.split(",")[1]);
          file.onerror = () => reject(file.error); file.readAsDataURL(png);
        });
        await window.easyreadDesktop.copyImage(data); PR.toast(PR.t("已复制")); return;
      }
      if (!navigator.clipboard?.write || !window.ClipboardItem) throw Error(PR.t("当前环境不支持复制图片，可另存原图"));
      await navigator.clipboard.write([new ClipboardItem({"image/png":pngImage(b)})]); PR.toast(PR.t("已复制"));
    }),action(PR.t("另存原图"),"download",async () => {
      const blob = await originalImage(b);
      const ext = ({"image/jpeg":"jpg","image/png":"png","image/webp":"webp","image/gif":"gif","image/svg+xml":"svg"})[blob.type.split(";")[0]] ||
        String(b.src).split(/[?#]/)[0].match(/\.(png|jpe?g|webp|gif|svg)$/i)?.[1] || "img";
      await PR.saveExport(blob, (label(b)+"-"+b.id).replace(/[\\/:*?"<>|]/g,"") + "." + ext);
    })];
  }
  PR.media = { transferActions, isMedia, blocks, label, caption, reference, resolve, html, hover, thumbnail };
})(window.PR);
