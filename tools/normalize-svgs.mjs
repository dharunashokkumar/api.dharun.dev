import { readdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";

// Gives every logo a viewBox so it scales when an <img> or the PNG renderer asks for
// a size other than the intrinsic one. Safe to re-run: files that already have one are skipped.

const root = process.cwd();
let changed = 0;
const skipped = [];

for (const folder of ["nse", "bse"]) {
  const directory = path.join(root, "logos", folder);
  for (const name of (await readdir(directory)).filter((file) => file.endsWith(".svg"))) {
    const filePath = path.join(directory, name);
    const source = await readFile(filePath, "utf8");
    const openTag = source.match(/<svg\b[^>]*>/);

    if (!openTag) {
      skipped.push(`${folder}/${name}: no <svg> tag`);
      continue;
    }
    if (/\sviewBox\s*=/.test(openTag[0])) continue;

    const width = openTag[0].match(/\swidth="(\d+(?:\.\d+)?)(?:px)?"/);
    const height = openTag[0].match(/\sheight="(\d+(?:\.\d+)?)(?:px)?"/);
    if (!width || !height) {
      skipped.push(`${folder}/${name}: no numeric width/height`);
      continue;
    }

    const fixedTag = openTag[0].replace(/^<svg\b/, `<svg viewBox="0 0 ${width[1]} ${height[1]}"`);
    await writeFile(filePath, source.replace(openTag[0], fixedTag));
    changed += 1;
  }
}

console.log(`Added viewBox to ${changed} logos`);
if (skipped.length) {
  console.error(`Could not normalize ${skipped.length} logos:`);
  for (const line of skipped) console.error(`  ${line}`);
  process.exit(1);
}
