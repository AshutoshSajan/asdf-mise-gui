package services

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/updater"
)

const updateSignatureSuffix = ".sig"

// signedProvider wraps an updater provider and attaches the Ed25519 signature
// published beside the selected artifact. Wails' GitHub provider only reads
// checksums.txt, so without this wrapper a pinned Config.PublicKey would make
// every release fail closed instead of verifying.
type signedProvider struct {
	inner     updater.Provider
	publicKey ed25519.PublicKey
	client    *http.Client
	baseURL   string
}

func newSignedProvider(inner updater.Provider, publicKey ed25519.PublicKey) *signedProvider {
	return &signedProvider{
		inner:     inner,
		publicKey: publicKey,
		client:    &http.Client{Timeout: 30 * time.Second},
		baseURL:   "https://github.com/" + updateRepository + "/releases/download",
	}
}

// Name must match the wrapped provider: the Updater routes the later Download
// back by this name.
func (p *signedProvider) Name() string { return p.inner.Name() }

func (p *signedProvider) Check(ctx context.Context, req updater.CheckRequest) (*updater.Release, error) {
	release, err := p.inner.Check(ctx, req)
	if err != nil || release == nil {
		return release, err
	}
	if release.Verification == nil || len(release.Verification.Digest) == 0 {
		return nil, fmt.Errorf("release %s has no SHA-256 checksum to verify", release.Version)
	}
	if release.Artifact.Filename == "" {
		return nil, fmt.Errorf("release %s has no artifact filename", release.Version)
	}

	signature, err := p.fetchSignature(ctx, releaseTag(release), release.Artifact.Filename)
	if err != nil {
		return nil, err
	}
	if !ed25519.Verify(p.publicKey, release.Verification.Digest, signature) {
		return nil, fmt.Errorf("release %s signature does not match the pinned update key", release.Version)
	}

	release.Verification.SignatureAlgo = "ed25519"
	release.Verification.Signature = signature
	return release, nil
}

func (p *signedProvider) Download(
	ctx context.Context,
	release *updater.Release,
	dst io.Writer,
	onProgress func(written, total int64),
) error {
	return p.inner.Download(ctx, release, dst, onProgress)
}

func (p *signedProvider) fetchSignature(ctx context.Context, tag, asset string) ([]byte, error) {
	endpoint := strings.TrimRight(p.baseURL, "/") + "/" +
		url.PathEscape(tag) + "/" + url.PathEscape(asset+updateSignatureSuffix)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("release signature: %w", err)
	}
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("release signature %s: %w", asset+updateSignatureSuffix, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release signature %s: HTTP %d", asset+updateSignatureSuffix, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return nil, fmt.Errorf("release signature %s: %w", asset+updateSignatureSuffix, err)
	}
	signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(body)))
	if err != nil {
		return nil, fmt.Errorf("release signature %s: %w", asset+updateSignatureSuffix, err)
	}
	if len(signature) != ed25519.SignatureSize {
		return nil, fmt.Errorf(
			"release signature %s: got %d bytes, want %d",
			asset+updateSignatureSuffix, len(signature), ed25519.SignatureSize,
		)
	}
	return signature, nil
}

func releaseTag(release *updater.Release) string {
	if release != nil && release.Metadata != nil {
		if tag, ok := release.Metadata["github.release.tag"].(string); ok && tag != "" {
			return tag
		}
	}
	if release == nil {
		return ""
	}
	return "v" + strings.TrimPrefix(release.Version, "v")
}

func hasSignature(release *updater.Release) bool {
	return release != nil &&
		release.Verification != nil &&
		len(release.Verification.Signature) > 0 &&
		strings.EqualFold(release.Verification.SignatureAlgo, "ed25519")
}
