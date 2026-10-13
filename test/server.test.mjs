import assert from "node:assert/strict";
import http from "node:http";
import { after, before, test } from "node:test";

import { createApp } from "../server/app.mjs";
import { loadConfig } from "../server/config.mjs";

const ORIGIN = "https://api.example.test";

let app;
let server;
let base;

function start(env = {}) {
  const instance = createApp(loadConfig({ LOG_REQUESTS: "0", PUBLIC_ORIGIN: ORIGIN, ...env }));
  const listener = http.createServer(instance.handler);
  return new Promise((resolve) => {
    listener.listen(0, "127.0.0.1", () => {
      resolve({ instance, listener, url: `http://127.0.0.1:${listener.address().port}` });
    });
  });
}

function stop({ instance, listener }) {
  instance.close();
  return new Promise((resolve) => listener.close(resolve));
}

before(async () => {
  const started = await start({ RATE_LIMIT_PER_MIN: "0" });
  app = started.instance;
  server = started;
  base = started.url;
});

after(() => stop(server));

const get = (path, init) => fetch(`${base}${path}`, { redirect: "manual", ...init });

test("logo endpoint returns the SVG with caching and CORS headers", async () => {
  const response = await get("/logos/v1/logo/TCS");
  assert.equal(response.status, 200);
  assert.match(response.headers.get("content-type"), /^image\/svg\+xml/);
  assert.equal(response.headers.get("access-control-allow-origin"), "*");
  assert.match(response.headers.get("cache-control"), /max-age=86400/);
  assert.match(response.headers.get("content-security-policy"), /default-src 'none'/);
  assert.equal(response.headers.get("x-content-type-options"), "nosniff");

  const svg = await response.text();
  assert.ok(svg.startsWith("<svg"));
  assert.match(svg, /viewBox="0 0 18 18"/);

  const revalidated = await get("/logos/v1/logo/TCS", { headers: { "If-None-Match": response.headers.get("etag") } });
  assert.equal(revalidated.status, 304);
});

test("every accepted symbol form returns the same logo", async () => {
  const expected = await (await get("/logos/v1/logo/TCS")).text();
  for (const symbol of ["tcs", "NSE:TCS", "NSE_TCS", "TCS.NS", "INE467B01029", "TCS.svg", "TCS?exchange=nse"]) {
    const response = await get(`/logos/v1/logo/${symbol}`);
    assert.equal(response.status, 200, symbol);
    assert.equal(await response.text(), expected, symbol);
  }
  assert.equal((await get("/logos/v1/logo/M%26M")).status, 200);
  assert.equal((await get("/logos/v1/logo/BAJAJ-AUTO")).status, 200);
});

test("unknown symbols are a JSON 404, or a monogram when asked", async () => {
  const missing = await get("/logos/v1/logo/NOTLISTED");
  assert.equal(missing.status, 404);
  assert.equal((await missing.json()).error.code, "not_found");

  const fallback = await get("/logos/v1/logo/NOTLISTED?fallback=monogram");
  assert.equal(fallback.status, 200);
  assert.equal(fallback.headers.get("x-logo-fallback"), "monogram");
  assert.match(await fallback.text(), />NO<\/text>/);

  const hostile = await get(`/logos/v1/logo/${encodeURIComponent('"><script>alert(1)</script>')}?fallback=monogram`);
  assert.equal(hostile.status, 200);
  assert.doesNotMatch(await hostile.text(), /<script|alert/);
});

test("bad parameters are rejected with 400", async () => {
  for (const path of [
    "/logos/v1/logo/TCS?format=gif",
    "/logos/v1/logo/TCS?format=png&size=8",
    "/logos/v1/logo/TCS?format=png&size=4096",
    "/logos/v1/logo/TCS?format=png&size=abc",
    "/logos/v1/logo/TCS?fallback=blank",
    "/logos/v1/logo/TCS?exchange=nyse",
    "/logos/v1/logo/%E0%A4%A",
    "/logos/v1/search?limit=0",
    "/logos/v1/search?limit=101",
    "/logos/v1/search?offset=-1",
  ]) {
    const response = await get(path);
    assert.equal(response.status, 400, path);
    assert.ok((await response.json()).error.message, path);
  }
});

test("PNG output is a real PNG of the requested size", async (t) => {
  if (!(await app.png.available())) return t.skip("@resvg/resvg-js is not installed");

  for (const [path, size] of [
    ["/logos/v1/logo/TCS?format=png", 128],
    ["/logos/v1/logo/TCS.png?size=64", 64],
    ["/logos/v1/logo/NOTLISTED.png?size=32&fallback=monogram", 32],
  ]) {
    const response = await get(path);
    assert.equal(response.status, 200, path);
    assert.equal(response.headers.get("content-type"), "image/png");
    const png = Buffer.from(await response.arrayBuffer());
    assert.equal(png.subarray(0, 8).toString("hex"), "89504e470d0a1a0a");
    assert.equal(png.readUInt32BE(16), size, `${path} width`);
    assert.equal(png.readUInt32BE(20), size, `${path} height`);
  }
});

test("company endpoint returns metadata with absolute links", async () => {
  const response = await get("/logos/v1/companies/reliance");
  assert.equal(response.status, 200);
  const company = await response.json();
  assert.equal(company.id, "NSE:RELIANCE");
  assert.ok(company.company && company.sector && company.isin);
  assert.equal(company.logo.svg, `${ORIGIN}/logos/v1/logo/NSE:RELIANCE`);
  assert.equal(company.links.self, `${ORIGIN}/logos/v1/companies/NSE:RELIANCE`);
  assert.equal(company.file, undefined);

  const ampersand = await (await get("/logos/v1/companies/M%26M")).json();
  assert.equal(ampersand.logo.svg, `${ORIGIN}/logos/v1/logo/NSE:M%26M`);

  assert.equal((await get("/logos/v1/companies/NOTLISTED")).status, 404);
});

test("links the API hands out resolve back to the same company", async () => {
  for (const symbol of ["TCS", "M%26M", "BIRET.RR", "BAJAJ_AUTO"]) {
    const company = await (await get(`/logos/v1/companies/${symbol}`)).json();
    for (const link of [company.logo.svg, company.logo.png, company.links.self, company.links.page]) {
      const response = await get(link.slice(ORIGIN.length));
      assert.equal(response.status, link.includes("format=png") && !(await app.png.available()) ? 501 : 200, link);
    }
  }
});

test("search returns ranked, paginated results", async () => {
  const result = await (await get("/logos/v1/search?q=hdfc+bank&limit=3")).json();
  assert.equal(result.limit, 3);
  assert.equal(result.results.length, 3);
  assert.equal(result.results[0].id, "NSE:HDFCBANK");
  assert.ok(result.total >= 1);

  const nifty = await (await get("/logos/v1/search?index=Nifty%2050&limit=100")).json();
  assert.ok(nifty.results.length >= 45);
});

test("dataset summary and machine-readable docs are served", async () => {
  const meta = await (await get("/logos/v1")).json();
  assert.equal(meta.counts.total, app.dataset.logos.length);
  assert.ok(meta.sectors.length > 5);

  const openapi = await (await get("/logos/openapi.json")).json();
  assert.equal(openapi.openapi, "3.1.0");
  assert.equal(openapi.servers[0].url, `${ORIGIN}/logos/v1`);
  assert.deepEqual(Object.keys(openapi.paths), ["/logo/{symbol}", "/companies/{symbol}", "/search"]);

  for (const path of ["/llms.txt", "/logos/llms.txt"]) {
    const llms = await (await get(path)).text();
    assert.match(llms, /^# Indian Listed Company Logos API/);
    assert.ok(llms.includes(`${ORIGIN}/logos/v1/logo/{symbol}`));
  }

  const robots = await (await get("/robots.txt")).text();
  assert.match(robots, /User-agent: ClaudeBot/);
  assert.ok(robots.includes(`Sitemap: ${ORIGIN}/logos/sitemap.xml`));

  const sitemap = await (await get("/logos/sitemap.xml")).text();
  assert.equal(sitemap.match(/<url>/g).length, app.dataset.logos.length + 1);
  assert.ok(sitemap.includes(`${ORIGIN}/logos/company/NSE:M%26amp;M`) || sitemap.includes(`${ORIGIN}/logos/company/NSE:M%26M`));

  const root = await (await get("/")).json();
  assert.equal(root.apis[0].url, `${ORIGIN}/logos/`);
});

test("company pages are crawlable HTML with escaped content", async () => {
  const response = await get("/logos/company/NSE:TCS");
  assert.equal(response.status, 200);
  assert.match(response.headers.get("content-type"), /^text\/html/);
  const html = await response.text();
  assert.match(html, /<title>Tata Consultancy Services Limited logo \(NSE: TCS\)/);
  assert.ok(html.includes(`<link rel="canonical" href="${ORIGIN}/logos/company/NSE:TCS" />`));
  assert.ok(html.includes(`src="${ORIGIN}/logos/v1/logo/NSE:TCS"`));

  const ampersand = await (await get("/logos/company/M%26M")).text();
  assert.ok(ampersand.includes("<h1>Mahindra &amp; Mahindra Ltd. logo</h1>"));
  assert.ok(ampersand.includes("<dd>M&amp;M</dd>"));

  assert.equal((await get("/logos/company/NOTLISTED")).status, 404);
});

test("the website and raw logo files are served under the base path", async () => {
  const redirect = await get("/logos?q=tcs");
  assert.equal(redirect.status, 308);
  assert.equal(redirect.headers.get("location"), "/logos/?q=tcs");

  const page = await get("/logos/");
  assert.equal(page.status, 200);
  const html = await page.text();
  assert.match(html, /<h1>Indian Listed Company Logos<\/h1>/);
  assert.doesNotMatch(html, /github\.io|analytics\.js/);

  for (const [path, type] of [
    ["/logos/app.js", /javascript/],
    ["/logos/styles.css", /css/],
    ["/logos/favicon.svg", /svg/],
    ["/logos/data/logos.json", /json/],
    ["/logos/nse/NSE_TCS.svg", /svg/],
  ]) {
    const response = await get(path);
    assert.equal(response.status, 200, path);
    assert.match(response.headers.get("content-type"), type, path);
  }

  const gzipped = await get("/logos/data/logos.json", { headers: { "Accept-Encoding": "gzip" } });
  assert.equal(gzipped.headers.get("content-encoding"), "gzip");
  assert.equal((await gzipped.json()).logos.length, app.dataset.logos.length);
});

test("nothing outside the published files can be read", async () => {
  for (const path of [
    "/logos/nse/..%2F..%2Fpackage.json",
    "/logos/nse/%2e%2e%2f%2e%2e%2fpackage.json",
    "/logos/..%2Fpackage.json",
    "/logos/package.json",
    "/logos/data/logo_metadata.full.json",
    "/logos/server/app.mjs",
    "/package.json",
    "/metrics",
    "/logos/metrics",
  ]) {
    const response = await get(path);
    assert.equal(response.status, 404, path);
  }
});

test("HEAD, OPTIONS and unsupported methods behave", async () => {
  const head = await get("/logos/v1/companies/TCS", { method: "HEAD" });
  assert.equal(head.status, 200);
  assert.equal(await head.text(), "");
  assert.ok(Number(head.headers.get("content-length")) > 100);

  const preflight = await get("/logos/mcp", { method: "OPTIONS" });
  assert.equal(preflight.status, 204);
  assert.match(preflight.headers.get("access-control-allow-methods"), /POST/);

  assert.equal((await get("/logos/v1/logo/TCS", { method: "POST" })).status, 405);
  assert.equal((await get("/logos/v1/logo/TCS", { method: "DELETE" })).status, 405);
});

test("health check reports the version", async () => {
  const health = await (await get("/healthz")).json();
  assert.equal(health.status, "ok");
  assert.equal(health.logos, app.dataset.logos.length);
  assert.match(health.version, /^\d+\.\d+\.\d+/);
  assert.equal((await get("/logos/healthz")).status, 200);
});

test("MCP endpoint speaks stateless JSON-RPC", async () => {
  const rpc = async (body) => {
    const response = await get("/logos/mcp", {
      method: "POST",
      headers: { "Content-Type": "application/json", Accept: "application/json, text/event-stream" },
      body: typeof body === "string" ? body : JSON.stringify(body),
    });
    return { status: response.status, body: response.status === 202 ? null : await response.json() };
  };

  const init = await rpc({ jsonrpc: "2.0", id: 1, method: "initialize", params: { protocolVersion: "2025-06-18", capabilities: {}, clientInfo: { name: "test", version: "0" } } });
  assert.equal(init.status, 200);
  assert.equal(init.body.result.protocolVersion, "2025-06-18");
  assert.ok(init.body.result.capabilities.tools);

  const future = await rpc({ jsonrpc: "2.0", id: 2, method: "initialize", params: { protocolVersion: "2099-01-01" } });
  assert.equal(future.body.result.protocolVersion, "2025-06-18");

  assert.equal((await rpc({ jsonrpc: "2.0", method: "notifications/initialized" })).status, 202);

  const tools = await rpc({ jsonrpc: "2.0", id: 3, method: "tools/list" });
  assert.deepEqual(tools.body.result.tools.map((tool) => tool.name), ["search_logos", "get_logo"]);

  const found = await rpc({ jsonrpc: "2.0", id: 4, method: "tools/call", params: { name: "get_logo", arguments: { symbol: "tcs.ns", include_svg: true } } });
  assert.equal(found.body.result.structuredContent.id, "NSE:TCS");
  assert.ok(found.body.result.structuredContent.svg.startsWith("<svg"));
  assert.equal(JSON.parse(found.body.result.content[0].text).id, "NSE:TCS");

  const searched = await rpc({ jsonrpc: "2.0", id: 5, method: "tools/call", params: { name: "search_logos", arguments: { query: "infosys", limit: 2 } } });
  assert.equal(searched.body.result.structuredContent.results[0].id, "NSE:INFY");

  const missing = await rpc({ jsonrpc: "2.0", id: 6, method: "tools/call", params: { name: "get_logo", arguments: { symbol: "NOTLISTED" } } });
  assert.equal(missing.body.result.isError, true);

  assert.equal((await rpc({ jsonrpc: "2.0", id: 7, method: "tools/call", params: { name: "nope" } })).body.error.code, -32602);
  assert.equal((await rpc({ jsonrpc: "2.0", id: 8, method: "resources/list" })).body.error.code, -32601);

  const garbage = await rpc("not json");
  assert.equal(garbage.status, 400);
  assert.equal(garbage.body.error.code, -32700);

  assert.equal((await rpc("x".repeat(70_000))).status, 413);
  assert.equal((await get("/logos/mcp")).status, 405);
});

test("metrics count requests without leaking symbols into labels", async () => {
  await get("/logos/v1/logo/TCS");
  await get("/logos/v1/logo/NOTLISTED");

  const chunks = [];
  const response = {
    writeHead(status) {
      this.status = status;
    },
    end(body) {
      chunks.push(body);
    },
  };
  app.metricsHandler({ url: "/metrics" }, response);
  const text = chunks.join("");

  assert.equal(response.status, 200);
  assert.match(text, /logos_http_requests_total\{route="logo",method="GET",status="200"\} \d+/);
  assert.match(text, /logos_logo_requests_total\{format="svg",result="not_found"\} \d+/);
  assert.match(text, /logos_http_request_duration_seconds_bucket\{route="logo",le="\+Inf"\} \d+/);
  assert.match(text, /logos_dataset_logos\{exchange="NSE"\} \d+/);
  assert.doesNotMatch(text, /TCS|NOTLISTED/);
});

test("rate limiter rejects a client that exceeds its budget", async () => {
  const limited = await start({ RATE_LIMIT_PER_MIN: "3" });
  try {
    const statuses = [];
    for (let i = 0; i < 5; i += 1) statuses.push((await fetch(`${limited.url}/logos/v1/logo/TCS`)).status);
    assert.deepEqual(statuses, [200, 200, 200, 429, 429]);

    const blocked = await fetch(`${limited.url}/logos/v1/logo/TCS`);
    assert.equal(blocked.headers.get("retry-after"), "60");
    // The website and health check stay reachable for a throttled client.
    assert.equal((await fetch(`${limited.url}/logos/`)).status, 200);
    assert.equal((await fetch(`${limited.url}/healthz`)).status, 200);
  } finally {
    await stop(limited);
  }
});

test("base path is configurable, including mounting at the root", async () => {
  const rooted = await start({ BASE_PATH: "/", RATE_LIMIT_PER_MIN: "0" });
  try {
    assert.equal((await fetch(`${rooted.url}/v1/logo/TCS`)).status, 200);
    assert.equal((await fetch(`${rooted.url}/`)).headers.get("content-type").startsWith("text/html"), true);
    const company = await (await fetch(`${rooted.url}/v1/companies/TCS`)).json();
    assert.equal(company.logo.svg, `${ORIGIN}/v1/logo/NSE:TCS`);
  } finally {
    await stop(rooted);
  }
});
