import assert from "node:assert/strict";
import { execFile, execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { chmodSync, existsSync, mkdirSync, mkdtempSync, readFileSync, readlinkSync, rmSync, writeFileSync } from "node:fs";
import http from "node:http";
import os from "node:os";
import path from "node:path";
import { after, before, test } from "node:test";
import { fileURLToPath } from "node:url";

// Exercises deploy/logos-api-update.sh end to end against a fake release server and a
// fake service. It needs symlinks, GNU tar and bash, so it only runs on Linux (as CI does).

const SKIP = process.platform !== "linux" && "the updater targets Linux";
const UPDATER = fileURLToPath(new URL("../deploy/logos-api-update.sh", import.meta.url));

let scratch;
let published;
let home;
let releaseServer;
let env;

// Publishes a bundle the way the release workflow does: VERSION, tarball, checksum.
function publish(version, { broken = false, corruptChecksum = false } = {}) {
  const stage = path.join(scratch, "stage");
  rmSync(stage, { recursive: true, force: true });
  mkdirSync(path.join(stage, "logos-api"), { recursive: true });
  writeFileSync(path.join(stage, "logos-api", "VERSION"), `${version}\n`);
  if (broken) writeFileSync(path.join(stage, "logos-api", "BROKEN"), "");

  const tarball = path.join(published, "logos-api.tar.gz");
  execFileSync("tar", ["-C", stage, "-czf", tarball, "logos-api"]);
  const digest = corruptChecksum ? "0".repeat(64) : createHash("sha256").update(readFileSync(tarball)).digest("hex");
  writeFileSync(`${tarball}.sha256`, `${digest}  logos-api.tar.gz\n`);
  writeFileSync(path.join(published, "VERSION"), `${version}\n`);
}

// Asynchronous on purpose: the release server lives in this process and must keep answering.
function runUpdater() {
  return new Promise((resolve) => {
    execFile("bash", [UPDATER], { env, encoding: "utf8" }, (error, stdout, stderr) => {
      resolve({ code: error ? error.code : 0, output: `${stdout}${stderr}` });
    });
  });
}

const currentVersion = () => readFileSync(path.join(home, "current", "VERSION"), "utf8").trim();

async function servedVersion() {
  try {
    return (await (await fetch(env.LOGOS_HEALTH_URL)).json()).version;
  } catch {
    return null;
  }
}

before(async () => {
  if (SKIP) return;
  scratch = mkdtempSync(path.join(os.tmpdir(), "logos-updater-"));
  published = path.join(scratch, "published");
  home = path.join(scratch, "home");
  mkdirSync(published);
  mkdirSync(home);

  releaseServer = http.createServer((req, res) => {
    const file = path.join(published, path.basename(req.url));
    if (!existsSync(file)) return res.writeHead(404).end();
    res.writeHead(200).end(readFileSync(file));
  });
  await new Promise((resolve) => releaseServer.listen(0, "127.0.0.1", resolve));

  const servicePort = await new Promise((resolve) => {
    const probe = http.createServer().listen(0, "127.0.0.1", () => {
      const { port } = probe.address();
      probe.close(() => resolve(port));
    });
  });

  // Stand-in for `systemctl restart`: stops the old process and starts one that reports
  // whatever VERSION `current` points at. A release containing BROKEN never comes up.
  const service = path.join(scratch, "service.mjs");
  writeFileSync(
    service,
    `import http from "node:http";
import { readFileSync } from "node:fs";
const version = readFileSync(process.argv[2] + "/current/VERSION", "utf8").trim();
http.createServer((req, res) => res.end(JSON.stringify({ status: "ok", version, logos: 1 }))).listen(${servicePort}, "127.0.0.1");
`,
  );
  const restart = path.join(scratch, "restart.sh");
  writeFileSync(
    restart,
    `#!/usr/bin/env bash
pidfile="${scratch}/service.pid"
if [ -f "$pidfile" ]; then kill "$(cat "$pidfile")" 2>/dev/null || true; rm -f "$pidfile"; sleep 0.3; fi
[ -e "${home}/current/BROKEN" ] && exit 0
nohup "${process.execPath}" "${service}" "${home}" >/dev/null 2>&1 &
echo $! > "$pidfile"
`,
  );
  chmodSync(restart, 0o755);

  env = {
    PATH: process.env.PATH,
    LOGOS_RELEASE_URL: `http://127.0.0.1:${releaseServer.address().port}`,
    LOGOS_HOME: home,
    LOGOS_HEALTH_URL: `http://127.0.0.1:${servicePort}/healthz`,
    LOGOS_RESTART_CMD: restart,
    LOGOS_HEALTH_TRIES: "5",
  };
});

after(() => {
  if (SKIP) return;
  releaseServer.close();
  try {
    process.kill(Number(readFileSync(path.join(scratch, "service.pid"), "utf8")));
  } catch {
    // already stopped
  }
  rmSync(scratch, { recursive: true, force: true });
});

test("updater installs, upgrades, rolls back and refuses bad bundles", { skip: SKIP }, async (t) => {
  await t.test("first run installs the published release", async () => {
    publish("1.0.0");
    const run = await runUpdater();
    assert.equal(run.code, 0, run.output);
    assert.equal(readlinkSync(path.join(home, "current")), "releases/1.0.0");
    assert.equal(await servedVersion(), "1.0.0");
  });

  await t.test("running again with nothing new changes nothing", async () => {
    const run = await runUpdater();
    assert.equal(run.code, 0, run.output);
    assert.equal(run.output, "");
    assert.equal(currentVersion(), "1.0.0");
  });

  await t.test("a new release is deployed", async () => {
    publish("1.1.0");
    const run = await runUpdater();
    assert.equal(run.code, 0, run.output);
    assert.match(run.output, /deployed 1\.1\.0/);
    assert.equal(currentVersion(), "1.1.0");
    assert.equal(await servedVersion(), "1.1.0");
    assert.ok(existsSync(path.join(home, "releases", "1.0.0")), "previous release is kept for rollback");
  });

  await t.test("a release that fails its health check is rolled back and not retried", async () => {
    publish("1.2.0", { broken: true });
    const run = await runUpdater();
    assert.equal(run.code, 1, run.output);
    assert.match(run.output, /rolled back to 1\.1\.0/);
    assert.equal(currentVersion(), "1.1.0");
    assert.equal(await servedVersion(), "1.1.0");
    assert.ok(existsSync(path.join(home, "failed", "1.2.0")));

    const again = await runUpdater();
    assert.equal(again.code, 0, again.output);
    assert.match(again.output, /skipping 1\.2\.0/);
    assert.equal(await servedVersion(), "1.1.0");
  });

  await t.test("a bundle with a wrong checksum is never activated", async () => {
    publish("1.3.0", { corruptChecksum: true });
    const run = await runUpdater();
    assert.notEqual(run.code, 0, run.output);
    assert.equal(currentVersion(), "1.1.0");
    assert.equal(await servedVersion(), "1.1.0");
    assert.ok(!existsSync(path.join(home, "releases", "1.3.0")));
  });

  await t.test("a VERSION that is not a version number is rejected", async () => {
    writeFileSync(path.join(published, "VERSION"), "../../etc\n");
    const run = await runUpdater();
    assert.equal(run.code, 1, run.output);
    assert.match(run.output, /not a version number/);
    assert.equal(currentVersion(), "1.1.0");
  });

  await t.test("old releases are pruned, the running one is kept", async () => {
    for (const version of ["1.4.0", "1.5.0", "1.6.0"]) {
      publish(version);
      assert.equal((await runUpdater()).code, 0, version);
    }
    assert.equal(await servedVersion(), "1.6.0");
    const kept = execFileSync("ls", [path.join(home, "releases")], { encoding: "utf8" }).trim().split("\n");
    assert.equal(kept.length, 3, kept.join(","));
    assert.ok(kept.includes("1.6.0"));
    assert.ok(!kept.includes("1.0.0"));
  });
});
