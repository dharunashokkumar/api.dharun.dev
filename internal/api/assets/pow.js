// The browser check for signup and the tool pages: a small proof of work in the ALTCHA
// scheme, solved here with the browser's own SHA-256. Nothing is loaded from elsewhere.
// window.indiaProof() resolves to the base64 payload the server checks.
(function () {
  "use strict";
  function hex(buffer) {
    var bytes = new Uint8Array(buffer), out = "";
    for (var i = 0; i < bytes.length; i++) out += (bytes[i] < 16 ? "0" : "") + bytes[i].toString(16);
    return out;
  }
  async function solve(challenge) {
    var encoder = new TextEncoder(), batch = 500;
    for (var start = 0; start <= challenge.maxnumber; start += batch) {
      var tries = [];
      for (var n = start; n < start + batch && n <= challenge.maxnumber; n++) {
        tries.push(crypto.subtle.digest("SHA-256", encoder.encode(challenge.salt + n)));
      }
      var sums = await Promise.all(tries);
      for (var i = 0; i < sums.length; i++) {
        if (hex(sums[i]) === challenge.challenge) return start + i;
      }
    }
    throw new Error("no solution");
  }
  window.indiaProof = async function () {
    var response = await fetch("/account/api/challenge", { cache: "no-store" });
    if (!response.ok) throw new Error("The check could not start (" + response.status + ").");
    var challenge = await response.json();
    var number = await solve(challenge);
    return btoa(JSON.stringify({ algorithm: challenge.algorithm, challenge: challenge.challenge, number: number, salt: challenge.salt, signature: challenge.signature }));
  };
  // A site token for the tool pages, kept for its hour in this tab only.
  window.indiaSiteToken = async function () {
    try {
      var kept = JSON.parse(sessionStorage.getItem("india-site-token") || "null");
      if (kept && Date.parse(kept.expires) - Date.now() > 60000) return kept.token;
    } catch (e) {}
    var proof = await window.indiaProof();
    var response = await fetch("/account/api/site-token", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ altcha: proof }) });
    if (!response.ok) return "";
    var data = (await response.json()).data;
    try { sessionStorage.setItem("india-site-token", JSON.stringify(data)); } catch (e) {}
    return data.token;
  };
  // The API key a logged-in visitor chose to use on this site's pages, if any.
  window.indiaKey = function () {
    try { return localStorage.getItem("india-api-key") || ""; } catch (e) { return ""; }
  };
})();
