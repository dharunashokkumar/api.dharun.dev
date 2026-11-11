// The playground: find an endpoint, build and send a request, share it through the part
// of the address after #, keep a history in this browser, run ready-made chains, or
// simulate answers and the rate limit without calling the server.
(function () {
  "use strict";
  var T = window.indiaTry, el = T.el;
  var page = JSON.parse(document.getElementById("page-data").textContent);
  var specs = page.specs, byID = {}, byPath = {};
  specs.forEach(function (s) { byID[s.id] = s; byPath[s.path] = byPath[s.path] || s; });
  var list = document.getElementById("play-list"), find = document.getElementById("play-find"), bench = document.getElementById("play-bench");
  var simulate = document.getElementById("play-simulate"), allowanceBox = document.getElementById("play-allowance");
  var historyKey = "india-play-history", samples = null, spent = 0, windowStart = Date.now();

  document.getElementById("play-key").textContent = window.indiaKey() ?
    "Requests use the API key you made on this browser (" + page.keyLimit + " units a minute)." :
    "No key on this browser: requests count against your address (" + page.limit + " units a minute). Make a free key at /account to raise it.";

  function draw(filter) {
    list.textContent = "";
    var words = (filter || "").toLowerCase().split(/\s+/).filter(Boolean);
    specs.forEach(function (s) {
      var hay = (s.summary + " " + s.path + " " + s.groupTitle + " " + s.group).toLowerCase();
      if (words.some(function (w) { return hay.indexOf(w) < 0; })) return;
      var item = el("li"), button = el("button", { type: "button", "data-id": s.id });
      button.appendChild(document.createTextNode(s.summary + " "));
      button.appendChild(el("code", null, s.method + " " + s.path));
      button.addEventListener("click", function () { open(s, null); });
      item.appendChild(button);
      list.appendChild(item);
    });
  }
  find.addEventListener("input", function () { draw(find.value); });
  draw("");

  function extraHeaders() {
    var out = {};
    document.getElementById("play-headers").value.split("\n").forEach(function (line) {
      var i = line.indexOf(":");
      if (i > 0) out[line.slice(0, i).trim()] = line.slice(i + 1).trim();
    });
    return out;
  }

  function meterSimulated(spec) {
    if (Date.now() - windowStart > 60000) { spent = 0; windowStart = Date.now(); }
    spent += spec.cost || 0;
    var limit = window.indiaKey() ? page.keyLimit : page.limit;
    allowanceBox.textContent = "Simulated allowance: " + Math.max(limit - spent, 0) + " of " + limit + " units left this minute (this endpoint costs " + (spec.cost || 0) + ").";
    return spent > limit;
  }

  async function simulated(spec, out) {
    if (!samples) samples = await (await fetch("/play/samples.json")).json();
    out.textContent = "";
    if (meterSimulated(spec)) {
      out.appendChild(el("p", { class: "status error" }, "Simulated 429: the allowance is spent. A real answer would carry Retry-After: " + Math.ceil((60000 - (Date.now() - windowStart)) / 1000) + "."));
      return;
    }
    var body = samples[spec.id];
    if (!body) { out.appendChild(el("p", { class: "note" }, "No documented sample for this endpoint (it returns a file, or its answer depends on the network)." )); return; }
    out.appendChild(el("p", { class: "note" }, "Simulated: the documented sample answer, not a call to the server."));
    var pre = el("pre", { class: "sample" }); pre.appendChild(el("code", null, body)); out.appendChild(pre);
  }

  function remember(spec, form) {
    if (spec.private) return;
    var items = [];
    try { items = JSON.parse(localStorage.getItem(historyKey) || "[]"); } catch (e) {}
    items.unshift({ id: spec.id, values: form.values(), at: new Date().toISOString() });
    items = items.slice(0, 20);
    try { localStorage.setItem(historyKey, JSON.stringify(items)); } catch (e) {}
    drawHistory();
  }
  function drawHistory() {
    var box = document.getElementById("play-history"), items = [];
    try { items = JSON.parse(localStorage.getItem(historyKey) || "[]"); } catch (e) {}
    box.textContent = "";
    items.forEach(function (item) {
      var spec = byID[item.id];
      if (!spec) return;
      var li = el("li"), b = el("button", { type: "button", class: "key key--small" }, "Open");
      li.appendChild(el("span", null, spec.summary + " · " + item.at.slice(0, 16).replace("T", " ")));
      b.addEventListener("click", function () { open(spec, item.values); });
      li.appendChild(b);
      box.appendChild(li);
    });
    if (!items.length) box.appendChild(el("li", null, "Nothing yet."));
  }
  drawHistory();

  function share(spec, form) {
    if (spec.private) { history.replaceState(null, "", "#" + spec.id); return; }
    var hash = new URLSearchParams(form.values());
    hash.set("_", spec.id);
    history.replaceState(null, "", "#" + hash.toString());
  }

  function agentLine(spec, form) {
    var v = form.values(), parts = Object.keys(v).map(function (k) { return k + " " + v[k]; });
    return "Ask your assistant (with the India API MCP server connected): “Use India API to " + spec.summary.toLowerCase() + (parts.length ? " for " + parts.join(", ") : "") + ".”";
  }

  var current = null;
  function open(spec, values) {
    list.querySelectorAll("button").forEach(function (b) { b.toggleAttribute("aria-current", b.getAttribute("data-id") === spec.id); });
    bench.textContent = "";
    bench.appendChild(el("h2", null, spec.summary));
    bench.appendChild(el("p", { class: "route" }, spec.method + " " + spec.path + (spec.key ? " · will need a free key" : "") + " · costs " + spec.cost + (spec.cost === 1 ? " unit" : " units")));
    var form = T.build(bench, spec);
    if (values) form.fill(values);
    var actions = el("div", { class: "row" }), run = el("button", { type: "button", class: "key key--primary" }, "Send");
    actions.appendChild(run);
    T.errorDemos(spec).forEach(function (demo) {
      var b = el("button", { type: "button", class: "key key--small" }, demo[0]);
      b.addEventListener("click", function () { demo[1](form); run.click(); });
      actions.appendChild(b);
    });
    var docs = el("a", { href: "/docs/" + spec.group + "#" + spec.id.slice(spec.group.length + 1), class: "key key--small" }, "Documentation");
    actions.appendChild(docs);
    bench.appendChild(actions);
    var agent = el("p", { class: "note" }), codes = el("div", { class: "answer" }), out = el("div", { class: "answer", "aria-live": "polite" });
    bench.appendChild(agent); bench.appendChild(codes); bench.appendChild(out);
    async function update() {
      T.drawSamples(codes, T.samples(await form.request(false), spec));
      agent.textContent = agentLine(spec, form);
      share(spec, form);
    }
    form.onChange(update);
    update();
    run.addEventListener("click", async function () {
      run.disabled = true;
      try {
        if (simulate.checked) { await simulated(spec, out); return; }
        var req = await form.request(true), extra = extraHeaders();
        for (var k in extra) req.init.headers[k] = extra[k];
        current = await T.send(spec, req, out, { siteToken: true });
        remember(spec, form);
      } finally { run.disabled = false; }
    });
    current = null;
    return { form: form, run: run, out: out };
  }

  // Chains: "from" functions turn the answers so far into the next step's values.
  function today() { return new Date(Date.now() + 5.5 * 3600000).toISOString().slice(0, 10); }
  var transforms = {
    pincodeToSky: function (answers) { return { date: today(), pincode: answers[0].json.data.pincode }; },
    skyToCard: function (answers) {
      var place = answers[0].json.data, sun = answers[1].json.data.sun;
      return { title: "Sunrise in " + place.district + ": " + sun.sunrise.slice(11, 16), subtitle: "Sunset " + sun.sunset.slice(11, 16) + ", from the India API", site: location.host };
    },
    gstinToState: function (answers) { return { code: answers[0].json.data.parts.stateCode }; },
    stateToHolidays: function (answers) { return { year: today().slice(0, 4), state: answers[1].json.data.code }; },
    domainOn: function (answers) { return { host: answers[0].json.data.name, query: answers[0].json.data.name }; },
    ocrToTranslate: function (answers) { return { text: (answers[0].json.data.text || "").slice(0, 2000), from: "ta", to: "en" }; },
    translateToSpeech: function (answers) { return { text: answers[1].json.data.translation || answers[1].json.data.text || "" }; },
    blobToNext: function () { return { page: "1", width: "600" }; }
  };
  var chainBox = document.getElementById("play-chains");
  page.chains.forEach(function (chain) {
    var part = el("div", { class: "card" }), steps = el("ol", { class: "chain" }), go = el("button", { type: "button", class: "key" }, "Run this chain");
    part.appendChild(el("h3", null, chain.title));
    part.appendChild(el("p", { class: "note" }, chain.about));
    chain.steps.forEach(function (step) { var spec = byPath[step.path]; steps.appendChild(el("li", null, spec ? spec.summary + " (" + step.path + ")" : step.path)); });
    part.appendChild(steps);
    var out = el("div", { class: "answer" });
    part.appendChild(go); part.appendChild(out);
    go.addEventListener("click", async function () {
      go.disabled = true;
      var answers = [], previousBlob = null;
      out.textContent = "";
      try {
        for (var i = 0; i < chain.steps.length; i++) {
          var step = chain.steps[i], spec = byPath[step.path];
          if (!spec) throw new Error("No endpoint " + step.path);
          var box = el("div", { class: "card" }), holder = el("div");
          box.appendChild(el("h3", null, (i + 1) + ". " + spec.summary));
          box.appendChild(holder);
          out.appendChild(box);
          var form = T.build(holder, spec);
          holder.hidden = true;
          if (step.set) form.fill(Object.fromEntries(new URLSearchParams(step.set)));
          if (step.from) form.fill(transforms[step.from](answers));
          var req = await form.request(true);
          if (step.from === "blobToNext" && previousBlob) { req.init.body = previousBlob; req.init.headers["Content-Type"] = previousBlob.type; }
          var answerBox = el("div", { class: "answer" });
          box.appendChild(answerBox);
          var answer = await T.send(spec, req, answerBox, { siteToken: true });
          if (!answer || answer.status >= 400) throw new Error("Step " + (i + 1) + " failed; the chain stops here.");
          answers.push(answer);
          previousBlob = answer.blob || null;
        }
      } catch (e) {
        out.appendChild(el("p", { class: "status error" }, e.message));
      } finally { go.disabled = false; }
    });
    chainBox.appendChild(part);
  });

  // A shared link: #_=<endpoint id>&name=value...
  if (location.hash.length > 1) {
    var shared = new URLSearchParams(location.hash.slice(1)), id = shared.get("_") || location.hash.slice(1);
    if (byID[id]) {
      shared.delete("_");
      open(byID[id], Object.fromEntries(shared));
    }
  }
})();
