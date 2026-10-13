import { readFileSync } from "node:fs";
import path from "node:path";

const EXCHANGES = ["NSE", "BSE"];
const ISIN_PATTERN = /^IN[A-Z0-9]{10}$/;

export class BadRequestError extends Error {}

export function loadDataset(root) {
  const source = JSON.parse(readFileSync(path.join(root, "data", "logos.json"), "utf8"));

  const logos = source.logos.map((logo) => ({
    ...logo,
    marketCapValue: Number(logo.marketCap || 0),
    searchText: fold(
      [logo.id, logo.ticker, logo.company, logo.sector, logo.industry, logo.isin, ...(logo.indexes || [])]
        .filter(Boolean)
        .join(" "),
    ),
  }));

  // NSE first, then the larger company, so a bare ticker resolves the way most callers expect.
  const preferred = (a, b) =>
    EXCHANGES.indexOf(a.exchange) - EXCHANGES.indexOf(b.exchange) || b.marketCapValue - a.marketCapValue;

  const byExactTicker = new Map();
  const byLooseTicker = new Map();
  const byIsin = new Map();
  for (const logo of logos) {
    push(byExactTicker, logo.ticker.toUpperCase(), logo);
    push(byLooseTicker, looseKey(logo.ticker), logo);
    if (logo.isin) push(byIsin, logo.isin.toUpperCase(), logo);
  }
  for (const index of [byExactTicker, byLooseTicker, byIsin]) {
    for (const matches of index.values()) matches.sort(preferred);
  }

  return {
    generatedAt: source.generatedAt,
    counts: source.counts,
    sectors: source.sectors,
    indexes: source.indexes,
    logos,
    files: new Set(logos.map((logo) => logo.file)),

    // Accepts TCS, NSE:TCS, NSE_TCS, TCS.NS / TCS.BO, an ISIN, and punctuation variants
    // such as M&M / MM or BAJAJ-AUTO / BAJAJ_AUTO.
    resolve(input, exchangeHint) {
      const parsed = parseSymbol(input);
      const exchange = parseExchange(exchangeHint) || parsed.exchange;
      if (!parsed.ticker) return null;

      // Exact spelling wins over an ISIN, which wins over the punctuation-insensitive match.
      const candidateLists = [
        byExactTicker.get(parsed.ticker),
        ISIN_PATTERN.test(parsed.ticker) ? byIsin.get(parsed.ticker) : undefined,
        byLooseTicker.get(looseKey(parsed.ticker)),
      ];
      for (const candidates of candidateLists) {
        const match = candidates?.find((logo) => !exchange || logo.exchange === exchange);
        if (match) return match;
      }
      return null;
    },

    search({ q = "", exchange, sector, index, limit = 20, offset = 0 } = {}) {
      const tokens = tokenize(q);
      const wantedExchange = parseExchange(exchange);
      const wantedSector = sector ? fold(sector) : "";
      const wantedIndex = index ? fold(index) : "";

      const matches = [];
      for (const logo of logos) {
        if (wantedExchange && logo.exchange !== wantedExchange) continue;
        if (wantedSector && fold(logo.sector || "") !== wantedSector) continue;
        if (wantedIndex && !(logo.indexes || []).some((name) => fold(name) === wantedIndex)) continue;
        if (!tokens.every((token) => logo.searchText.includes(token))) continue;
        matches.push({ logo, score: tokens.length ? score(logo, tokens) : 0 });
      }

      matches.sort((a, b) => b.score - a.score || b.logo.marketCapValue - a.logo.marketCapValue);
      return {
        total: matches.length,
        results: matches.slice(offset, offset + limit).map((match) => match.logo),
      };
    },
  };
}

export function parseSymbol(input) {
  let ticker = String(input ?? "").trim().toUpperCase().replace(/\s+/g, "");
  let exchange = null;

  const prefixed = ticker.match(/^(NSE|BSE)[:_](.+)$/);
  if (prefixed) {
    exchange = prefixed[1];
    ticker = prefixed[2];
  }

  const yahoo = ticker.match(/^(.+)\.(NS|BO)$/);
  if (yahoo) {
    ticker = yahoo[1];
    exchange = yahoo[2] === "NS" ? "NSE" : "BSE";
  }

  return { ticker, exchange };
}

export function parseExchange(value) {
  if (value === undefined || value === null || value === "") return null;
  const exchange = String(value).trim().toUpperCase();
  if (exchange === "AUTO" || exchange === "ALL") return null;
  if (!EXCHANGES.includes(exchange)) {
    throw new BadRequestError(`Unknown exchange "${value}". Use nse or bse.`);
  }
  return exchange;
}

function looseKey(ticker) {
  return ticker.toUpperCase().replace(/[^A-Z0-9]/g, "");
}

function push(map, key, value) {
  const list = map.get(key);
  if (list) list.push(value);
  else map.set(key, [value]);
}

function fold(value) {
  return String(value || "")
    .toLowerCase()
    .normalize("NFKD")
    .replace(/[̀-ͯ]/g, "");
}

function tokenize(value) {
  return fold(value)
    .split(/[^a-z0-9&.]+/)
    .filter(Boolean);
}

// Mirrors the ranking the website uses so both return results in the same order.
function score(logo, tokens) {
  const ticker = fold(logo.ticker);
  const company = fold(logo.company || "");
  const sector = fold(logo.sector || "");
  let total = 0;

  for (const token of tokens) {
    if (ticker === token) total += 120;
    else if (ticker.startsWith(token)) total += 75;
    else if (ticker.includes(token)) total += 40;

    if (company.startsWith(token)) total += 45;
    else if (company.includes(token)) total += 22;

    if (sector.includes(token)) total += 12;
  }
  return total;
}
