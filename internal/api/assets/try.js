// The try-it panel: a form built from an endpoint's catalog entry (its spec), filled with
// the documented example, that sends the request from this browser and shows the answer.
// The playground (play.js) and the tool pages reuse window.indiaTry.
//
// Private endpoints: nothing typed is stored, and nothing goes into the address bar.
(function () {
  "use strict";
  function el(tag, attrs, text) {
    var node = document.createElement(tag);
    for (var k in attrs || {}) {
      if (k === "class") node.className = attrs[k]; else if (attrs[k] !== null && attrs[k] !== undefined) node.setAttribute(k, attrs[k]);
    }
    if (text !== undefined) node.textContent = text;
    return node;
  }
  function textOf(spec) {
    var p = spec.params.filter(function (x) { return x.name === "text"; })[0];
    return p ? p.value || "" : "";
  }

  // build draws the form into box and returns {request(), fill(values)}.
  function build(box, spec, options) {
    options = options || {};
    var fields = el("div", { class: "fields" }), inputs = {};
    spec.params.forEach(function (p) {
      var wrap = el("label", { class: "field" }), name = el("span", null, p.name + (p.required ? " *" : "") + " ");
      name.appendChild(el("small", null, p.in));
      wrap.appendChild(name);
      var input;
      if (p.enum && p.enum.length) {
        input = el("select", { name: p.name });
        if (!p.required) input.appendChild(el("option", { value: "" }, "(default)"));
        p.enum.forEach(function (v) { input.appendChild(el("option", { value: v }, v)); });
      } else if (p.name === "text" || p.name === "code") {
        input = el("textarea", { name: p.name, rows: 3 });
      } else {
        input = el("input", { name: p.name, type: p.type === "integer" ? "number" : "text", autocomplete: "off", spellcheck: "false" });
      }
      input.value = p.value || "";
      input.title = p.doc;
      wrap.appendChild(input);
      wrap.appendChild(el("small", null, p.doc));
      inputs[p.name] = input;
      fields.appendChild(wrap);
    });
    box.appendChild(fields);

    var body = null, file = null, usePost = null;
    if (spec.post) {
      var choose = el("label", { class: "toggle" });
      usePost = el("input", { type: "checkbox" });
      choose.appendChild(usePost);
      choose.appendChild(document.createTextNode(" Send the text as a POST body instead of in the address"));
      box.appendChild(choose);
    }
    if (spec.upload) {
      if (spec.upload.json) {
        var wrapJSON = el("label", { class: "field" });
        wrapJSON.appendChild(el("span", null, "JSON body"));
        body = el("textarea", { rows: 12, spellcheck: "false" });
        body.value = spec.exampleBody || "";
        wrapJSON.appendChild(body);
        box.appendChild(wrapJSON);
      } else {
        var wrapFile = el("label", { class: "field" });
        wrapFile.appendChild(el("span", null, "File " + (spec.upload.files > 1 ? "(one or more)" : "")));
        file = el("input", { type: "file" });
        if (spec.upload.files > 1) file.multiple = true;
        wrapFile.appendChild(file);
        wrapFile.appendChild(el("small", null, spec.upload.accepts + (spec.upload.sample ? " Leave empty to send the sample file " + spec.upload.sample + "." : "")));
        box.appendChild(wrapFile);
      }
    }

    function values() {
      var out = {};
      spec.params.forEach(function (p) { if (inputs[p.name].value !== "") out[p.name] = inputs[p.name].value; });
      return out;
    }
    function address() {
      var v = values(), path = spec.path, query = new URLSearchParams();
      spec.params.forEach(function (p) {
        if (!(p.name in v)) return;
        if (p.in === "path") path = path.replace("{" + p.name + "}", encodeURIComponent(v[p.name]).replace(/%2C/g, ",")).replace("{" + p.name + "...}", v[p.name]);
        else if (!(usePost && usePost.checked && p.name === "text")) query.set(p.name, v[p.name]);
      });
      path = path.replace(/\{[^}]+\}/g, "");
      var q = query.toString();
      return path + (q ? "?" + q : "");
    }
    // request builds the request; only forReal fetches the sample file to send.
    async function request(forReal) {
      var init = { method: spec.method, headers: {} };
      if (usePost && usePost.checked) {
        init.method = "POST";
        init.headers["Content-Type"] = "text/plain; charset=utf-8";
        init.body = inputs.text ? inputs.text.value : "";
      }
      if (spec.upload) {
        init.method = "POST";
        if (body) {
          init.headers["Content-Type"] = "application/json";
          init.body = body.value;
        } else if (file && file.files.length) {
          if (file.files.length === 1) {
            init.body = file.files[0];
            init.headers["Content-Type"] = file.files[0].type || "application/octet-stream";
          } else {
            var form = new FormData();
            Array.prototype.forEach.call(file.files, function (f, i) { form.append("file" + (i + 1), f); });
            init.body = form;
          }
        } else if (spec.upload.sample && !forReal) {
          init.headers["Content-Type"] = "application/octet-stream";
        } else if (spec.upload.sample) {
          var sample = await fetch("/samples/" + spec.upload.sample);
          init.body = await sample.blob();
          init.headers["Content-Type"] = sample.headers.get("Content-Type") || "application/octet-stream";
        }
      }
      return { url: address(), init: init };
    }
    function fill(v) { for (var k in v) if (inputs[k]) inputs[k].value = v[k]; }
    function onChange(fn) { box.addEventListener("input", fn); box.addEventListener("change", fn); }
    return { request: request, address: address, values: values, fill: fill, onChange: onChange, inputs: inputs, usePost: usePost };
  }

  // credentials adds the visitor's key, or a site token on key routes when asked.
  async function credentials(spec, init, options) {
    var key = window.indiaKey ? window.indiaKey() : "";
    if (key && !(options && options.noKey)) init.headers["X-API-Key"] = key;
    else if (spec.key && options && options.siteToken && window.indiaSiteToken) {
      var token = await window.indiaSiteToken();
      if (token) init.headers["X-Site-Token"] = token;
    }
  }

  // send runs a request and draws the answer into out.
  async function send(spec, req, out, options) {
    out.textContent = "";
    var status = el("p", { class: "status" }, "Sending…");
    out.appendChild(status);
    await credentials(spec, req.init, options);
    var started = performance.now(), response, dropped = false;
    try {
      response = await fetch(req.url, req.init);
      // A key saved in this browser that the server refuses (revoked, rotated, or its
      // account deleted) would break every request on the site: forget it and ask again.
      if (response.status === 401 && req.init.headers["X-API-Key"] && window.indiaKey && req.init.headers["X-API-Key"] === window.indiaKey()) {
        var refused = await response.clone().json().catch(function () { return null; });
        if (refused && refused.error && refused.error.code === "invalid_key") {
          try { localStorage.removeItem("india-api-key"); } catch (e) {}
          delete req.init.headers["X-API-Key"];
          dropped = true;
          await credentials(spec, req.init, options);
          started = performance.now();
          response = await fetch(req.url, req.init);
        }
      }
    } catch (e) {
      status.textContent = "The request did not reach the server: " + e.message;
      status.className = "status error";
      return null;
    }
    var ms = Math.round(performance.now() - started);
    out.textContent = "";
    var meta = el("div", { class: "meta" });
    function fact(label, value) { if (!value) return; var s = el("span"); s.appendChild(el("b", null, label + " ")); s.appendChild(document.createTextNode(value)); meta.appendChild(s); }
    fact("status", response.status + " " + response.statusText);
    fact("time", ms + " ms");
    fact("limit", response.headers.get("RateLimit-Limit") && response.headers.get("RateLimit-Remaining") + " of " + response.headers.get("RateLimit-Limit") + " left, resets in " + response.headers.get("RateLimit-Reset") + " s");
    fact("quota", response.headers.get("X-Quota-Remaining") && response.headers.get("X-Quota-Remaining") + " model requests left today");
    fact("retry after", response.headers.get("Retry-After") && response.headers.get("Retry-After") + " s");
    out.appendChild(meta);
    if (dropped) out.appendChild(el("p", { class: "note" }, "The API key saved in this browser was refused (revoked, rotated or unknown), so it was forgotten and this request was sent without it. Make a new one at /account."));
    var notice = response.headers.get("X-Key-Notice");
    if (notice) out.appendChild(el("p", { class: "note" }, notice));
    var type = response.headers.get("Content-Type") || "", result = { status: response.status, type: type };
    if (type.indexOf("application/json") === 0) {
      var data = await response.json();
      result.json = data;
      var pre = el("pre", { class: "sample", tabindex: "0" }), code = el("code", null, JSON.stringify(data, null, 2));
      pre.appendChild(code);
      out.appendChild(pre);
      if (data && data.error) out.appendChild(el("p", { class: "note" }, explain(response.status, data.error.code)));
    } else {
      var blob = await response.blob(), link = URL.createObjectURL(blob);
      result.blob = blob;
      if (type.indexOf("image/") === 0) out.appendChild(el("img", { src: link, alt: "The answer" }));
      else if (type.indexOf("application/pdf") === 0) out.appendChild(el("iframe", { src: link, title: "The PDF" }));
      else if (type.indexOf("audio/") === 0) { var audio = el("audio", { src: link, controls: "" }); out.appendChild(audio); }
      else out.appendChild(el("pre", null, await blob.text()));
      var save = el("a", { href: link, download: "answer", class: "key key--small" }, "Download");
      out.appendChild(save);
    }
    return result;
  }

  function explain(status, code) {
    var notes = {
      400: "A 400 means a value is missing or in the wrong form; the message says which.",
      401: "A 401 means this endpoint needs a free API key: get one at /signup and send it as X-API-Key.",
      404: "A 404 means the thing looked up does not exist. It is cached for a minute.",
      413: "A 413 means the body or file is larger than this endpoint takes.",
      422: "A 422 means the input was read but cannot be worked on.",
      429: "A 429 means the per-minute allowance is spent; wait the Retry-After seconds. A free key raises it.",
      503: "A 503 means the server is busy or lacks what this needs right now; wait the Retry-After seconds."
    };
    return (notes[status] || "") + (code ? " (code: " + code + ")" : "");
  }

  // samples writes the curl, JavaScript and Python lines for the current request.
  function samples(req, spec) {
    var url = location.origin + req.url, lines = [], method = req.init.method, key = spec.key ? " \\\n  -H 'X-API-Key: YOUR_KEY'" : "";
    if (method === "GET") {
      lines.push(["curl", "curl '" + url + "'" + key]);
      lines.push(["JavaScript", "const response = await fetch(\"" + url + "\"" + (spec.key ? ", { headers: { \"X-API-Key\": KEY } }" : "") + ");\nconst answer = await response.json();"]);
      lines.push(["Python", "import requests\n\nanswer = requests.get(\"" + url + "\"" + (spec.key ? ", headers={\"X-API-Key\": KEY}" : "") + ").json()"]);
    } else {
      var type = req.init.headers["Content-Type"] || "application/octet-stream";
      var data = typeof req.init.body === "string" ? "--data-binary '" + req.init.body.replace(/'/g, "'\\''").slice(0, 400) + "'" : "--data-binary @your-file";
      lines.push(["curl", "curl -X POST '" + url + "' \\\n  -H 'Content-Type: " + type + "'" + key + " \\\n  " + data]);
      lines.push(["JavaScript", "const response = await fetch(\"" + url + "\", {\n  method: \"POST\",\n  headers: { \"Content-Type\": \"" + type + "\"" + (spec.key ? ", \"X-API-Key\": KEY" : "") + " },\n  body: " + (typeof req.init.body === "string" ? JSON.stringify(req.init.body.slice(0, 400)) : "file") + ",\n});"]);
      lines.push(["Python", "import requests\n\nanswer = requests.post(\"" + url + "\", headers={\"Content-Type\": \"" + type + "\"" + (spec.key ? ", \"X-API-Key\": KEY" : "") + "}, data=" + (typeof req.init.body === "string" ? JSON.stringify(req.init.body.slice(0, 400)) : "open(\"your-file\", \"rb\")") + ")"]);
    }
    return lines;
  }

  function drawSamples(box, lines) {
    box.textContent = "";
    lines.forEach(function (pair) {
      var wrap = el("div", { class: "codebox", "data-label": pair[0] }), strip = el("div", { class: "strip" });
      strip.appendChild(el("span", null, pair[0]));
      var copy = el("button", { type: "button", class: "copy" }, "Copy");
      strip.appendChild(copy);
      wrap.appendChild(strip);
      var pre = el("pre"); pre.appendChild(el("code", null, pair[1])); wrap.appendChild(pre);
      box.appendChild(wrap);
    });
  }

  // errorDemo changes the form so the request fails on purpose, to show what that looks like.
  function errorDemos(spec) {
    var demos = [];
    var pathParam = spec.params.filter(function (p) { return p.in === "path"; })[0];
    if (pathParam) demos.push(["Show a 404", function (form) { var v = {}; v[pathParam.name] = "NOT-A-REAL-" + pathParam.name.toUpperCase(); form.fill(v); }]);
    var number = spec.params.filter(function (p) { return p.type === "integer" && p.in === "query"; })[0];
    if (number) demos.push(["Show a 400", function (form) { var v = {}; v[number.name] = "not-a-number"; form.fill(v); }]);
    else { var required = spec.params.filter(function (p) { return p.required && p.in === "query"; })[0]; if (required) demos.push(["Show a 400", function (form) { var v = {}; v[required.name] = ""; form.fill(v); }]); }
    return demos;
  }

  window.indiaTry = { build: build, send: send, samples: samples, drawSamples: drawSamples, errorDemos: errorDemos, el: el, credentials: credentials, textOf: textOf };

  // The panels on an API page.
  document.querySelectorAll("details.tryit[data-spec]").forEach(function (panel) {
    var drawn = false;
    panel.addEventListener("toggle", function () {
      if (!panel.open || drawn) return;
      drawn = true;
      var spec = JSON.parse(panel.getAttribute("data-spec")), inner = panel.querySelector(".inner");
      inner.textContent = "";
      var form = build(inner, spec), actions = el("div", { class: "row" }), run = el("button", { type: "button", class: "key key--primary" }, "Send request");
      actions.appendChild(run);
      var demos = errorDemos(spec);
      demos.forEach(function (demo) {
        var b = el("button", { type: "button", class: "key key--small" }, demo[0]);
        b.addEventListener("click", function () { demo[1](form); run.click(); });
        actions.appendChild(b);
      });
      var reset = el("button", { type: "button", class: "key key--small" }, "Back to the example");
      reset.addEventListener("click", function () { var v = {}; spec.params.forEach(function (p) { v[p.name] = p.value || ""; }); form.fill(v); update(); });
      actions.appendChild(reset);
      inner.appendChild(actions);
      if (spec.key) inner.appendChild(el("p", { class: "note" }, window.indiaKey && window.indiaKey() ? "Sent with the key you made on this browser." : "Model endpoint: it will need a free key. Make one at /account and it is used here too."));
      var codes = el("div", { class: "answer" }), out = el("div", { class: "answer", "aria-live": "polite" });
      inner.appendChild(codes);
      inner.appendChild(out);
      async function update() { drawSamples(codes, samples(await form.request(false), spec)); }
      form.onChange(update);
      update();
      run.addEventListener("click", async function () {
        run.disabled = true;
        try { await send(spec, await form.request(true), out); } finally { run.disabled = false; }
      });
    });
  });
})();
