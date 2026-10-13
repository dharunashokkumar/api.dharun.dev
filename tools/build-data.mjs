import { readFile, writeFile, mkdir } from "node:fs/promises";
import path from "node:path";

const root = process.cwd();
const inputPath = path.join(root, "data", "logo_metadata.full.json");
const outputPath = path.join(root, "data", "logos.json");

const source = JSON.parse(await readFile(inputPath, "utf8"));

const logos = source.logos.map((logo) => {
  const indexes = Array.isArray(logo.indexes)
    ? logo.indexes.map((item) => item.name).filter(Boolean)
    : [];

  return {
    id: logo.id,
    exchange: logo.exchange,
    ticker: logo.ticker,
    file: logo.file,
    company: logo.company_name,
    sector: logo.sector,
    industry: logo.industry,
    oneLine: logo.one_liner,
    isin: logo.isin,
    currency: logo.currency,
    marketCap: logo.market_cap,
    isIndexSymbol: logo.is_index_symbol,
    isIndexConstituent: logo.is_index_constituent,
    indexes,
  };
});

const sectors = [...new Set(logos.map((logo) => logo.sector).filter(Boolean))].sort();
const indexes = [
  ...new Set(logos.flatMap((logo) => logo.indexes || []).filter(Boolean)),
].sort();

const output = {
  generatedAt: source.generated_at,
  counts: source.counts,
  sectors,
  indexes,
  logos,
};

await mkdir(path.dirname(outputPath), { recursive: true });
await writeFile(outputPath, `${JSON.stringify(output)}\n`);

console.log(`Wrote ${path.relative(root, outputPath)} with ${logos.length} logos`);
