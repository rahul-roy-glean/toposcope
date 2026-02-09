// Package surface publishes Toposcope results to external systems.
package surface

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/toposcope/toposcope/pkg/surface"
)

// GitHubPublisher publishes Check Runs to the GitHub API using
// GitHub App authentication (JWT -> installation token).
type GitHubPublisher struct {
	appID      int64
	privateKey *rsa.PrivateKey
	httpClient *http.Client
}

// NewGitHubPublisher creates a publisher from the App ID and PEM-encoded private key.
func NewGitHubPublisher(appID int64, privateKeyPEM []byte) (*GitHubPublisher, error) {
	block, _ := pem.Decode(privateKeyPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	return &GitHubPublisher{
		appID:      appID,
		privateKey: key,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// PublishCheckRun creates a GitHub Check Run on the given commit.
func (p *GitHubPublisher) PublishCheckRun(ctx context.Context, installationID int64, owner, repo, headSHA string, data surface.CheckRunData) error {
	token, err := p.getInstallationToken(ctx, installationID)
	if err != nil {
		return fmt.Errorf("get installation token: %w", err)
	}

	body := map[string]interface{}{
		"name":       "Toposcope",
		"head_sha":   headSHA,
		"status":     "completed",
		"conclusion": data.Conclusion,
		"output": map[string]string{
			"title":   data.Title,
			"summary": data.Summary,
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal check run: %w", err)
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/check-runs", owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("post check run: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github API error %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// getInstallationToken generates a JWT and exchanges it for an installation access token.
func (p *GitHubPublisher) getInstallationToken(ctx context.Context, installationID int64) (string, error) {
	jwt, err := p.generateJWT()
	if err != nil {
		return "", fmt.Errorf("generate JWT: %w", err)
	}

	url := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", installationID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return "", fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request installation token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request failed %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	return result.Token, nil
}

// generateJWT creates a short-lived JWT for GitHub App authentication.
func (p *GitHubPublisher) generateJWT() (string, error) {
	now := time.Now()
	// GitHub App JWTs: iat is backdated 60s, exp is max 10 minutes
	iat := now.Add(-60 * time.Second)
	exp := now.Add(5 * time.Minute)

	return signJWT(p.appID, iat, exp, p.privateKey)
}

// signJWT creates a minimal RS256 JWT. This avoids importing a full JWT library
// for a single use case.
func signJWT(appID int64, iat, exp time.Time, key *rsa.PrivateKey) (string, error) {
	header := map[string]string{"alg": "RS256", "typ": "JWT"}
	payload := map[string]interface{}{
		"iss": appID,
		"iat": iat.Unix(),
		"exp": exp.Unix(),
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	headerB64 := base64URLEncode(headerJSON)
	payloadB64 := base64URLEncode(payloadJSON)
	signingInput := headerB64 + "." + payloadB64

	signature, err := rsaSign([]byte(signingInput), key)
	if err != nil {
		return "", fmt.Errorf("rsa sign: %w", err)
	}

	return signingInput + "." + base64URLEncode(signature), nil
}
