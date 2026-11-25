/* Reader-owned equation references and sentence navigation. */
(function (PR) {
  "use strict";
  PR.revealOriginalSentence = function (selection, captured) {
    PR.toggleEn(selection.anchor, true);
    const source = captured?.el;
    const original = source && PR.enOfZh(source);
    const ids = source && original ? Array.from(source.querySelectorAll(".snt"))
      .filter(span => captured.range.intersectsNode(span)).map(span => span.dataset.s) : [];
    let targets = original ? Array.from(original.querySelectorAll(".snt")).filter(span => ids.includes(span.dataset.s)) : [];
    if (!targets.length) {
      const fallback = original || PR.webAnnotationRoots?.(selection, "en")?.[0] || PR.webBlockHost?.(selection.anchor) || document.getElementById("b-" + selection.anchor);
      if (!fallback) return;
      targets = [fallback];
    }
    targets[0].scrollIntoView({block:"center"});
    for (const node of targets) node.animate([{backgroundColor:"var(--accent-soft)"}, {backgroundColor:"transparent"}], {duration:1800});
  };

  function targetOf(note) {
    const saved = note.equation, block = PR.blockById[saved.id];
    if (block?.type === "math" && block.tex === saved.tex) return block;
    const matches = PR.state.paper.blocks.filter(b => b.type === "math" && b.tex === saved.tex && b.tag === saved.tag && b.page === saved.page);
    return matches.length === 1 ? matches[0] : null;
  }
  PR.followEquation = note => {
    const block = targetOf(note);
    if (!block) return PR.pickEquation(note);
    PR.hidePopover(); PR.finishPaperChunks?.(); PR.jumpTo("b-" + block.id);
  };
  PR.pickEquation = function (selection) {
    const {anchor, key, start, quote, prefix, suffix, side} = selection;
    const saved = selection.equation ? selection : PR.myNotes().find(n => n.equation && n.anchor === anchor && n.key === key && n.start === start && n.side === side && n.quote === quote);
    const blocks = PR.state.paper.blocks.filter(b => b.type === "math");
    const dialog = document.createElement("dialog"), select = document.createElement("select"), preview = document.createElement("div");
    dialog.className = "equation-picker"; dialog.setAttribute("aria-label", PR.t("关联公式"));
    const label = document.createElement("label"); label.textContent = PR.t("选择目标公式"); label.append(select);
    for (const b of blocks) select.add(new Option(PR.t("第 {page} 页", {page:b.page}) + " · " + (b.tag ? "(" + b.tag + ") " : "") + b.tex.slice(0,70), b.id));
    const initial = saved && targetOf(saved); if (initial) select.value = initial.id;
    const render = () => { const b = PR.blockById[select.value]; preview.innerHTML = b ? PR.tex(b.tex,true) : ""; };
    select.onchange = render; render();
    const actions = document.createElement("div"); actions.className = "actions";
    function button(text, fn) {
      const btn = document.createElement("button"); btn.type = "button"; btn.className = "btn"; btn.textContent = text;
      btn.onclick = () => {fn(); dialog.close();}; actions.append(btn); return btn;
    }
    if (saved) button(PR.t("移除关联"), () => {PR.commit({op:"note_del",id:saved.id,at:PR.nowIso()});PR.applyMarks(anchor);});
    button(PR.t("取消"), () => {});
    button(PR.t("保存"), () => {
      const {id, tex, tag, page} = PR.blockById[select.value];
      const note = saved || {id:PR.uid("n"), kind:"highlight", anchor,key,start,quote,prefix,suffix,side,created:PR.nowIso()};
      PR.saveNote({...note,equation:{id,tex,tag,page}}); PR.applyMarks(anchor);
    }).disabled = !blocks.length;
    dialog.onclose = () => dialog.remove();
    dialog.onkeydown = event => event.stopPropagation();
    dialog.append(label, preview, actions); document.body.append(dialog); dialog.showModal(); select.focus();
  };
  function marked(e) { return e.target.closest?.("[data-equation-note]"); }
  document.addEventListener("mouseover", e => {
    const mark = marked(e); if (!mark) return;
    const note = PR.state.reader.notes[mark.dataset.equationNote], b = note && targetOf(note);
    if (b) PR.popover(mark, PR.tex(b.tex,true));
    e.stopImmediatePropagation();
  }, true);
  document.addEventListener("mouseout", e => { if (marked(e)) PR.hidePopoverSoon(); });
  document.addEventListener("contextmenu", e => {
    const mark = marked(e); if (!mark) return;
    e.preventDefault(); e.stopImmediatePropagation();
    PR.menu({x:e.clientX,y:e.clientY}, [{label:PR.t("修改公式关联"),fn:() => PR.pickEquation(PR.state.reader.notes[mark.dataset.equationNote])}]);
  },true);
  document.addEventListener("keydown", e => {
    const mark = marked(e); if (mark && e.key === "Enter") { e.preventDefault(); PR.followEquation(PR.state.reader.notes[mark.dataset.equationNote]); }
  });
})(window.PR);
