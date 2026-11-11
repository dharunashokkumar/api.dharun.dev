// The account pages: login and signup by a mailed code, and the key list. Talks to
// /account/api/ on this site only. The login token is kept in this browser's localStorage; nothing else is stored.
(function () {
  "use strict";
  var root = document.getElementById("account-app");
  if (!root) return;
  var view = root.getAttribute("data-view");
  var tokenKey = "india-account-token";
  function token() { try { return localStorage.getItem(tokenKey) || ""; } catch (e) { return ""; } }
  function setToken(value) { try { value ? localStorage.setItem(tokenKey, value) : localStorage.removeItem(tokenKey); } catch (e) {} }
  function $(id) { return document.getElementById(id); }
  function say(id, text, kind) { var el = $(id); if (!el) return; el.textContent = text; el.className = "status" + (kind ? " " + kind : ""); }
  async function call(method, path, body, auth) {
    var headers = { "Content-Type": "application/json" };
    if (auth && token()) headers.Authorization = token();
    var response = await fetch("/account/api/" + path, { method: method, headers: headers, body: body ? JSON.stringify(body) : undefined, cache: "no-store" });
    var data = null;
    try { data = await response.json(); } catch (e) {}
    if (!response.ok) {
      var message = (data && (data.error && data.error.message || data.message)) || ("Something went wrong (" + response.status + ").");
      if (data && data.data) {
        var first = Object.keys(data.data)[0];
        if (first && data.data[first].message) message = first + ": " + data.data[first].message;
      }
      var error = new Error(message); error.status = response.status; throw error;
    }
    return data;
  }
  function form(el) { var out = {}; new FormData(el).forEach(function (v, k) { out[k] = v; }); return out; }
  function busy(el, on) { el.querySelectorAll("button").forEach(function (b) { b.disabled = on; }); }

  if (view === "code") {
    if (token()) { location.href = "/account"; return; }
    var otpId = "", emailForm = $("email-form"), codeForm = $("code-form");
    emailForm.addEventListener("submit", async function (event) {
      event.preventDefault();
      busy(emailForm, true);
      try {
        say("code-status", "Running the browser check…");
        var proof = await window.indiaProof();
        say("code-status", "Sending the code…");
        var answer = await call("POST", "code", { email: form(emailForm).email, altcha: proof });
        otpId = answer.data.otpId;
        emailForm.hidden = true;
        codeForm.hidden = false;
        say("code-status-2", answer.data.message, "good");
        codeForm.elements.code.focus();
      } catch (e) { say("code-status", e.message, "error"); }
      busy(emailForm, false);
    });
    codeForm.addEventListener("submit", async function (event) {
      event.preventDefault();
      busy(codeForm, true);
      try {
        var answer = await call("POST", "collections/users/auth-with-otp", { otpId: otpId, password: form(codeForm).code.trim() });
        setToken(answer.token);
        location.href = "/account";
      } catch (e) {
        say("code-status-2", e.status === 400 ? "That code is not right, or it is older than 10 minutes." : e.message, "error");
      }
      busy(codeForm, false);
    });
    $("code-again").addEventListener("click", function () { codeForm.hidden = true; emailForm.hidden = false; codeForm.reset(); });
  }

  if (view === "account") {
    if (!token()) { location.href = "/login"; return; }
    var me = null;
    function li(key) {
      var item = document.createElement("li"), label = document.createElement("span");
      label.innerHTML = "<b></b> <code></code> <small></small>";
      label.querySelector("b").textContent = key.name;
      label.querySelector("code").textContent = key.hint;
      label.querySelector("small").textContent = key.revoked ? "revoked" : "made " + key.created.slice(0, 10);
      item.appendChild(label);
      if (!key.revoked) {
        var actions = document.createElement("span"); actions.className = "row";
        [["Rotate", "rotate"], ["Revoke", "revoke"]].forEach(function (pair) {
          var button = document.createElement("button");
          button.type = "button"; button.className = "key key--small" + (pair[1] === "revoke" ? " key--danger" : ""); button.textContent = pair[0];
          button.addEventListener("click", async function () {
            if (pair[1] === "revoke" && !confirm("Revoke " + key.name + "? Programs using it stop working within a minute.")) return;
            try {
              var answer = await call("POST", "india/keys/" + key.id + "/" + pair[1], null, true);
              if (answer.key) showKey(answer.key);
              load();
            } catch (e) { say("account-status", e.message, "error"); }
          });
          actions.appendChild(button);
        });
        item.appendChild(actions);
      }
      return item;
    }
    function showKey(value) {
      $("new-key").hidden = false;
      $("new-key-value").textContent = value;
      try { localStorage.setItem("india-api-key", value); } catch (e) {}
    }
    $("copy-key").addEventListener("click", function () { navigator.clipboard && navigator.clipboard.writeText($("new-key-value").textContent); this.textContent = "Copied"; });
    async function load() {
      try {
        me = await call("GET", "india/me", null, true);
      } catch (e) {
        if (e.status === 401) { setToken(""); location.href = "/login"; return; }
        say("account-status", e.message, "error"); return;
      }
      $("account-body").hidden = false;
      say("account-status", me.verified ? "" : "Confirm your email address to make a key: open the link we mailed you.", me.verified ? "" : "error");
      $("account-email").textContent = "Logged in as " + me.email + (me.verified ? "" : " (not confirmed yet)") + ".";
      var list = $("keys"); list.textContent = "";
      me.keys.forEach(function (k) { list.appendChild(li(k)); });
      if (!me.keys.length) { var none = document.createElement("li"); none.textContent = "No keys yet."; list.appendChild(none); }
      try {
        var usage = (await call("GET", "usage", null, true)).data, box = $("usage");
        box.textContent = "";
        usage.keys.forEach(function (k) {
          var part = document.createElement("div"), share = Math.min(100, Math.round(100 * k.modelRequestsToday / Math.max(k.modelQuota, 1)));
          var groups = Object.keys(k.month).sort(function (a, b) { return k.month[b] - k.month[a]; });
          part.innerHTML = "<h3></h3><p class=\"note\"></p><div class=\"meter\"><span></span></div><p class=\"note\"></p>";
          part.querySelector("h3").textContent = k.name;
          part.querySelectorAll(".note")[0].textContent = "Model requests today: " + k.modelRequestsToday + " of " + k.modelQuota + ".";
          part.querySelector(".meter span").style.width = share + "%";
          part.querySelectorAll(".note")[1].textContent = groups.length ? "This month: " + groups.map(function (g) { return g + " " + k.month[g] + (k.today[g] ? " (" + k.today[g] + " today)" : ""); }).join(", ") + "." : "No requests this month yet.";
          box.appendChild(part);
        });
        if (!usage.keys.length) box.innerHTML = '<p class="note">Make a key to see its use here.</p>';
      } catch (e) {}
    }
    $("key-form").addEventListener("submit", async function (event) {
      event.preventDefault();
      try {
        var answer = await call("POST", "india/keys", { name: form(event.target).name }, true);
        showKey(answer.key); event.target.reset(); load();
      } catch (e) { say("account-status", e.message, "error"); }
    });
    $("logout").addEventListener("click", function () { setToken(""); location.href = "/login"; });
    $("logout-all").addEventListener("click", async function () {
      try { await call("POST", "india/logout-all", null, true); } catch (e) {}
      setToken(""); location.href = "/login";
    });
    $("delete-form").addEventListener("submit", async function (event) {
      event.preventDefault();
      try {
        await call("POST", "india/delete-account", form(event.target), true);
        setToken("");
        try { localStorage.removeItem("india-api-key"); } catch (e) {}
        root.innerHTML = '<p class="status good">Your account, keys and usage counts are deleted.</p>';
      } catch (e) { say("account-status", e.message, "error"); }
    });
    load();
  }
})();
