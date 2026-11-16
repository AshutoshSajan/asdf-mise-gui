#!/usr/bin/env node
// Compare the UPDATE_SIGNING_KEY secret with the public key committed in
// internal/version, without printing either value.
//
// A mismatch is otherwise only discovered by the publish job after a tag is
// pushed, which leaves a tagged release with no assets. This check exists so a
// rotated secret can be validated first, from the "Verify update signing key"
// workflow, by anyone who can read the public key but not the secret.
//
// Usage:
//   UPDATE_SIGNING_KEY="$(cat update-signing-key.pem)" node scripts/check-signing-key.mjs

import crypto from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const publicKeyPath = path.join(root, "internal/version/update-signing-key.pub.pem");

function spki(key) {
  return key.export({ type: "spki", format: "der" });
}

// Public fingerprints only. Safe to print: they are derived from public keys
// and cannot be used to recover the private key.
function fingerprint(der) {
  return crypto.createHash("sha256").update(der).digest("hex").slice(0, 16);
}

let committed;
try {
  committed = crypto.createPublicKey(fs.readFileSync(publicKeyPath, "utf8"));
} catch (err) {
  console.error(`cannot read ${publicKeyPath}: ${err.message}`);
  process.exit(1);
}
if (committed.asymmetricKeyType !== "ed25519") {
  console.error(`committed public key is ${committed.asymmetricKeyType}, want ed25519`);
  process.exit(1);
}

const pem = process.env.UPDATE_SIGNING_KEY?.trim();
if (!pem) {
  console.error("UPDATE_SIGNING_KEY is not set");
  process.exit(1);
}

let privateKey;
try {
  privateKey = crypto.createPrivateKey(pem);
} catch {
  // Deliberately generic: the parser can echo fragments of its input.
  console.error(
    "UPDATE_SIGNING_KEY is not a parsable PEM private key; check that the " +
      "BEGIN/END lines and the line breaks inside the base64 body survived the copy",
  );
  process.exit(1);
}
if (privateKey.asymmetricKeyType !== "ed25519") {
  console.error(`UPDATE_SIGNING_KEY is ${privateKey.asymmetricKeyType}, want ed25519`);
  process.exit(1);
}

const committedDer = spki(committed);
const derivedDer = spki(crypto.createPublicKey(privateKey));

console.log(`committed public key     ${fingerprint(committedDer)}`);
console.log(`UPDATE_SIGNING_KEY public ${fingerprint(derivedDer)}`);

if (!committedDer.equals(derivedDer)) {
  console.error(
    "UPDATE_SIGNING_KEY does not match the committed public key; the release " +
      "would fail to sign, and existing builds would reject the result",
  );
  process.exit(1);
}

console.log("UPDATE_SIGNING_KEY matches the committed public key");
