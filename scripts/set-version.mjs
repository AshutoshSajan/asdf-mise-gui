#!/usr/bin/env node
// Set the application version everywhere it is recorded.
//
// Usage:
//   node scripts/set-version.mjs 0.2.0
//   node scripts/set-version.mjs v0.2.0

import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const input = process.argv[2] ?? "";
const version = input.replace(/^v/, "");

if (!/^\d+\.\d+\.\d+(?:[-.][0-9A-Za-z.-]+)?$/.test(version)) {
  console.error("usage: node scripts/set-version.mjs <semver>");
  process.exit(1);
}

function replaceLine(file, pattern, replacement) {
  const full = path.join(root, file);
  const source = fs.readFileSync(full, "utf8");
  if (!pattern.test(source)) {
    throw new Error(`${file}: expected version line was not found`);
  }
  const next = source.replace(pattern, replacement);
  if (next !== source) {
    fs.writeFileSync(full, next);
  }
}

replaceLine("build/config.yml", /^  version: ".*"(.*)$/m, `  version: "${version}"$1`);
replaceLine("build/linux/nfpm/nfpm.yaml", /^version: ".*"(.*)$/m, `version: "${version}"$1`);
replaceLine("internal/version/version.go", /^var Version = ".*"(.*)$/m, `var Version = "${version}"$1`);

for (const file of ["build/darwin/Info.plist", "build/darwin/Info.dev.plist"]) {
  replaceLine(
    file,
    /(<key>CFBundleShortVersionString<\/key>\s*<string>)[^<]*(<\/string>)/,
    `$1${version}$2`,
  );
  replaceLine(file, /(<key>CFBundleVersion<\/key>\s*<string>)[^<]*(<\/string>)/, `$1${version}$2`);
}

const packagePath = path.join(root, "frontend/package.json");
const pkg = JSON.parse(fs.readFileSync(packagePath, "utf8"));
pkg.version = version;
fs.writeFileSync(packagePath, `${JSON.stringify(pkg, null, 2)}\n`);

const lockPath = path.join(root, "frontend/package-lock.json");
const lock = JSON.parse(fs.readFileSync(lockPath, "utf8"));
lock.version = version;
lock.packages ??= {};
lock.packages[""] ??= {};
lock.packages[""].version = version;
fs.writeFileSync(lockPath, `${JSON.stringify(lock, null, 2)}\n`);

console.log(`Set application version to ${version}`);
