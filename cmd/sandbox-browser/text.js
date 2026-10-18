function (markdown) {
  // The page's text after its scripts ran: as the reader sees it (innerText), or as
  // Markdown with headings, lists, links, emphasis, code and tables kept.
  const result = {
    url: location.href,
    title: document.title || "",
    language: document.documentElement.lang || "",
    text: "",
  };
  const limit = 2000000;
  if (!markdown) {
    result.text = (document.body ? document.body.innerText : "").slice(0, limit);
    return result;
  }
  const skip = new Set(["SCRIPT", "STYLE", "NOSCRIPT", "SVG", "TEMPLATE", "IFRAME", "CANVAS", "OBJECT", "HEAD", "BUTTON", "SELECT", "INPUT", "TEXTAREA"]);
  const hidden = (el) => {
    const style = getComputedStyle(el);
    return style.display === "none" || style.visibility === "hidden";
  };
  const squash = (s) => s.replace(/\s+/g, " ");
  const inline = (node) => {
    let out = "";
    for (const child of node.childNodes) {
      if (child.nodeType === Node.TEXT_NODE) {
        out += squash(child.textContent);
        continue;
      }
      if (child.nodeType !== Node.ELEMENT_NODE || skip.has(child.tagName) || hidden(child)) continue;
      const inner = inline(child);
      switch (child.tagName) {
        case "A": {
          const href = child.href;
          out += href && inner.trim() && !href.startsWith("javascript:") ? `[${inner.trim()}](${href})` : inner;
          break;
        }
        case "STRONG": case "B": out += inner.trim() ? `**${inner.trim()}**` : ""; break;
        case "EM": case "I": out += inner.trim() ? `*${inner.trim()}*` : ""; break;
        case "CODE": out += inner ? "`" + inner + "`" : ""; break;
        case "BR": out += "  \n"; break;
        case "IMG": out += child.alt ? `![${squash(child.alt)}](${child.src})` : ""; break;
        default: out += inner;
      }
    }
    return out;
  };
  const blocks = [];
  const walk = (node, depth) => {
    for (const child of node.childNodes) {
      if (child.nodeType === Node.TEXT_NODE) {
        const text = squash(child.textContent).trim();
        if (text) blocks.push(text);
        continue;
      }
      if (child.nodeType !== Node.ELEMENT_NODE || skip.has(child.tagName) || hidden(child)) continue;
      const tag = child.tagName;
      if (/^H[1-6]$/.test(tag)) {
        const text = inline(child).trim();
        if (text) blocks.push("#".repeat(Number(tag[1])) + " " + text);
      } else if (tag === "P") {
        const text = inline(child).trim();
        if (text) blocks.push(text);
      } else if (tag === "PRE") {
        blocks.push("```\n" + child.innerText.replace(/\n+$/, "") + "\n```");
      } else if (tag === "BLOCKQUOTE") {
        const text = inline(child).trim();
        if (text) blocks.push(text.split("\n").map((line) => "> " + line).join("\n"));
      } else if (tag === "UL" || tag === "OL") {
        const items = [];
        let n = 1;
        for (const item of child.children) {
          if (item.tagName !== "LI" || hidden(item)) continue;
          const text = inline(item).trim();
          if (text) items.push("  ".repeat(depth) + (tag === "OL" ? `${n++}. ` : "- ") + text);
        }
        if (items.length) blocks.push(items.join("\n"));
      } else if (tag === "TABLE") {
        const rows = [...child.querySelectorAll("tr")].map((row) =>
          [...row.children].map((cell) => inline(cell).trim().replace(/\|/g, "\\|")));
        if (rows.length) {
          const width = Math.max(...rows.map((row) => row.length));
          const line = (row) => "| " + Array.from({ length: width }, (_, i) => row[i] || "").join(" | ") + " |";
          blocks.push([line(rows[0]), "|" + " --- |".repeat(width), ...rows.slice(1).map(line)].join("\n"));
        }
      } else if (tag === "HR") {
        blocks.push("---");
      } else if (tag === "IMG") {
        if (child.alt) blocks.push(`![${squash(child.alt)}](${child.src})`);
      } else {
        walk(child, tag === "LI" ? depth + 1 : depth);
      }
      if (blocks.reduce((sum, block) => sum + block.length, 0) > limit) return;
    }
  };
  if (document.body) walk(document.body, 0);
  result.text = blocks.join("\n\n").slice(0, limit);
  return result;
}
