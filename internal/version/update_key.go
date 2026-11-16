package version

import (
	"crypto/ed25519"
	"crypto/x509"
	_ "embed"
	"encoding/pem"
	"errors"
	"fmt"
)

// UpdatePublicKeyPEM is the Ed25519 key that authenticates release artifacts.
// It is the only trust anchor for update signatures: the release feed cannot
// replace it at runtime. The matching private key lives only in the
// UPDATE_SIGNING_KEY repository secret and is never committed.
//
//go:embed update-signing-key.pub.pem
var UpdatePublicKeyPEM []byte

// UpdatePublicKey parses the embedded trust anchor. The updater takes the PEM
// bytes directly; this helper exists so startup can reject a malformed key
// before a release check ever reaches the update download.
func UpdatePublicKey() (ed25519.PublicKey, error) {
	block, _ := pem.Decode(UpdatePublicKeyPEM)
	if block == nil {
		return nil, errors.New("version: embedded update public key is not valid PEM")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("version: parse update public key: %w", err)
	}
	key, ok := parsed.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("version: update public key has type %T, want ed25519", parsed)
	}
	if len(key) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("version: update public key has %d bytes, want %d", len(key), ed25519.PublicKeySize)
	}
	return key, nil
}
