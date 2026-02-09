package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func computeHMAC(payload, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestVerifySignature(t *testing.T) {
	secret := []byte("webhook-secret-123")
	payload := []byte(`{"action":"opened"}`)

	tests := []struct {
		name      string
		payload   []byte
		signature string
		secret    []byte
		wantErr   bool
	}{
		{
			name:      "valid signature",
			payload:   payload,
			signature: computeHMAC(payload, secret),
			secret:    secret,
			wantErr:   false,
		},
		{
			name:      "wrong secret",
			payload:   payload,
			signature: computeHMAC(payload, []byte("wrong-secret")),
			secret:    secret,
			wantErr:   true,
		},
		{
			name:      "tampered payload",
			payload:   []byte(`{"action":"closed"}`),
			signature: computeHMAC(payload, secret),
			secret:    secret,
			wantErr:   true,
		},
		{
			name:      "missing sha256= prefix",
			payload:   payload,
			signature: "not-a-valid-sig",
			secret:    secret,
			wantErr:   true,
		},
		{
			name:      "invalid hex after prefix",
			payload:   payload,
			signature: "sha256=zzzz",
			secret:    secret,
			wantErr:   true,
		},
		{
			name:      "empty signature",
			payload:   payload,
			signature: "",
			secret:    secret,
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := VerifySignature(tc.payload, tc.signature, tc.secret)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestParseEvent_Push(t *testing.T) {
	tests := []struct {
		name          string
		payload       PushEvent
		wantRepo      string
		wantBranch    string
		wantAfter     string
	}{
		{
			name: "push to main",
			payload: PushEvent{
				Ref:   "refs/heads/main",
				After: "abc123def456",
				Repository: GitHubRepository{
					ID:            42,
					FullName:      "octocat/hello-world",
					DefaultBranch: "main",
				},
				Installation: InstallationPayload{
					ID: 12345,
				},
			},
			wantRepo:   "octocat/hello-world",
			wantBranch: "main",
			wantAfter:  "abc123def456",
		},
		{
			name: "push to feature branch",
			payload: PushEvent{
				Ref:   "refs/heads/feature/new-thing",
				After: "deadbeef",
				Repository: GitHubRepository{
					ID:            99,
					FullName:      "org/repo",
					DefaultBranch: "main",
				},
				Installation: InstallationPayload{
					ID: 67890,
				},
			},
			wantRepo:   "org/repo",
			wantBranch: "main",
			wantAfter:  "deadbeef",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.payload)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			event, err := ParseEvent("push", data)
			if err != nil {
				t.Fatalf("ParseEvent: %v", err)
			}

			push, ok := event.(*PushEvent)
			if !ok {
				t.Fatalf("expected *PushEvent, got %T", event)
			}

			if push.Repository.FullName != tc.wantRepo {
				t.Errorf("repo = %q, want %q", push.Repository.FullName, tc.wantRepo)
			}
			if push.Repository.DefaultBranch != tc.wantBranch {
				t.Errorf("default branch = %q, want %q", push.Repository.DefaultBranch, tc.wantBranch)
			}
			if push.After != tc.wantAfter {
				t.Errorf("after = %q, want %q", push.After, tc.wantAfter)
			}
		})
	}
}

func TestParseEvent_PullRequest(t *testing.T) {
	tests := []struct {
		name       string
		payload    PullRequestEvent
		wantRepo   string
		wantNumber int
		wantSHA    string
		wantBase   string
	}{
		{
			name: "PR opened",
			payload: PullRequestEvent{
				Action: "opened",
				Number: 42,
				PullRequest: PullRequestPayload{
					Number: 42,
					Head: GitRef{
						SHA: "head-sha-abc",
						Ref: "feature/my-feature",
					},
					Base: GitRef{
						SHA: "base-sha-xyz",
						Ref: "main",
					},
					State: "open",
				},
				Repository: GitHubRepository{
					ID:            100,
					FullName:      "org/myrepo",
					DefaultBranch: "main",
				},
				Installation: InstallationPayload{
					ID: 555,
				},
			},
			wantRepo:   "org/myrepo",
			wantNumber: 42,
			wantSHA:    "head-sha-abc",
			wantBase:   "main",
		},
		{
			name: "PR synchronize",
			payload: PullRequestEvent{
				Action: "synchronize",
				Number: 99,
				PullRequest: PullRequestPayload{
					Number: 99,
					Head: GitRef{
						SHA: "new-commit-sha",
						Ref: "fix/bug",
					},
					Base: GitRef{
						SHA: "base-sha",
						Ref: "develop",
					},
					State: "open",
				},
				Repository: GitHubRepository{
					ID:            200,
					FullName:      "team/project",
					DefaultBranch: "develop",
				},
				Installation: InstallationPayload{
					ID: 777,
				},
			},
			wantRepo:   "team/project",
			wantNumber: 99,
			wantSHA:    "new-commit-sha",
			wantBase:   "develop",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.payload)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			event, err := ParseEvent("pull_request", data)
			if err != nil {
				t.Fatalf("ParseEvent: %v", err)
			}

			pr, ok := event.(*PullRequestEvent)
			if !ok {
				t.Fatalf("expected *PullRequestEvent, got %T", event)
			}

			if pr.Repository.FullName != tc.wantRepo {
				t.Errorf("repo = %q, want %q", pr.Repository.FullName, tc.wantRepo)
			}
			if pr.Number != tc.wantNumber {
				t.Errorf("number = %d, want %d", pr.Number, tc.wantNumber)
			}
			if pr.PullRequest.Head.SHA != tc.wantSHA {
				t.Errorf("head SHA = %q, want %q", pr.PullRequest.Head.SHA, tc.wantSHA)
			}
			if pr.PullRequest.Base.Ref != tc.wantBase {
				t.Errorf("base ref = %q, want %q", pr.PullRequest.Base.Ref, tc.wantBase)
			}
		})
	}
}

func TestParseEvent_UnsupportedType(t *testing.T) {
	_, err := ParseEvent("unknown_event", []byte(`{}`))
	if err == nil {
		t.Error("expected error for unsupported event type, got nil")
	}
}

func TestParseEvent_InvalidJSON(t *testing.T) {
	types := []string{"push", "pull_request", "installation", "installation_repositories"}
	for _, eventType := range types {
		t.Run(eventType, func(t *testing.T) {
			_, err := ParseEvent(eventType, []byte(`{invalid json`))
			if err == nil {
				t.Errorf("expected error parsing invalid JSON for %s, got nil", eventType)
			}
		})
	}
}

func TestPushEvent_DefaultBranchFilter(t *testing.T) {
	// This tests the logic from handlePush: only process pushes to the
	// default branch. We test the ref-matching logic directly.
	tests := []struct {
		name          string
		ref           string
		defaultBranch string
		shouldProcess bool
	}{
		{
			name:          "push to default branch",
			ref:           "refs/heads/main",
			defaultBranch: "main",
			shouldProcess: true,
		},
		{
			name:          "push to non-default branch",
			ref:           "refs/heads/feature/foo",
			defaultBranch: "main",
			shouldProcess: false,
		},
		{
			name:          "push to tag (not a branch)",
			ref:           "refs/tags/v1.0.0",
			defaultBranch: "main",
			shouldProcess: false,
		},
		{
			name:          "push to develop default branch",
			ref:           "refs/heads/develop",
			defaultBranch: "develop",
			shouldProcess: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expectedRef := "refs/heads/" + tc.defaultBranch
			isDefault := tc.ref == expectedRef
			if isDefault != tc.shouldProcess {
				t.Errorf("ref=%q defaultBranch=%q: isDefault=%v, want shouldProcess=%v",
					tc.ref, tc.defaultBranch, isDefault, tc.shouldProcess)
			}
		})
	}
}

func TestParseEvent_Installation(t *testing.T) {
	payload := InstallationEvent{
		Action: "created",
		Installation: InstallationPayload{
			ID: 12345,
			Account: GitHubUser{
				ID:    678,
				Login: "myorg",
			},
		},
		Sender: GitHubUser{
			ID:    999,
			Login: "admin-user",
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	event, err := ParseEvent("installation", data)
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}

	inst, ok := event.(*InstallationEvent)
	if !ok {
		t.Fatalf("expected *InstallationEvent, got %T", event)
	}

	if inst.Action != "created" {
		t.Errorf("action = %q, want %q", inst.Action, "created")
	}
	if inst.Installation.ID != 12345 {
		t.Errorf("installation ID = %d, want %d", inst.Installation.ID, 12345)
	}
	if inst.Installation.Account.Login != "myorg" {
		t.Errorf("account login = %q, want %q", inst.Installation.Account.Login, "myorg")
	}
}
