#!/usr/bin/env node
// Sign release artifacts with the repository's Ed25519 update key.
//
// The app pins the public key embedded in internal/version and Wails verifies
// each download against it. The private key is never committed; it lives in
// the UPDATE_SIGNING_KEY repository secret.
//
// Usage:
//   UPDATE_SIGNING_KEY="$(cat update-signing-key.pem)" node scripts/sign-release.mjs dist
//   node scripts/sign-release.mjs --verify dist
//   node scripts/sign-release.mjs --public-key old.pub.pem dist
//
// Each signature is Ed25519 over the raw SHA-256 digest of the artifact and is
// written beside it as "<artifact>.sig" in base64. CI signs every published
// file, including checksums.txt, and verifies each signature against the
// committed public key before the release is published.

import crypto from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const args = process.argv.slice(2);
const verifyOnly = args.includes("--verify");

const publicKeyFlag = args.indexOf("--public-key");
const publicKeyArg = publicKeyFlag >= 0 ? args[publicKeyFlag + 1] : undefined;
if (publicKeyFlag >= 0 && !publicKeyArg) {
  console.error("--public-key requires a PEM path");
  process.exit(1);
}
const publicKeyPath = publicKeyArg
  ? path.resolve(root, publicKeyArg)
  : path.join(root, "internal/version/update-signing-key.pub.pem");

const positional = args.filter(
  (arg, index) => !arg.startsWith("--") && !(publicKeyFlag >= 0 && index === publicKeyFlag + 1),
);
const dist = path.resolve(root, positional[0] ?? "dist");

if (!fs.existsSync(dist) || !fs.statSync(dist).isDirectory()) {
  console.error(`release directory not found: ${dist}`);
  process.exit(1);
}

const publicKey = crypto.createPublicKey(fs.readFileSync(publicKeyPath, "utf8"));
if (publicKey.asymmetricKeyType !== "ed25519") {
  console.error(`update public key is ${publicKey.asymmetricKeyType}, want ed25519`);
  process.exit(1);
}

let privateKey;
if (!verifyOnly) {
  const pem = process.env.UPDATE_SIGNING_KEY?.trim();
  if (!pem) {
    console.error("UPDATE_SIGNING_KEY is not set; refusing to publish unsigned artifacts");
    process.exit(1);
  }
  privateKey = crypto.createPrivateKey(pem);
  if (privateKey.asymmetricKeyType !== "ed25519") {
    console.error(`UPDATE_SIGNING_KEY is ${privateKey.asymmetricKeyType}, want ed25519`);
    process.exit(1);
  }
}

const files = fs
  .readdirSync(dist, { withFileTypes: true })
  .filter((entry) => entry.isFile() && !entry.name.endsWith(".sig"))
  .map((entry) => entry.name)
  .sort();

if (files.length === 0) {
  console.error(`no release artifacts found in ${dist}`);
  process.exit(1);
}

function decodeSignature(value, name) {
  const signature = Buffer.from(value.trim(), "base64");
  if (signature.length !== 64) {
    console.error(`${name}: signature has ${signature.length} bytes, want 64`);
    return null;
  }
  return signature;
}

let failed = false;
for (const name of files) {
  const artifact = path.join(dist, name);
  const signaturePath = `${artifact}.sig`;
  const digest = crypto.createHash("sha256").update(fs.readFileSync(artifact)).digest();

  if (verifyOnly) {
    if (!fs.existsSync(signaturePath)) {
      console.error(`${name}: missing ${name}.sig`);
      failed = true;
      continue;
    }
    const signature = decodeSignature(fs.readFileSync(signaturePath, "utf8"), name);
    if (!signature || !crypto.verify(null, digest, publicKey, signature)) {
      console.error(`${name}: signature did not verify against the pinned public key`);
      failed = true;
      continue;
    }
    console.log(`verified ${name}`);
    continue;
  }

  const signature = crypto.sign(null, digest, privateKey);
  if (!crypto.verify(null, digest, publicKey, signature)) {
    console.error(`${name}: UPDATE_SIGNING_KEY does not match the committed public key`);
    failed = true;
    break;
  }
  fs.writeFileSync(signaturePath, `${signature.toString("base64")}\n`);
  console.log(`signed ${name} (${signature.length} bytes)`);
}

if (failed) {
  process.exit(1);
}
console.log(`${verifyOnly ? "Verified" : "Signed"} ${files.length} artifacts in ${dist}`);
