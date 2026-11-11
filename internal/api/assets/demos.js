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
})();
