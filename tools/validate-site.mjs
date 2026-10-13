import { readdir, readFile } from "node:fs/promises";
import path from "node:path";

const root = process.cwd();
const dataPath = path.join(root, "data", "logos.json");
const data = JSON.parse(await readFile(dataPath, "utf8"));

// The API serves these files to browsers, so nothing executable or remote may hide in them.
const UNSAFE_SVG = [
  [/<script/i, "script element"],
  [/\son[a-z]+\s*=/i, "event handler attribute"],
  [/javascript:/i, "javascript: URL"],
  [/<(foreignObject|iframe|object|embed)\b/i, "embedded document"],
  [/(href|src)\s*=\s*["']\s*(https?:)?\/\//i, "external reference"],
];

const problems = [];
let missingMetadata = 0;

for (const logo of data.logos) {
  let svg;
  try {
    svg = await readFile(path.join(root, "logos", logo.file), "utf8");
  } catch {
    problems.push(`${logo.file}: file is missing`);
    continue;
  }

  const openTag = svg.match(/<svg\b[^>]*>/)?.[0] || "";
  if (!svg.startsWith("<svg")) problems.push(`${logo.file}: does not start with <svg`);
  if (!/\sviewBox\s*=/.test(openTag)) problems.push(`${logo.file}: no viewBox (run npm run normalize:svgs)`);
  for (const [pattern, label] of UNSAFE_SVG) {
    if (pattern.test(svg)) problems.push(`${logo.file}: contains ${label}`);
  }

  if (!logo.company || !logo.sector || !logo.industry || !logo.oneLine) {
    missingMetadata += 1;
  }
}

// Every SVG on disk must be in the index, or the API would silently never serve it.
const indexed = new Set(data.logos.map((logo) => logo.file));
for (const folder of ["nse", "bse"]) {
  for (const name of await readdir(path.join(root, "logos", folder))) {
    if (name.endsWith(".svg") && !indexed.has(`${folder}/${name}`)) {
      problems.push(`${folder}/${name}: not in data/logos.json (run npm run build:data)`);
    }
  }
}

if (problems.length || missingMetadata) {
  for (const problem of problems.slice(0, 50)) console.error(problem);
  console.error(`Logo file problems: ${problems.length}`);
  console.error(`Missing metadata fields: ${missingMetadata}`);
  process.exit(1);
}

console.log(`Validated ${data.logos.length} logos`);
