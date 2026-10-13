import { createHash } from "node:crypto";

export const PNG_MIN_SIZE = 16;
export const PNG_MAX_SIZE = 1024;
export const PNG_DEFAULT_SIZE = 128;

const MONOGRAM_COLORS = ["#2248ff", "#0f766e", "#b45309", "#be123c", "#6d28d9", "#15803d", "#0369a1", "#a21caf"];

// Stand-in tile for symbols we have no logo for. Only [A-Z0-9] ever reaches the markup.
export function monogramSvg(symbol) {
  const letters = String(symbol).toUpperCase().replace(/[^A-Z0-9]/g, "").slice(0, 2) || "?";
  const hash = createHash("sha1").update(letters).digest();
  const color = MONOGRAM_COLORS[hash[0] % MONOGRAM_COLORS.length];
  const fontSize = letters.length > 1 ? 8 : 10;

  return (
    `<svg viewBox="0 0 18 18" width="18" height="18" xmlns="http://www.w3.org/2000/svg">` +
    `<path fill="${color}" d="M0 0h18v18H0z"/>` +
    `<text x="9" y="9" fill="#fff" font-family="DejaVu Sans, Arial, Helvetica, sans-serif" ` +
    `font-size="${fontSize}" font-weight="700" text-anchor="middle" dominant-baseline="central">${letters}</text>` +
    `</svg>`
  );
}

export function createPngRenderer({ cacheBytes }) {
  const cache = new Map();
  let cachedBytes = 0;
  let resvgModule;

  async function loadResvg() {
    // Loaded on first use so the service still starts, and still serves SVG, without the native module.
    resvgModule ??= import("@resvg/resvg-js").catch(() => null);
    return resvgModule;
  }

  function remember(key, png) {
    if (png.length > cacheBytes) return;
    cache.set(key, png);
    cachedBytes += png.length;
    for (const [oldestKey, oldest] of cache) {
      if (cachedBytes <= cacheBytes) break;
      cache.delete(oldestKey);
      cachedBytes -= oldest.length;
    }
  }

  return {
    async available() {
      return Boolean(await loadResvg());
    },

    // Returns null when the PNG renderer is not installed. Scanning system fonts is slow,
    // so it only happens for images that contain text (the monogram fallback).
    async render(key, svg, size, { hasText = false } = {}) {
      const cacheKey = `${key}|${size}`;
      const hit = cache.get(cacheKey);
      if (hit) {
        cache.delete(cacheKey);
        cache.set(cacheKey, hit);
        return hit;
      }

      const resvg = await loadResvg();
      if (!resvg) return null;

      const png = new resvg.Resvg(svg, {
        fitTo: { mode: "width", value: size },
        font: { loadSystemFonts: hasText },
      })
        .render()
        .asPng();
      remember(cacheKey, png);
      return png;
    },
  };
}

export function parsePngSize(value) {
  if (value === undefined || value === null || value === "") return PNG_DEFAULT_SIZE;
  const size = Number(value);
  if (!Number.isInteger(size) || size < PNG_MIN_SIZE || size > PNG_MAX_SIZE) return null;
  return size;
}
