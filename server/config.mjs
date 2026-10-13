import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

export function loadConfig(env = process.env) {
  const root = env.LOGOS_ROOT ? path.resolve(env.LOGOS_ROOT) : repoRoot;

  return {
    root,
    version: readVersion(root),
    host: env.HOST || "127.0.0.1",
    port: toInt(env.PORT, 8080),
    // Prometheus scrapes this listener directly; it is never reachable through the public route.
    metricsHost: env.METRICS_HOST || "127.0.0.1",
    metricsPort: toInt(env.METRICS_PORT, 9464),
    // The service is mounted under a path so other APIs can share the hostname.
    basePath: normalizeBasePath(env.BASE_PATH ?? "/logos"),
    // Absolute origin used in docs, sitemaps and JSON links. Falls back to the request Host.
    publicOrigin: (env.PUBLIC_ORIGIN || "").replace(/\/+$/, ""),
    // Only enable behind a proxy that sets CF-Connecting-IP / X-Forwarded-For itself.
    trustProxy: toBool(env.TRUST_PROXY, false),
    rateLimitPerMinute: toInt(env.RATE_LIMIT_PER_MIN, 600),
    logRequests: toBool(env.LOG_REQUESTS, true),
    pngCacheBytes: toInt(env.PNG_CACHE_MB, 8) * 1024 * 1024,
  };
}

function readVersion(root) {
  try {
    return readFileSync(path.join(root, "VERSION"), "utf8").trim();
  } catch {
    return JSON.parse(readFileSync(path.join(root, "package.json"), "utf8")).version;
  }
}

function normalizeBasePath(value) {
  const trimmed = String(value).trim().replace(/^\/+|\/+$/g, "");
  return trimmed ? `/${trimmed}` : "";
}

function toInt(value, fallback) {
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : fallback;
}

function toBool(value, fallback) {
  if (value === undefined || value === "") return fallback;
  return /^(1|true|yes|on)$/i.test(String(value));
}
