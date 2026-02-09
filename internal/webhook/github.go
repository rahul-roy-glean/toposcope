// Package webhook handles incoming GitHub webhook events.
package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// VerifySignature validates the X-Hub-Signature-256 header against the payload.
func VerifySignature(payload []byte, signature string, secret []byte) error {
	if !strings.HasPrefix(signature, "sha256=") {
		return fmt.Errorf("invalid signature format")
	}
	sig, err := hex.DecodeString(signature[7:])
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}

	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	expected := mac.Sum(nil)

	if !hmac.Equal(sig, expected) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}

// InstallationEvent represents a GitHub App installation event.
type InstallationEvent struct {
	Action       string              `json:"action"`
	Installation InstallationPayload `json:"installation"`
	Sender       GitHubUser          `json:"sender"`
}

// InstallationPayload contains installation details.
type InstallationPayload struct {
	ID      int64      `json:"id"`
	Account GitHubUser `json:"account"`
}

// InstallationRepositoriesEvent represents repos added/removed from an installation.
type InstallationRepositoriesEvent struct {
	Action              string              `json:"action"`
	Installation        InstallationPayload `json:"installation"`
	RepositoriesAdded   []GitHubRepository  `json:"repositories_added"`
	RepositoriesRemoved []GitHubRepository  `json:"repositories_removed"`
}

// PullRequestEvent represents a pull request webhook event.
type PullRequestEvent struct {
	Action       string              `json:"action"`
	Number       int                 `json:"number"`
	PullRequest  PullRequestPayload  `json:"pull_request"`
	Repository   GitHubRepository    `json:"repository"`
	Installation InstallationPayload `json:"installation"`
}

// PullRequestPayload contains pull request details.
type PullRequestPayload struct {
	Number int        `json:"number"`
	Head   GitRef     `json:"head"`
	Base   GitRef     `json:"base"`
	State  string     `json:"state"`
	User   GitHubUser `json:"user"`
}

// PushEvent represents a push webhook event.
type PushEvent struct {
	Ref          string              `json:"ref"`
	After        string              `json:"after"`
	Repository   GitHubRepository    `json:"repository"`
	Installation InstallationPayload `json:"installation"`
}

// GitRef represents a git reference (branch head).
type GitRef struct {
	SHA  string           `json:"sha"`
	Ref  string           `json:"ref"`
	Repo GitHubRepository `json:"repo"`
}

// GitHubUser represents a GitHub user or organization.
type GitHubUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

// GitHubRepository represents a GitHub repository.
type GitHubRepository struct {
	ID            int64  `json:"id"`
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
}

// ParseEvent parses a webhook payload based on the event type.
func ParseEvent(eventType string, payload []byte) (interface{}, error) {
	switch eventType {
	case "installation":
		var e InstallationEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, fmt.Errorf("parse installation event: %w", err)
		}
		return &e, nil
	case "installation_repositories":
		var e InstallationRepositoriesEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, fmt.Errorf("parse installation_repositories event: %w", err)
		}
		return &e, nil
	case "pull_request":
		var e PullRequestEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, fmt.Errorf("parse pull_request event: %w", err)
		}
		return &e, nil
	case "push":
		var e PushEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, fmt.Errorf("parse push event: %w", err)
		}
		return &e, nil
	default:
		return nil, fmt.Errorf("unsupported event type: %s", eventType)
	}
}
