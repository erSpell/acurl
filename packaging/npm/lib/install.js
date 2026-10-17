"use strict";
// Downloads the acurl release binary that matches this package version,
// verifies it against the release checksums and stores it next to this file.

const crypto = require("node:crypto");
const fs = require("node:fs");
const https = require("node:https");
const os = require("node:os");
const path = require("node:path");
const { execFileSync } = require("node:child_process");

const REPO = "tragent-io/acurl";
const VERSION = require("../package.json").version;
const BIN_DIR = path.join(__dirname, "..", "vendor");

function target(platform = process.platform, arch = process.arch) {
  const osName = { darwin: "darwin", linux: "linux", win32: "windows" }[platform];
  const cpu = { x64: "amd64", arm64: "arm64" }[arch];
  if (!osName || !cpu) {
    throw new Error(`acurl has no prebuilt binary for ${platform}/${arch}. Install it with Go instead: go install github.com/${REPO}/cmd/acurl@latest`);
  }
  const ext = osName === "windows" ? "zip" : "tar.gz";
  return {
    archive: `acurl_${VERSION}_${osName}_${cpu}.${ext}`,
    binary: osName === "windows" ? "acurl.exe" : "acurl",
  };
}

function binaryPath() {
  return path.join(BIN_DIR, target().binary);
}

function get(url, redirects = 5) {
  return new Promise((resolve, reject) => {
    https.get(url, { headers: { "User-Agent": "acurl-npm" } }, (res) => {
      if ([301, 302, 303, 307, 308].includes(res.statusCode) && res.headers.location && redirects > 0) {
        res.resume();
        resolve(get(res.headers.location, redirects - 1));
        return;
      }
      if (res.statusCode !== 200) {
        res.resume();
        reject(new Error(`GET ${url}: HTTP ${res.statusCode}`));
        return;
      }
      const chunks = [];
      res.on("data", (c) => chunks.push(c));
      res.on("end", () => resolve(Buffer.concat(chunks)));
    }).on("error", reject);
  });
}

function expectedSum(checksums, archive) {
  for (const line of checksums.split("\n")) {
    const [sum, name] = line.trim().split(/\s+/);
    if (name === archive) return sum;
  }
  throw new Error(`${archive} is not listed in checksums.txt`);
}

async function install() {
  const bin = binaryPath();
  if (fs.existsSync(bin)) return bin;
  const { archive, binary } = target();
  const base = `https://github.com/${REPO}/releases/download/v${VERSION}`;

  const [data, sums] = await Promise.all([get(`${base}/${archive}`), get(`${base}/checksums.txt`)]);
  const actual = crypto.createHash("sha256").update(data).digest("hex");
  if (actual !== expectedSum(sums.toString("utf8"), archive)) {
    throw new Error(`checksum mismatch for ${archive}`);
  }

  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "acurl-"));
  try {
    const file = path.join(tmp, archive);
    fs.writeFileSync(file, data);
    // bsdtar (macOS, Windows 10+) and GNU tar both read .tar.gz; bsdtar also reads .zip.
    execFileSync("tar", ["-xf", file, "-C", tmp, binary], { stdio: "ignore" });
    fs.mkdirSync(BIN_DIR, { recursive: true });
    fs.copyFileSync(path.join(tmp, binary), bin);
    fs.chmodSync(bin, 0o755);
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
  return bin;
}

module.exports = { install, binaryPath, target, expectedSum, VERSION };

if (require.main === module) {
  install().catch((err) => {
    // Do not fail npm install: bin/acurl.js retries on first run.
    console.warn(`acurl: could not download the binary now (${err.message}); it will be fetched on first run.`);
  });
}
