// The demos: each one a few calls to this site's API, drawn in the page.
(function () {
  "use strict";
  var root = document.querySelector("[data-demo]");
  if (!root) return;
  var demo = root.getAttribute("data-demo");
  function $(id) { return document.getElementById(id); }
  async function get(url) { var r = await fetch(url); return r.json(); }
  function ist() { return new Date(Date.now() + 5.5 * 3600000); }

  if (demo === "market-clock") {
    var holiday = null;
    async function tick() {
      try {
        var status = (await get("/v1/market/status")).data, now = ist(), state = $("clock-state");
        state.textContent = status.open ? "NSE is open" : "NSE is closed";
        state.style.color = status.open ? "var(--color-success)" : "var(--color-ink)";
        var hhmm = now.toISOString().slice(11, 16), close = (status.hours && status.hours.normal || "09:15-15:30").split("-");
        var mins = function (t) { return Number(t.slice(0, 2)) * 60 + Number(t.slice(3, 5)); }, at = mins(hhmm);
        var next = status.open ? close[1] : close[0], left = mins(next) - at;
        if (left < 0) left += 24 * 60;
        $("clock-next").textContent = (status.open ? "Closes" : "The next session opens") + " at " + next + " IST" + (status.open || left < 600 ? ", in " + Math.floor(left / 60) + " h " + (left % 60) + " min" : "") + ". Session: " + status.session + ".";
        if (!holiday) {
          var year = now.toISOString().slice(0, 4), list = (await get("/v1/holidays/" + year + "?type=market")).data || [];
          var today = now.toISOString().slice(0, 10);
          holiday = list.filter(function (h) { return h.date >= today; })[0] || null;
        }
        if (holiday) $("clock-holiday").textContent = "Next market holiday: " + holiday.name + ", " + holiday.weekday + " " + holiday.date + ".";
      } catch (e) { $("clock-state").textContent = "Could not reach the API."; }
    }
    tick();
    setInterval(tick, 30000);
  }

  if (demo === "script-switcher") {
    var scripts = ["devanagari", "bengali", "gurmukhi", "gujarati", "odia", "tamil", "telugu", "kannada", "malayalam", "iso15919"], timer = null;
    async function convert() {
      var text = $("scripts-text").value, out = $("scripts-out");
      out.textContent = "";
      for (var i = 0; i < scripts.length; i++) {
        var box = document.createElement("div"), label = document.createElement("small"), value = document.createElement("p");
        label.textContent = scripts[i];
        box.appendChild(label); box.appendChild(value); out.appendChild(box);
        try {
          var answer = await (await fetch("/v1/transliterate?to=" + scripts[i], { method: "POST", headers: { "Content-Type": "text/plain; charset=utf-8" }, body: text })).json();
          value.textContent = answer.data ? answer.data.text : answer.error.message;
        } catch (e) { value.textContent = "…"; }
      }
    }
    $("scripts-text").addEventListener("input", function () { clearTimeout(timer); timer = setTimeout(convert, 400); });
    convert();
  }

  if (demo === "logo-wall") {
    async function draw() {
      var index = $("wall-index").value, wall = $("wall"), all = [], offset = 0;
      wall.textContent = "Loading…";
      while (offset < 200) {
        var page = await get("/v1/companies?limit=100&offset=" + offset + "&index=" + encodeURIComponent(index));
        all = all.concat(page.data || []);
        offset += 100;
        if (!page.meta || offset >= page.meta.total) break;
      }
      var sectors = {};
      all.forEach(function (c) { (sectors[c.sector || "Other"] = sectors[c.sector || "Other"] || []).push(c); });
      wall.textContent = "";
      Object.keys(sectors).sort(function (a, b) { return sectors[b].length - sectors[a].length; }).forEach(function (sector) {
        var heading = document.createElement("h3"), grid = document.createElement("div");
        heading.textContent = sector + " (" + sectors[sector].length + ")";
        heading.style.marginTop = "var(--space-lg)";
        grid.className = "logo-wall";
        sectors[sector].forEach(function (c) {
          var figure = document.createElement("figure"), link = document.createElement("a"), img = document.createElement("img"), caption = document.createElement("figcaption");
          link.href = "/logos/" + encodeURIComponent(c.ticker); img.src = c.logo.svg; img.alt = c.company; img.width = 48; img.height = 48; img.loading = "lazy";
          caption.textContent = c.ticker;
          link.appendChild(img); figure.appendChild(link); figure.appendChild(caption); grid.appendChild(figure);
        });
        wall.appendChild(heading); wall.appendChild(grid);
      });
      if (!all.length) wall.textContent = "No companies found for that index.";
    }
    $("wall-index").addEventListener("change", draw);
    draw();
  }

  if (demo === "under-the-hood") {
    $("hood-go").addEventListener("click", async function () {
      var url = $("hood-url").value, steps = $("hood-steps");
      steps.textContent = "";
      function step(text) { var li = document.createElement("li"); li.textContent = text; steps.appendChild(li); }
      var started = performance.now(), response = await fetch(url, { cache: "no-store" }), ms = Math.round(performance.now() - started);
      var h = function (n) { return response.headers.get(n); };
      step("Sent GET " + url + " from your browser. Status " + response.status + " in " + ms + " ms, including the trip.");
      if (h("RateLimit-Limit")) step("Charged against your allowance: " + h("RateLimit-Remaining") + " of " + h("RateLimit-Limit") + " units left this minute, the window resets in " + h("RateLimit-Reset") + " s.");
      step("Cache: " + (h("Cache-Control") || "none") + ". ETag " + (h("ETag") || "none") + ": ask again with If-None-Match and an unchanged answer comes back as 304 with no body.");
      var type = h("Content-Type") || "";
      if (type.indexOf("json") >= 0) {
        var body = await response.json();
        if (body.meta && body.meta.source) step("Answered from " + body.meta.source.name + (body.meta.source.version ? ", version " + body.meta.source.version : "") + ", retrieved " + body.meta.source.retrieved + ". Licence: " + body.meta.source.licence + ".");
        if (body.error) step("Error code " + body.error.code + ": " + body.error.message);
      } else step("The answer is a file: " + type + ".");
      try {
        var health = await (await fetch("/healthz")).json();
        step("This server runs version " + health.version + " and holds " + Object.keys(health.datasets || {}).length + " datasets; each is a read-only SQLite file swapped in without a restart when a newer copy arrives.");
      } catch (e) {}
    });
  }
  function el(tag, attrs, text) { var n = document.createElement(tag); for (var k in attrs || {}) n.setAttribute(k, attrs[k]); if (text !== undefined) n.textContent = text; return n; }
  function time(iso) { return iso ? iso.slice(11, 16) : "–"; }
  function todayISO() { return ist().toISOString().slice(0, 10); }

  if (demo === "satellite-tracker") {
    async function passes() {
      var out = $("sat-out"); out.textContent = "…";
      try {
        var body = await get("/v1/sat-pass?lat=" + encodeURIComponent($("sat-lat").value) + "&lng=" + encodeURIComponent($("sat-lng").value));
        if (body.error) throw new Error(body.error.message);
        var d = body.data, list = el("ol", { class: "chain" });
        out.textContent = "";
        out.appendChild(el("p", { class: "lead" }, d.satellite.name + ": " + d.passes.length + " passes in the next three days."));
        d.passes.forEach(function (p) {
          list.appendChild(el("li", null, p.rise.time.slice(0, 10) + " · rises " + time(p.rise.time) + " in the " + p.rise.direction + ", highest " + Math.round(p.culmination.elevation) + "° in the " + p.culmination.direction + " at " + time(p.culmination.time) + ", sets " + time(p.set.time) + " in the " + p.set.direction + (p.visible ? " · visible to the eye" : "")));
        });
        out.appendChild(list);
      } catch (e) { out.textContent = e.message || "Could not reach the API."; }
    }
    $("sat-go").addEventListener("click", passes);
    $("sat-here").addEventListener("click", function () {
      if (!navigator.geolocation) return;
      navigator.geolocation.getCurrentPosition(function (pos) { $("sat-lat").value = pos.coords.latitude.toFixed(3); $("sat-lng").value = pos.coords.longitude.toFixed(3); passes(); });
    });
    passes();
  }

  if (demo === "path-from-india") {
    $("path-go").addEventListener("click", async function () {
      var out = $("path-out"); out.textContent = ""; out.appendChild(el("li", null, "Tracing… about ten seconds."));
      try {
        var body = await get("/v1/mtr/" + encodeURIComponent($("path-host").value.trim()));
        if (body.error) throw new Error(body.error.message);
        out.textContent = "";
        body.data.hops.forEach(function (h) {
          var who = h.private ? "inside the server's own network" : (h.address ? (h.hostname || h.address) + (h.network ? " · " + h.network.organization : "") + (h.country ? " · " + h.country : "") : "no answer");
          out.appendChild(el("li", null, "Hop " + h.hop + ": " + who + (h.averageMs !== null ? " · " + h.averageMs + " ms" : "")));
        });
        out.appendChild(el("li", null, body.data.reached ? "Reached " + body.data.host + " (" + body.data.address + ")." : "The host did not answer the last probes."));
      } catch (e) { out.textContent = e.message || "Could not reach the API."; }
    });
  }

  if (demo === "panchang-dial") {
    $("dial-date").value = todayISO();
    async function draw() {
      var out = $("dial-out"); out.textContent = "…";
      try {
        var body = await get("/v1/panchang/" + $("dial-date").value + "?pincode=" + encodeURIComponent($("dial-pin").value));
        if (body.error) throw new Error(body.error.message);
        var d = body.data, t = d.tithi[0], n = d.nakshatra[0];
        var phase = (t.paksha === "shukla" ? t.number : 15 + t.number) / 30; // 0 new moon, 0.5 full
        var ns = "http://www.w3.org/2000/svg", svg = document.createElementNS(ns, "svg");
        svg.setAttribute("viewBox", "-110 -110 220 220"); svg.setAttribute("width", "220"); svg.setAttribute("role", "img"); svg.setAttribute("aria-label", t.name + ", " + t.paksha + " paksha");
        var ring = document.createElementNS(ns, "circle"); ring.setAttribute("r", "100"); ring.setAttribute("fill", "none"); ring.setAttribute("stroke", "currentColor"); ring.setAttribute("stroke-opacity", "0.25"); svg.appendChild(ring);
        for (var i = 0; i < 30; i++) {
          var a = (i / 30) * 2 * Math.PI - Math.PI / 2, dot = document.createElementNS(ns, "circle");
          dot.setAttribute("cx", (100 * Math.cos(a)).toFixed(1)); dot.setAttribute("cy", (100 * Math.sin(a)).toFixed(1)); dot.setAttribute("r", i === Math.round(phase * 30) % 30 ? "7" : "2.5");
          dot.setAttribute("fill", i === Math.round(phase * 30) % 30 ? "var(--color-accent, #2b50d8)" : "currentColor"); svg.appendChild(dot);
        }
        var moon = document.createElementNS(ns, "circle"); moon.setAttribute("r", "45"); moon.setAttribute("fill", "#e9e4d4"); svg.appendChild(moon);
        var lit = Math.cos(phase * 2 * Math.PI), shade = document.createElementNS(ns, "path");
        var rx = Math.abs(lit) * 45, waxing = phase < 0.5;
        shade.setAttribute("d", "M0,-45 A45,45 0 0," + (waxing ? 0 : 1) + " 0,45 A" + rx.toFixed(1) + ",45 0 0," + ((lit > 0) === waxing ? 1 : 0) + " 0,-45 Z");
        shade.setAttribute("fill", "#22262e"); svg.appendChild(shade);
        out.textContent = "";
        out.appendChild(svg);
        var dl = el("dl", { class: "pairs" });
        [["Tithi", t.name + " (" + t.tamil + "), " + t.paksha + " paksha, until " + time(t.end)], ["Nakshatra", n.name + " (" + n.tamil + "), until " + time(n.end)],
          ["Sunrise · sunset", time(d.sunrise) + " · " + time(d.sunset)], ["Rahu kalam", time(d.rahuKalam && d.rahuKalam.start) + " – " + time(d.rahuKalam && d.rahuKalam.end)],
          ["Place", d.place.name]].forEach(function (p) { dl.appendChild(el("dt", null, p[0])); dl.appendChild(el("dd", null, p[1])); });
        out.appendChild(dl);
      } catch (e) { out.textContent = e.message || "Could not reach the API."; }
    }
    $("dial-go").addEventListener("click", draw);
    draw();
  }

  if (demo === "day-length") {
    var cities = [["Kanyakumari", "629702"], ["Chennai", "600001"], ["Mumbai", "400001"], ["Kolkata", "700001"], ["Delhi", "110001"], ["Guwahati", "781001"], ["Leh", "194101"]];
    $("day-date").value = todayISO();
    async function load() {
      var out = $("day-out"); out.textContent = "…";
      var rows = await Promise.all(cities.map(async function (c) {
        try { var d = (await get("/v1/sun-moon/" + $("day-date").value + "?pincode=" + c[1])).data; return [c[0], d.sun.sunrise, d.sun.sunset, d.sun.dayLengthMinutes]; } catch (e) { return [c[0], "", "", 0]; }
      }));
      out.textContent = "";
      var max = Math.max.apply(null, rows.map(function (r) { return r[3]; }));
      rows.forEach(function (r) {
        var line = el("div", { class: "card" }), bar = el("div");
        bar.style.cssText = "height:10px;border-radius:5px;background:var(--color-accent, #2b50d8);width:" + (max ? (r[3] / max * 100).toFixed(1) : 0) + "%";
        line.appendChild(el("p", null, r[0] + ": " + Math.floor(r[3] / 60) + " h " + Math.round(r[3] % 60) + " min · sunrise " + time(r[1]) + " · sunset " + time(r[2])));
        line.appendChild(bar);
        out.appendChild(line);
      });
    }
    $("day-date").addEventListener("change", load);
    load();
  }

  if (demo === "radio-weather") {
    (async function () {
      var out = $("radio-out");
      try {
        var d = (await get("/v1/propagation")).data;
        out.textContent = "";
        out.appendChild(el("p", { class: "big" }, "Geomagnetic field: " + d.kIndex.level + " (K " + d.kIndex.value + ")"));
        out.appendChild(el("p", { class: "lead" }, "Solar flux " + d.solarFlux.value + ", sunspots " + d.sunspotNumber.value + ", solar wind " + d.solarWindSpeed.value + " km/s, X-rays " + d.xray.class + ". NOAA scales: " + d.noaaScales.radioBlackout + " " + d.noaaScales.solarRadiation + " " + d.noaaScales.geomagneticStorm + "."));
        var t = el("table"), head = el("tr");
        ["Band", "Day", "Night"].forEach(function (h) { head.appendChild(el("th", null, h)); });
        t.appendChild(el("thead")).appendChild(head);
        var tb = el("tbody");
        d.bands.forEach(function (b) { var tr = el("tr"); [b.band, b.day, b.night].forEach(function (v) { tr.appendChild(el("td", null, v)); }); tb.appendChild(tr); });
        t.appendChild(tb);
        var wrap = el("div", { class: "prose" }); wrap.appendChild(t); out.appendChild(wrap);
      } catch (e) { out.textContent = "Could not reach the API."; }
    })();
  }

  if (demo === "invoice-live") {
    var timer = null, last = null;
    async function render() {
      var invoice = { number: "LIVE-1", date: todayISO(), seller: { name: "Sri Murugan Stores", address: "12 Car Street, Namakkal 637001", gstin: "33AABCS1429B1Z1" },
        buyer: { name: "Kaveri Traders", address: "4 MG Road", gstin: $("inv-buyer").value.trim() || undefined },
        items: [{ description: $("inv-item").value, quantity: Number($("inv-qty").value) || 1, rate: Number($("inv-rate").value) || 0, gst_rate: Number($("inv-gst").value) || 0 }] };
      try {
        var r = await fetch("/v1/invoice?format=png", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(invoice) });
        if (!r.ok) { var e = await r.json(); $("inv-out").setAttribute("data-error", e.error.message); return; }
        if (last) URL.revokeObjectURL(last);
        last = URL.createObjectURL(await r.blob());
        $("inv-img").src = last;
      } catch (e) {}
    }
    ["inv-item", "inv-qty", "inv-rate", "inv-gst", "inv-buyer"].forEach(function (id) { $(id).addEventListener("input", function () { clearTimeout(timer); timer = setTimeout(render, 600); }); });
    render();
  }
})();
