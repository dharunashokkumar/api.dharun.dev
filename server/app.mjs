import { createHash } from "node:crypto";
import { readdirSync, readFileSync } from "node:fs";
import { readFile } from "node:fs/promises";
import path from "node:path";
import { gzipSync } from "node:zlib";

import { BadRequestError, loadDataset } from "./dataset.mjs";
import {
  companyJson,
  companyPageHtml,
  llmsTxt,
  metaJson,
  openApiJson,
  robotsTxt,
  rootIndexJson,
  sitemapXml,
} from "./docs.mjs";
import { createPngRenderer, monogramSvg, parsePngSize, PNG_MAX_SIZE, PNG_MIN_SIZE } from "./images.mjs";
import { createMcpHandler, rpcError } from "./mcp.mjs";
import { createMetrics } from "./metrics.mjs";

const CONTENT_TYPES = {
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".svg": "image/svg+xml; charset=utf-8",
  ".json": "application/json; charset=utf-8",
  ".webmanifest": "application/manifest+json; charset=utf-8",
};
const SVG_TYPE = CONTENT_TYPES[".svg"];
const JSON_TYPE = CONTENT_TYPES[".json"];

const CACHE_LOGO = "public, max-age=86400, stale-while-revalidate=604800";
const CACHE_SHORT = "public, max-age=300";
const CACHE_API = "public, max-age=3600";

// A logo opened directly in a tab is a document, so it gets a policy that allows nothing but its own drawing.
const SVG_CSP = "default-src 'none'; style-src 'unsafe-inline'; img-src data:";
const MCP_BODY_LIMIT = 64 * 1024;

class HttpError extends Error {
  constructor(status, code, message) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

export function createApp(config) {
  const dataset = loadDataset(config.root);
  const metrics = createMetrics({ version: config.version, dataset });
  const png = createPngRenderer({ cacheBytes: config.pngCacheBytes });
  const staticFiles = loadStaticFiles(config.root);
  const limiter = createRateLimiter(config.rateLimitPerMinute);
  const base = config.basePath;
  const derived = new Map();

  const readSvg = (logo) => readFile(path.join(config.root, "logos", logo.file));
  const mcp = createMcpHandler({ dataset, version: config.version, readSvg });

  function urlsFor(req) {
    const origin = config.publicOrigin || `http://${req.headers.host || `localhost:${config.port}`}`;
    return { origin, site: `${origin}${base}`, api: `${origin}${base}/v1` };
  }

  // Documents that depend only on the dataset and the public origin are built once per origin.
  function cachedDocument(name, urls, build) {
    const key = `${name}|${urls.origin}`;
    if (!derived.has(key)) {
      if (derived.size > 64) derived.clear();
      derived.set(key, prepare(Buffer.from(build())));
    }
    return derived.get(key);
  }

  async function route(req, res, ctx, url) {
    const pathname = url.pathname;

    if (req.method === "OPTIONS") {
      ctx.route = "preflight";
      return send(req, res, 204, "", {
        "Access-Control-Allow-Methods": "GET, HEAD, POST, OPTIONS",
        "Access-Control-Allow-Headers": "Content-Type, Accept, Mcp-Session-Id, Mcp-Protocol-Version",
        "Access-Control-Max-Age": "86400",
      });
    }

    const isRead = req.method === "GET" || req.method === "HEAD";
    if (!isRead && pathname !== `${base}/mcp`) throw new HttpError(405, "method_not_allowed", "Use GET.");
    const urls = urlsFor(req);

    if (pathname === "/healthz" || pathname === `${base}/healthz`) {
      ctx.route = "health";
      return sendJson(req, res, 200, { status: "ok", version: config.version, logos: dataset.logos.length }, "no-store");
    }

    if (base && pathname === "/") {
      ctx.route = "docs";
      return sendJson(req, res, 200, rootIndexJson(urls), CACHE_API);
    }
    if (pathname === "/robots.txt") {
      ctx.route = "docs";
      return sendPrepared(req, res, cachedDocument("robots", urls, () => robotsTxt(urls)), "text/plain; charset=utf-8", CACHE_API);
    }
    if (pathname === "/llms.txt" || pathname === `${base}/llms.txt`) {
      ctx.route = "docs";
      return sendPrepared(req, res, cachedDocument("llms", urls, () => llmsTxt(dataset, urls)), "text/plain; charset=utf-8", CACHE_API);
    }
    if (pathname === "/sitemap.xml" || pathname === `${base}/sitemap.xml`) {
      ctx.route = "docs";
      return sendPrepared(req, res, cachedDocument("sitemap", urls, () => sitemapXml(dataset, urls)), "application/xml; charset=utf-8", CACHE_API);
    }

    if (pathname === base) {
      ctx.route = "docs";
      return send(req, res, 308, "", { Location: `${base}/${url.search}` });
    }
    if (!pathname.startsWith(`${base}/`)) throw new HttpError(404, "not_found", "Not found.");

    const rest = pathname.slice(base.length);
    const segments = rest.split("/").slice(1).map(decodeSegment);

    if (rest === "/mcp") {
      ctx.route = "mcp";
      return handleMcp(req, res, urls);
    }

    if (rest === "/v1" || rest === "/v1/") {
      ctx.route = "meta";
      return sendJson(req, res, 200, metaJson(dataset, urls, config.version), CACHE_API);
    }
    if (segments[0] === "v1" && segments[1] === "logo" && segments.length === 3) {
      ctx.route = "logo";
      return handleLogo(req, res, ctx, url, segments[2]);
    }
    if (segments[0] === "v1" && segments[1] === "companies" && segments.length === 3) {
      ctx.route = "company";
      ctx.symbol = segments[2];
      const logo = dataset.resolve(segments[2], url.searchParams.get("exchange"));
      if (!logo) throw new HttpError(404, "not_found", `No company found for "${segments[2]}".`);
      ctx.resolved = logo.id;
      return sendJson(req, res, 200, companyJson(logo, urls), CACHE_API);
    }
    if (rest === "/v1/search") {
      ctx.route = "search";
      return handleSearch(req, res, url, urls);
    }
    if (segments[0] === "v1") throw new HttpError(404, "not_found", "Unknown API endpoint. See /openapi.json.");

    if (segments[0] === "company" && segments.length === 2) {
      ctx.route = "page";
      ctx.symbol = segments[1];
      const logo = dataset.resolve(segments[1], url.searchParams.get("exchange"));
      if (!logo) throw new HttpError(404, "not_found", `No company found for "${segments[1]}".`);
      ctx.resolved = logo.id;
      return sendPrepared(req, res, prepare(Buffer.from(companyPageHtml(logo, urls))), CONTENT_TYPES[".html"], CACHE_API);
    }
    if (rest === "/openapi.json") {
      ctx.route = "docs";
      const document = cachedDocument("openapi", urls, () => JSON.stringify(openApiJson(dataset, urls, config.version), null, 2));
      return sendPrepared(req, res, document, JSON_TYPE, CACHE_API);
    }

    // Plain files: the raw logo set (nse/NSE_TCS.svg) and the website.
    if ((segments[0] === "nse" || segments[0] === "bse") && segments.length === 2) {
      ctx.route = "file";
      const file = `${segments[0]}/${segments[1]}`;
      if (!dataset.files.has(file)) throw new HttpError(404, "not_found", "No such logo file.");
      const body = await readFile(path.join(config.root, "logos", file));
      return sendPrepared(req, res, prepare(body, false), SVG_TYPE, CACHE_LOGO, { "Content-Security-Policy": SVG_CSP });
    }

    const asset = staticFiles.get(rest === "/" ? "/index.html" : rest);
    if (asset) {
      ctx.route = "static";
      return sendPrepared(req, res, asset, asset.type, rest === "/" ? CACHE_SHORT : CACHE_API);
    }

    throw new HttpError(404, "not_found", "Not found.");
  }

  async function handleLogo(req, res, ctx, url, rawSymbol) {
    const query = url.searchParams;
    let symbol = rawSymbol;
    let format = (query.get("format") || "").toLowerCase();

    const suffix = symbol.match(/\.(svg|png)$/i);
    if (suffix) {
      symbol = symbol.slice(0, -suffix[0].length);
      format ||= suffix[1].toLowerCase();
    }
    format ||= "svg";
    if (format !== "svg" && format !== "png") {
      throw new HttpError(400, "bad_format", "format must be svg or png.");
    }

    const size = parsePngSize(query.get("size"));
    if (format === "png" && size === null) {
      throw new HttpError(400, "bad_size", `size must be an integer from ${PNG_MIN_SIZE} to ${PNG_MAX_SIZE}.`);
    }

    const fallback = (query.get("fallback") || "").toLowerCase();
    if (fallback && fallback !== "monogram") {
      throw new HttpError(400, "bad_fallback", "fallback must be monogram.");
    }

    ctx.symbol = symbol;
    const logo = dataset.resolve(symbol, query.get("exchange"));
    if (!logo && !fallback) {
      metrics.observeLogo(format, "not_found");
      throw new HttpError(404, "not_found", `No logo found for "${symbol}". Add ?fallback=monogram for a placeholder.`);
    }

    ctx.resolved = logo?.id;
    metrics.observeLogo(format, logo ? "found" : "fallback");

    const svg = logo ? await readSvg(logo) : Buffer.from(monogramSvg(symbol));
    const headers = logo ? {} : { "X-Logo-Fallback": "monogram" };
    const cache = logo ? CACHE_LOGO : CACHE_SHORT;

    if (format === "svg") {
      return sendPrepared(req, res, prepare(svg, false), SVG_TYPE, cache, { ...headers, "Content-Security-Policy": SVG_CSP });
    }

    const image = await png.render(logo ? logo.id : `monogram:${symbol.toUpperCase()}`, svg, size, { hasText: !logo });
    if (!image) throw new HttpError(501, "png_unavailable", "PNG rendering is not installed on this server. Use format=svg.");
    return sendPrepared(req, res, prepare(image, false), "image/png", cache, headers);
  }

  function handleSearch(req, res, url, urls) {
    const query = url.searchParams;
    const limit = boundedInt(query.get("limit"), 20, 1, 100, "limit");
    const offset = boundedInt(query.get("offset"), 0, 0, 100000, "offset");

    const { total, results } = dataset.search({
      q: query.get("q") || "",
      exchange: query.get("exchange"),
      sector: query.get("sector"),
      index: query.get("index"),
      limit,
      offset,
    });

    return sendJson(req, res, 200, { total, limit, offset, results: results.map((logo) => companyJson(logo, urls)) }, CACHE_API);
  }

  async function handleMcp(req, res, urls) {
    if (req.method !== "POST") {
      res.setHeader("Allow", "POST, OPTIONS");
      throw new HttpError(405, "method_not_allowed", "This MCP endpoint is stateless: POST one JSON-RPC message per request.");
    }

    let message;
    try {
      message = JSON.parse(await readBody(req, MCP_BODY_LIMIT));
    } catch (error) {
      if (error instanceof HttpError) throw error;
      return sendJson(req, res, 400, rpcError(null, -32700, "Parse error."), "no-store");
    }

    const response = await mcp(message, urls);
    if (response === null) return send(req, res, 202, "", { "Cache-Control": "no-store" });
    return sendJson(req, res, 200, response, "no-store");
  }

  function clientKey(req) {
    if (config.trustProxy) {
      const forwarded = req.headers["cf-connecting-ip"] || String(req.headers["x-forwarded-for"] || "").split(",")[0].trim();
      if (forwarded) return forwarded;
    }
    return req.socket.remoteAddress || "unknown";
  }

  async function handler(req, res) {
    const startedAt = process.hrtime.bigint();
    const ctx = { route: "other" };

    res.on("finish", () => {
      const seconds = Number(process.hrtime.bigint() - startedAt) / 1e9;
      metrics.observeRequest(ctx.route, knownMethod(req.method), res.statusCode, seconds);
      if (config.logRequests && ctx.route !== "health") {
        console.log(
          JSON.stringify({
            msg: "request",
            method: req.method,
            route: ctx.route,
            path: String(req.url).slice(0, 300),
            status: res.statusCode,
            ms: Math.round(seconds * 1e5) / 100,
            symbol: clip(ctx.symbol),
            resolved: ctx.resolved,
            country: req.headers["cf-ipcountry"],
            ua: clip(req.headers["user-agent"]),
            referer: clip(req.headers.referer),
          }),
        );
      }
    });

    setCommonHeaders(res);

    try {
      if (!String(req.url).startsWith("/")) throw new HttpError(400, "bad_request", "Bad request target.");
      const url = new URL(`http://internal${req.url}`);

      // Classify before limiting so only the routes that do real work are counted.
      const rest = url.pathname.startsWith(`${base}/`) ? url.pathname.slice(base.length) : "";
      const limited = /^\/(v1\/(logo|companies|search)|mcp|company)(\/|$)/.exec(rest);
      if (limited && req.method !== "OPTIONS" && !limiter.allow(clientKey(req))) {
        ctx.route = "rate_limited";
        metrics.observeRateLimited();
        res.setHeader("Retry-After", "60");
        throw new HttpError(429, "rate_limited", `Rate limit is ${config.rateLimitPerMinute} requests per minute.`);
      }

      await route(req, res, ctx, url);
    } catch (error) {
      if (res.headersSent) {
        res.destroy();
        return;
      }
      if (error instanceof HttpError) {
        return sendJson(req, res, error.status, { error: { code: error.code, message: error.message } }, error.status === 404 ? "public, max-age=60" : "no-store");
      }
      if (error instanceof BadRequestError) {
        return sendJson(req, res, 400, { error: { code: "bad_request", message: error.message } }, "no-store");
      }
      console.error(JSON.stringify({ msg: "error", path: String(req.url).slice(0, 300), error: String(error?.stack || error) }));
      return sendJson(req, res, 500, { error: { code: "internal_error", message: "Internal error." } }, "no-store");
    }
  }

  function metricsHandler(req, res) {
    if (req.url === "/metrics") {
      const body = metrics.render();
      res.writeHead(200, { "Content-Type": "text/plain; version=0.0.4; charset=utf-8", "Content-Length": Buffer.byteLength(body) });
      return res.end(body);
    }
    res.writeHead(404, { "Content-Type": "text/plain" });
    res.end("Not found\n");
  }

  return { handler, metricsHandler, dataset, png, close: () => limiter.close() };
}

function loadStaticFiles(root) {
  const files = new Map();
  const add = (urlPath, filePath) => {
    const type = CONTENT_TYPES[path.extname(filePath)];
    if (type) files.set(urlPath, { ...prepare(readFileSync(filePath)), type });
  };

  const webDir = path.join(root, "web");
  for (const name of readdirSync(webDir)) add(`/${name}`, path.join(webDir, name));
  add("/data/logos.json", path.join(root, "data", "logos.json"));
  return files;
}

// Body plus its validators, with a gzip copy when that is worth sending.
function prepare(raw, compress = true) {
  const etag = `"${createHash("sha1").update(raw).digest("base64url").slice(0, 20)}"`;
  const gzip = compress && raw.length > 1024 ? gzipSync(raw) : null;
  return { raw, gzip, etag };
}

function sendPrepared(req, res, body, type, cacheControl, extraHeaders = {}) {
  const headers = { "Content-Type": type, "Cache-Control": cacheControl, ETag: body.etag, ...extraHeaders };
  if (body.gzip) headers.Vary = "Accept-Encoding";

  if (req.headers["if-none-match"] === body.etag) return send(req, res, 304, "", headers);

  if (body.gzip && /\bgzip\b/.test(String(req.headers["accept-encoding"] || ""))) {
    return send(req, res, 200, body.gzip, { ...headers, "Content-Encoding": "gzip" });
  }
  return send(req, res, 200, body.raw, headers);
}

function sendJson(req, res, status, value, cacheControl) {
  const body = prepare(Buffer.from(`${JSON.stringify(value)}\n`));
  const headers = { "Content-Type": JSON_TYPE, "Cache-Control": cacheControl };
  if (status !== 200) return send(req, res, status, body.raw, headers);
  return sendPrepared(req, res, body, JSON_TYPE, cacheControl);
}

function send(req, res, status, body, headers = {}) {
  const buffer = typeof body === "string" ? Buffer.from(body) : body;
  const hasBody = status !== 204 && status !== 304;
  res.writeHead(status, hasBody ? { ...headers, "Content-Length": buffer.length } : headers);
  res.end(hasBody && req.method !== "HEAD" ? buffer : undefined);
}

function setCommonHeaders(res) {
  res.setHeader("Access-Control-Allow-Origin", "*");
  res.setHeader("Access-Control-Expose-Headers", "ETag, X-Logo-Fallback");
  res.setHeader("Cross-Origin-Resource-Policy", "cross-origin");
  res.setHeader("X-Content-Type-Options", "nosniff");
  res.setHeader("Referrer-Policy", "strict-origin-when-cross-origin");
}

function decodeSegment(segment) {
  try {
    return decodeURIComponent(segment);
  } catch {
    throw new HttpError(400, "bad_request", "Malformed percent-encoding in the URL.");
  }
}

function boundedInt(value, fallback, min, max, name) {
  if (value === null || value === "") return fallback;
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed < min || parsed > max) {
    throw new HttpError(400, "bad_request", `${name} must be an integer from ${min} to ${max}.`);
  }
  return parsed;
}

function readBody(req, limit) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    let size = 0;
    // An oversized body is drained, not stored, so the client still receives the 413.
    // The server's request timeout bounds how long that can take.
    req.on("data", (chunk) => {
      size += chunk.length;
      if (size <= limit) chunks.push(chunk);
    });
    req.on("end", () => {
      if (size > limit) reject(new HttpError(413, "payload_too_large", "Request body is too large."));
      else resolve(Buffer.concat(chunks).toString("utf8"));
    });
    req.on("error", reject);
  });
}

// Fixed one-minute windows per client. Coarse, but it costs one Map entry per active client.
function createRateLimiter(perMinute) {
  if (!perMinute) return { allow: () => true, close() {} };

  let counts = new Map();
  const timer = setInterval(() => {
    counts = new Map();
  }, 60_000);
  timer.unref();

  return {
    allow(key) {
      const count = (counts.get(key) || 0) + 1;
      counts.set(key, count);
      return count <= perMinute;
    },
    close() {
      clearInterval(timer);
    },
  };
}

function knownMethod(method) {
  return ["GET", "HEAD", "POST", "OPTIONS"].includes(method) ? method : "OTHER";
}

function clip(value) {
  return value ? String(value).slice(0, 200) : undefined;
}
