package services

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/updater"
)

type stubProvider struct {
	name     string
	release  *updater.Release
	checkErr error
	download bool
}

func (p *stubProvider) Name() string {
	if p.name == "" {
		return "github"
	}
	return p.name
}

func (p *stubProvider) Check(context.Context, updater.CheckRequest) (*updater.Release, error) {
	return p.release, p.checkErr
}

func (p *stubProvider) Download(context.Context, *updater.Release, io.Writer, func(int64, int64)) error {
	p.download = true
	return nil
}

func TestSignedProviderAttachesVerifiedSignature(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("appimage bytes"))
	signature := ed25519.Sign(privateKey, digest[:])

	var requested string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = r.URL.Path
		_, _ = io.WriteString(w, base64.StdEncoding.EncodeToString(signature)+"\n")
	}))
	defer server.Close()

	inner := &stubProvider{release: signedTestRelease(digest[:])}
	provider := newSignedProvider(inner, publicKey)
	provider.baseURL = server.URL
	provider.client = server.Client()

	release, err := provider.Check(context.Background(), updater.CheckRequest{})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if requested != "/v0.4.0/asdf-mise-gui-linux-x86_64.AppImage.sig" {
		t.Fatalf("requested %q, want the release sidecar", requested)
	}
	if release.Verification.SignatureAlgo != "ed25519" {
		t.Fatalf("signature algo = %q", release.Verification.SignatureAlgo)
	}
	if string(release.Verification.Signature) != string(signature) {
		t.Fatal("release signature was not attached")
	}
	if provider.Name() != inner.Name() {
		t.Fatalf("Name() = %q, want %q", provider.Name(), inner.Name())
	}
	if err := provider.Download(context.Background(), release, io.Discard, func(int64, int64) {}); err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if !inner.download {
		t.Fatal("Download() was not delegated to the wrapped provider")
	}
}

func TestSignedProviderRejectsSignatureFromAnotherKey(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, otherKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("appimage bytes"))
	signature := ed25519.Sign(otherKey, digest[:])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, base64.StdEncoding.EncodeToString(signature))
	}))
	defer server.Close()

	provider := newSignedProvider(&stubProvider{release: signedTestRelease(digest[:])}, publicKey)
	provider.baseURL = server.URL
	provider.client = server.Client()

	_, err = provider.Check(context.Background(), updater.CheckRequest{})
	if err == nil || !strings.Contains(err.Error(), "does not match the pinned update key") {
		t.Fatalf("Check() error = %v, want a pinned-key mismatch", err)
	}
}

func TestSignedProviderRequiresChecksum(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	release := signedTestRelease(nil)
	release.Verification = nil

	provider := newSignedProvider(&stubProvider{release: release}, publicKey)
	_, err = provider.Check(context.Background(), updater.CheckRequest{})
	if err == nil || !strings.Contains(err.Error(), "no SHA-256 checksum") {
		t.Fatalf("Check() error = %v, want a missing-checksum error", err)
	}
}

func TestSignedProviderRejectsMissingSignatureAsset(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("appimage bytes"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, nil)
	}))
	defer server.Close()

	provider := newSignedProvider(&stubProvider{release: signedTestRelease(digest[:])}, publicKey)
	provider.baseURL = server.URL
	provider.client = server.Client()

	_, err = provider.Check(context.Background(), updater.CheckRequest{})
	if err == nil || !strings.Contains(err.Error(), "HTTP 404") {
		t.Fatalf("Check() error = %v, want a missing-sidecar error", err)
	}
}

func TestSignedProviderPropagatesProviderErrors(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("rate limited")
	provider := newSignedProvider(&stubProvider{checkErr: want}, publicKey)

	_, err = provider.Check(context.Background(), updater.CheckRequest{})
	if !errors.Is(err, want) {
		t.Fatalf("Check() error = %v, want %v", err, want)
	}
}

func TestReleaseTagFallsBackToVersion(t *testing.T) {
	release := &updater.Release{Version: "0.4.0"}
	if got := releaseTag(release); got != "v0.4.0" {
		t.Fatalf("releaseTag() = %q, want v0.4.0", got)
	}
	release.Metadata = map[string]any{"github.release.tag": "v0.4.0-rc.1"}
	if got := releaseTag(release); got != "v0.4.0-rc.1" {
		t.Fatalf("releaseTag() = %q, want the metadata tag", got)
	}
}

func signedTestRelease(digest []byte) *updater.Release {
	return &updater.Release{
		Version: "0.4.0",
		Artifact: updater.Artifact{
			Filename: "asdf-mise-gui-linux-x86_64.AppImage",
		},
		Verification: &updater.Verification{
			DigestAlgo: "sha256",
			Digest:     digest,
		},
		Metadata: map[string]any{
			"github.release.tag": "v0.4.0",
		},
	}
}
