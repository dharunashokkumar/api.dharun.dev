// The terminal page: sign in with GitHub (device flow), pass Cloudflare's browser check,
// start a session and carry its terminal over a WebSocket. MIT-0.
(() => {
  "use strict";
  const siteKey = document.currentScript.dataset.sitekey;
  const keyName = "india-api-sandbox-token";
  const $ = (id) => document.getElementById(id);
  const say = (text) => { $("message").textContent = text || ""; };
  let token = null;
  try { token = localStorage.getItem(keyName); } catch (e) { /* private window */ }
  let turnstileToken = "";
  let widget = null;

  async function api(method, path, body) {
    const headers = { "Content-Type": "application/json" };
    if (token) headers.Authorization = "Bearer " + token;
    const response = await fetch(path, { method, headers, body: body === undefined ? undefined : JSON.stringify(body) });
    const answer = await response.json().catch(() => ({}));
    if (!response.ok) {
      const error = new Error((answer.error && answer.error.message) || "HTTP " + response.status);
      error.status = response.status;
      throw error;
    }
    return answer;
  }

  function show(id) {
    for (const section of ["signin", "start", "session"]) $(section).hidden = section !== id;
  }

  function signOut() {
    token = null;
    try { localStorage.removeItem(keyName); } catch (e) { /* nothing kept */ }
    $("who").textContent = "";
    $("signout").hidden = true;
    show("signin");
  }

  async function refreshAccount() {
    try {
      const { data } = await api("GET", "/v1/sandbox/account");
      $("who").textContent = data.login;
      $("signout").hidden = false;
      $("quota").textContent = `Left today: ${data.left.runs} sessions and commands, ${Math.floor(data.left.cpuMs / 60000)} CPU-minutes.`;
      return true;
    } catch (error) {
      if (error.status === 401) signOut();
      else say(error.message);
      return false;
    }
  }

  $("login").addEventListener("click", async () => {
    say("");
    $("login").disabled = true;
    try {
      const { data } = await api("POST", "/v1/sandbox/login");
      $("usercode").textContent = data.userCode;
      $("verify").href = data.verificationUri;
      $("code").hidden = false;
      let wait = data.interval * 1000;
      const deadline = Date.now() + data.expiresIn * 1000;
      while (Date.now() < deadline) {
        await new Promise((done) => setTimeout(done, wait));
        const poll = await api("POST", "/v1/sandbox/login/poll", { deviceCode: data.deviceCode });
        if (poll.data.status === "slow_down") wait += 5000;
        if (poll.data.status === "ok") {
          token = poll.data.token;
          try { localStorage.setItem(keyName, token); } catch (e) { /* kept for this page only */ }
          $("code").hidden = true;
          await ready();
          return;
        }
      }
      say("The code expired. Try again.");
    } catch (error) {
      say(error.message);
    } finally {
      $("login").disabled = false;
    }
  });

  function renderTurnstile() {
    if (!window.turnstile) return setTimeout(renderTurnstile, 200);
    if (widget !== null) return window.turnstile.reset(widget);
    widget = window.turnstile.render("#turnstile", {
      sitekey: siteKey,
      callback: (value) => { turnstileToken = value; $("go").disabled = false; },
      "expired-callback": () => { turnstileToken = ""; $("go").disabled = true; },
    });
  }

  async function ready() {
    if (!(await refreshAccount())) return;
    show("start");
    $("go").disabled = true;
    renderTurnstile();
  }

  $("image").addEventListener("change", () => {
    $("network").disabled = $("image").value !== "code";
    if ($("network").disabled) $("network").checked = false;
  });

  $("go").addEventListener("click", async () => {
    say("");
    $("go").disabled = true;
    try {
      const { data } = await api("POST", "/v1/sandbox/sessions", {
        image: $("image").value, network: $("network").checked, terminal: true, turnstile: turnstileToken,
      });
      openTerminal(data);
    } catch (error) {
      say(error.message);
      renderTurnstile();
    }
  });

  function openTerminal(session) {
    show("session");
    $("what").textContent = $("image").selectedOptions[0].textContent.replace(/ \(.*\)$/, "");
    const term = new Terminal({ cursorBlink: true, fontSize: 15, scrollback: 5000, theme: { background: "#111111" } });
    const fit = new FitAddon.FitAddon();
    term.loadAddon(fit);
    term.open($("term"));
    fit.fit();
    const address = location.origin.replace(/^http/, "ws") + session.links.tty.replace(/^wss?:\/\/[^/]+/, "") +
      `?token=${encodeURIComponent(token)}&cols=${term.cols}&rows=${term.rows}`;
    const socket = new WebSocket(address);
    socket.binaryType = "arraybuffer";
    const encoder = new TextEncoder();
    socket.onopen = () => term.focus();
    socket.onmessage = (event) => term.write(new Uint8Array(event.data));
    term.onData((data) => { if (socket.readyState === 1) socket.send(encoder.encode(data)); });
    term.onResize(({ cols, rows }) => { if (socket.readyState === 1) socket.send(JSON.stringify({ cols, rows })); });
    const onResize = () => fit.fit();
    window.addEventListener("resize", onResize);
    const ends = Date.parse(session.expiresAt);
    const timer = setInterval(() => {
      const left = Math.max(0, Math.round((ends - Date.now()) / 1000));
      $("left").textContent = `${Math.floor(left / 60)}:${String(left % 60).padStart(2, "0")} left`;
    }, 1000);
    const finish = (text) => {
      clearInterval(timer);
      window.removeEventListener("resize", onResize);
      term.write(`\r\n\x1b[2m${text}\x1b[0m\r\n`);
      $("stop").textContent = "New session";
      $("stop").onclick = () => { term.dispose(); ready(); };
    };
    socket.onclose = () => finish("The session ended.");
    $("stop").textContent = "Stop";
    $("stop").onclick = async () => {
      try { await api("POST", `/v1/sandbox/sessions/${session.id}/stop`); } catch (e) { /* ended already */ }
      socket.close();
    };
  }

  $("signout").addEventListener("click", async (event) => {
    event.preventDefault();
    try { await api("POST", "/v1/sandbox/logout"); } catch (e) { /* the token is gone either way */ }
    signOut();
  });

  if (!siteKey || siteKey.startsWith("{{")) {
    say("The terminal is not set up on this server yet.");
    $("signin").hidden = true;
  } else if (token) {
    ready();
  } else {
    show("signin");
  }
})();
