// The tool pages: one small form each, calling this site's API from the browser.
// Model tools send a site token (pow.js) so they keep working once keys are required.
(function () {
  "use strict";
  var form = document.querySelector("form[data-tool]");
  if (!form) return;
  var tool = form.getAttribute("data-tool"), out = document.getElementById("tool-answer");
  function el(tag, attrs, text) {
    var node = document.createElement(tag);
    for (var k in attrs || {}) node.setAttribute(k, attrs[k]);
    if (text !== undefined) node.textContent = text;
    return node;
  }
  function value(name) { var f = form.elements[name]; return f ? f.value.trim() : ""; }
  function today() { return new Date(Date.now() + 5.5 * 3600000).toISOString().slice(0, 10); }
  function show() { out.hidden = false; out.textContent = ""; return out; }
  function error(message) { show().appendChild(el("p", { class: "status error" }, message)); }
  async function headers(model) {
    var h = {}, key = window.indiaKey ? window.indiaKey() : "";
    if (key) h["X-API-Key"] = key;
    else if (model && window.indiaSiteToken) { try { var t = await window.indiaSiteToken(); if (t) h["X-Site-Token"] = t; } catch (e) {} }
    return h;
  }
  async function getJSON(url, model) {
    var response = await fetch(url, { headers: await headers(model) });
    var data = await response.json();
    if (!response.ok) throw new Error(data.error ? data.error.message : "Something went wrong.");
    return data;
  }
  function fileAnswer(blob, name) {
    var box = show(), link = URL.createObjectURL(blob);
    if (blob.type.indexOf("image/") === 0) box.appendChild(el("img", { src: link, alt: "The result" }));
    if (blob.type === "application/pdf") box.appendChild(el("iframe", { src: link, title: "The PDF" }));
    box.appendChild(el("p")).appendChild(el("a", { href: link, download: name, class: "key key--primary" }, "Download " + name));
  }
  function table(rows, columns) {
    var t = el("table"), head = el("tr");
    columns.forEach(function (c) { head.appendChild(el("th", null, c[0])); });
    t.appendChild(el("thead")).appendChild(head);
    var body = el("tbody");
    rows.forEach(function (row) { var tr = el("tr"); columns.forEach(function (c) { tr.appendChild(el("td", null, c[1](row) || "")); }); body.appendChild(tr); });
    t.appendChild(body);
    var wrap = el("div", { class: "prose" }); wrap.appendChild(t); return wrap;
  }

  var handlers = {
    "gst-invoice": async function () {
      var items = [];
      document.querySelectorAll("#invoice-items .fields").forEach(function (row) {
        var get = function (n) { return row.querySelector("[name=" + n + "]").value.trim(); };
        if (get("description")) items.push({ description: get("description"), hsn: get("hsn"), quantity: Number(get("quantity")) || 1, rate: Number(get("rate")) || 0, gst_rate: Number(get("gst_rate")) || 0 });
      });
      var invoice = { number: value("number"), date: value("date") || today(), items: items, upi: value("upi") || undefined,
        seller: { name: value("seller.name"), gstin: value("seller.gstin"), address: value("seller.address") },
        buyer: { name: value("buyer.name"), gstin: value("buyer.gstin") || undefined, address: value("buyer.address") } };
      var response = await fetch("/v1/invoice", { method: "POST", headers: Object.assign({ "Content-Type": "application/json" }, await headers(false)), body: JSON.stringify(invoice) });
      if (!response.ok) { var e = await response.json(); throw new Error(e.error.message); }
      fileAnswer(await response.blob(), invoice.number + ".pdf");
    },
    "upi-qr": async function () {
      var q = new URLSearchParams({ pa: value("pa"), pn: value("pn"), format: "png", size: "512" });
      if (value("am")) q.set("am", value("am"));
      if (value("tn")) q.set("tn", value("tn"));
      var response = await fetch("/v1/upi-qr?" + q);
      if (!response.ok) { var e = await response.json(); throw new Error(e.error.message); }
      fileAnswer(await response.blob(), "upi-qr.png");
    },
    "ifsc": async function () {
      if (value("code")) {
        var one = (await getJSON("/v1/ifsc/" + encodeURIComponent(value("code").toUpperCase()))).data;
        show().appendChild(table([one], [["IFSC", function (r) { return r.ifsc; }], ["Bank", function (r) { return r.bank.name; }], ["Branch", function (r) { return r.branch; }], ["Address", function (r) { return r.address; }], ["MICR", function (r) { return r.micr; }], ["Takes", function (r) { return ["neft", "rtgs", "imps", "upi"].filter(function (k) { return r[k]; }).join(", ").toUpperCase(); }]]));
        return;
      }
      var bank = value("bank");
      if (bank && !/^[A-Z]{4}$/.test(bank)) {
        var banks = (await getJSON("/v1/banks?limit=1&q=" + encodeURIComponent(bank))).data;
        if (!banks.length) throw new Error("No bank by that name.");
        bank = banks[0].code;
      }
      var q = new URLSearchParams({ limit: "50" });
      if (bank) q.set("bank", bank);
      if (value("state")) q.set("state", value("state"));
      if (value("q")) q.set("q", value("q"));
      var found = await getJSON("/v1/ifsc?" + q), box = show();
      box.appendChild(el("p", { class: "note" }, found.meta.total + " branches" + (found.meta.total > 50 ? ", the first 50 shown" : "") + "."));
      box.appendChild(table(found.data, [["IFSC", function (r) { return r.ifsc; }], ["Bank", function (r) { return r.bank.name; }], ["Branch", function (r) { return r.branch; }], ["City", function (r) { return r.city; }], ["State", function (r) { return r.state; }]]));
    },
    "pincode": async function () {
      var q = value("q"), rows;
      if (/^\d{6}$/.test(q)) rows = (await getJSON("/v1/pincode/" + q)).data.offices;
      else rows = (await getJSON("/v1/pincode?limit=50&q=" + encodeURIComponent(q))).data;
      show().appendChild(table(rows, [["Pincode", function (r) { return r.pincode || q; }], ["Post office", function (r) { return r.name; }], ["Type", function (r) { return r.type; }], ["District", function (r) { return r.district; }], ["State", function (r) { return r.state; }]]));
    },
    "amount-in-words": async function () {
      var data = (await getJSON("/v1/words/" + encodeURIComponent(value("number").replace(/,/g, "")) + "?style=currency&lang=" + value("lang"))).data, box = show();
      box.appendChild(el("p", { class: "big" }, data.words));
      var copy = el("button", { type: "button", class: "key key--small" }, "Copy");
      copy.addEventListener("click", function () { navigator.clipboard.writeText(data.words); copy.textContent = "Copied"; });
      box.appendChild(copy);
    },
    "bank-holidays": async function () {
      var year = value("year") || today().slice(0, 4), type = value("type"), q = new URLSearchParams({ type: type });
      if (type === "bank") q.set("state", value("state"));
      var data = (await getJSON("/v1/holidays/" + year + "?" + q)).data, box = show();
      box.appendChild(table(data, [["Date", function (r) { return r.date; }], ["Day", function (r) { return r.weekday; }], ["Holiday", function (r) { return r.name; }]]));
      var feed = location.origin + "/v1/holidays/" + year + ".ics?" + q;
      var p = el("p", { class: "aside" });
      p.appendChild(document.createTextNode("Calendar to subscribe to: "));
      p.appendChild(el("code", null, feed));
      p.appendChild(document.createTextNode(" "));
      p.appendChild(el("a", { href: feed.replace(/^https?:/, "webcal:") }, "Open in your calendar app"));
      box.appendChild(p);
    },
    "panchang": async function () {
      var d = (await getJSON("/v1/panchang/" + (value("date") || today()) + "?pincode=" + encodeURIComponent(value("pincode")))).data, box = show();
      function first(list) { return list && list[0] ? list[0].name + (list[0].tamil ? " (" + list[0].tamil + ")" : "") + (list[0].end ? ", until " + list[0].end.slice(11, 16) : "") : ""; }
      var rows = [["Place", d.place.name || d.place.pincode], ["Weekday", d.weekday.name + " (" + d.weekday.tamil + ")"], ["Sunrise", (d.sunrise || "").slice(11, 16)], ["Sunset", (d.sunset || "").slice(11, 16)], ["Tithi", first(d.tithi)], ["Nakshatra", first(d.nakshatra)], ["Yoga", first(d.yoga)], ["Karana", first(d.karana)]];
      ["rahuKalam", "yamagandam", "gulikaKalam"].forEach(function (k) { if (d[k]) rows.push([k.replace(/([A-Z])/g, " $1").replace(/^./, function (c) { return c.toUpperCase(); }), d[k].start.slice(11, 16) + " to " + d[k].end.slice(11, 16)]); });
      box.appendChild(table(rows, [["", function (r) { return r[0]; }], ["", function (r) { return r[1]; }]]));
    },
    "image-to-text": async function () {
      var file = form.elements.file.files[0];
      if (!file) throw new Error("Choose a file first.");
      show().appendChild(el("p", { class: "status" }, "Reading…"));
      var response = await fetch("/v1/ocr?lang=" + value("lang"), { method: "POST", headers: Object.assign({ "Content-Type": file.type || "application/octet-stream" }, await headers(true)), body: file });
      var data = await response.json();
      if (!response.ok) throw new Error(data.error.message);
      var box = show(), area = el("textarea", { rows: "14", style: "width:100%" });
      area.value = data.data.text;
      box.appendChild(area);
    },
    "indian-language-typing": async function () {
      var data = (await getJSON("/v1/transliterate/suggest?lang=" + value("lang") + "&text=" + encodeURIComponent(value("text")), true)).data, box = show();
      var line = el("p", { class: "big", style: "font-size:var(--text-2xl)" }), words = data.words.map(function (w) { return w.suggestions[0] ? w.suggestions[0].text : w.input; });
      function redraw() { line.textContent = words.join(" "); }
      redraw();
      box.appendChild(line);
      data.words.forEach(function (w, i) {
        if (w.suggestions.length < 2) return;
        var row = el("div", { class: "row" });
        row.appendChild(el("span", { class: "note" }, w.input + ":"));
        w.suggestions.slice(0, 5).forEach(function (s) {
          var b = el("button", { type: "button", class: "key key--small" }, s.text);
          b.addEventListener("click", function () { words[i] = s.text; redraw(); });
          row.appendChild(b);
        });
        box.appendChild(row);
      });
      var copy = el("button", { type: "button", class: "key" }, "Copy");
      copy.addEventListener("click", function () { navigator.clipboard.writeText(line.textContent); copy.textContent = "Copied"; });
      box.appendChild(copy);
    },
    "pdf": async function () {
      var files = form.elements.file.files, action = value("action");
      if (!files.length) throw new Error("Choose a PDF first.");
      var url = "/v1/pdf/" + action, init = { method: "POST", headers: await headers(false) };
      if (action === "pages" || action === "rotate") url += "?" + new URLSearchParams(action === "rotate" ? { pages: value("pages") || "1-", angle: value("angle") } : { pages: value("pages") || "1" });
      if (action === "merge") {
        if (files.length < 2) throw new Error("Choose two or more PDFs to merge.");
        var data = new FormData();
        Array.prototype.forEach.call(files, function (f, i) { data.append("file" + (i + 1), f); });
        init.body = data;
      } else { init.body = files[0]; init.headers["Content-Type"] = "application/pdf"; }
      var response = await fetch(url, init);
      if (!response.ok) { var e = await response.json(); throw new Error(e.error.message); }
      if (action === "text") {
        var text = await response.json(), area = el("textarea", { rows: "14", style: "width:100%" });
        area.value = text.data.text || (text.data.pages || []).map(function (p) { return p.text; }).join("\n\n");
        show().appendChild(area);
      } else fileAnswer(await response.blob(), action + ".pdf");
    }
  };

  if (tool === "gst-invoice") {
    var items = document.getElementById("invoice-items");
    var addItem = function (d) {
      var row = el("div", { class: "fields" });
      [["description", "Item", d[0]], ["hsn", "HSN", d[1]], ["quantity", "Quantity", d[2]], ["rate", "Rate (₹)", d[3]], ["gst_rate", "GST %", d[4]]].forEach(function (f) {
        var label = el("label", { class: "field" }); label.appendChild(el("span", null, f[1]));
        var input = el("input", { name: f[0] }); input.value = f[2] === undefined ? "" : f[2];
        label.appendChild(input); row.appendChild(label);
      });
      items.appendChild(row);
    };
    addItem(["Basmati rice, 25 kg bag", "1006", 4, 2150, 5]);
    addItem(["Steel tiffin box", "7323", 10, 349.5, 18]);
    document.getElementById("add-item").addEventListener("click", function () { addItem(["", "", 1, "", 18]); });
    form.elements.date.value = today();
  }
  if (tool === "bank-holidays") form.elements.year.value = today().slice(0, 4);
  if (tool === "panchang") form.elements.date.value = today();

  form.addEventListener("submit", async function (event) {
    event.preventDefault();
    var button = form.querySelector("button[type=submit]");
    button.disabled = true;
    try { await handlers[tool](); } catch (e) { error(e.message); } finally { button.disabled = false; }
  });
})();
