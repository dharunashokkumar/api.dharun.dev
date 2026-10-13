import assert from "node:assert/strict";
import { test } from "node:test";

import { loadConfig } from "../server/config.mjs";
import { BadRequestError, loadDataset, parseSymbol } from "../server/dataset.mjs";

const dataset = loadDataset(loadConfig({}).root);

test("every logo resolves by its own id and by its bare ticker", () => {
  for (const logo of dataset.logos) {
    assert.equal(dataset.resolve(logo.id)?.id, logo.id, `id ${logo.id}`);
    assert.equal(dataset.resolve(logo.ticker, logo.exchange)?.id, logo.id, `ticker ${logo.ticker}`);
  }
});

test("accepts the common ways of writing a symbol", () => {
  for (const symbol of ["TCS", "tcs", " Tcs ", "NSE:TCS", "NSE_TCS", "TCS.NS", "INE467B01029"]) {
    assert.equal(dataset.resolve(symbol)?.id, "NSE:TCS", symbol);
  }
});

test("punctuation in tickers is optional", () => {
  assert.equal(dataset.resolve("M&M")?.id, "NSE:M&M");
  assert.equal(dataset.resolve("MM")?.id, "NSE:M&M");
  assert.equal(dataset.resolve("BAJAJ-AUTO")?.id, "NSE:BAJAJ_AUTO");
  assert.equal(dataset.resolve("BAJAJAUTO")?.id, "NSE:BAJAJ_AUTO");
});

test("an exact ticker is never shadowed by a punctuation-insensitive match", () => {
  const exact = new Map();
  for (const logo of dataset.logos) exact.set(`${logo.exchange}:${logo.ticker.toUpperCase()}`, logo.id);
  for (const logo of dataset.logos) {
    assert.equal(dataset.resolve(logo.ticker, logo.exchange).id, exact.get(`${logo.exchange}:${logo.ticker.toUpperCase()}`));
  }
});

test("exchange hints and suffixes select the listing", () => {
  const bseOnly = dataset.logos.find((logo) => logo.exchange === "BSE");
  assert.equal(dataset.resolve(bseOnly.ticker)?.id, bseOnly.id, "bare ticker falls through to BSE");
  assert.equal(dataset.resolve(`${bseOnly.ticker}.BO`)?.id, bseOnly.id);
  assert.equal(dataset.resolve(bseOnly.ticker, "nse"), null);
  assert.equal(dataset.resolve("TCS", "bse"), null);
  assert.throws(() => dataset.resolve("TCS", "nyse"), BadRequestError);
});

test("unknown and empty symbols resolve to null", () => {
  assert.equal(dataset.resolve("DEFINITELYNOTLISTED"), null);
  assert.equal(dataset.resolve(""), null);
  assert.equal(dataset.resolve(undefined), null);
});

test("a ticker that is also an exchange name is not mistaken for a prefix", () => {
  assert.deepEqual(parseSymbol("BSE"), { ticker: "BSE", exchange: null });
  assert.deepEqual(parseSymbol("nse:bse"), { ticker: "BSE", exchange: "NSE" });
});

test("search ranks an exact ticker first and honours filters", () => {
  const exact = dataset.search({ q: "TCS" });
  assert.equal(exact.results[0].id, "NSE:TCS");

  const nifty = dataset.search({ index: "nifty 50", limit: 100 });
  assert.ok(nifty.total >= 45 && nifty.total <= 55, `Nifty 50 has ${nifty.total} members`);
  assert.ok(nifty.results.every((logo) => logo.indexes.includes("Nifty 50")));

  const bse = dataset.search({ q: "ltd", exchange: "bse", limit: 5 });
  assert.equal(bse.results.length, 5);
  assert.ok(bse.results.every((logo) => logo.exchange === "BSE"));

  const paged = dataset.search({ q: "bank", limit: 2, offset: 2 });
  assert.deepEqual(
    paged.results.map((logo) => logo.id),
    dataset.search({ q: "bank", limit: 4 }).results.slice(2).map((logo) => logo.id),
  );

  assert.equal(dataset.search({ q: "zzzzqqqq" }).total, 0);
});
