package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCollectProfilesHealthAndActiveMarker(t *testing.T) {
	ccsDir := filepath.Join(t.TempDir(), "ccs")
	if err := os.MkdirAll(ccsDir, 0700); err != nil {
		t.Fatal(err)
	}
	active := OAuthAccount{"accountUuid": "ready-id", "emailAddress": "me@example.com"}
	profiles := []struct {
		name    string
		account OAuthAccount
	}{
		{"ready", active},
		{"legacy", OAuthAccount{"emailAddress": "legacy@example.com"}},
		{"broken", OAuthAccount{"emailAddress": "broken@example.com"}},
	}
	for _, profile := range profiles {
		if err := writeProfileFile(ccsDir, profile.name, &Profile{OAuthAccount: profile.account}); err != nil {
			t.Fatal(err)
		}
	}
	if err := writeProfileFile(ccsDir, "provider", &Profile{Env: map[string]interface{}{"ANTHROPIC_BASE_URL": "https://example.test"}}); err != nil {
		t.Fatal(err)
	}
	store := NewFileProfileCredentialStore(ccsDir)
	now := time.UnixMilli(1_700_000_000_000)
	if err := store.Save("ready", &ClaudeAIOAuthCredential{AccessToken: "access-secret", RefreshToken: "refresh-secret", ExpiresAt: now.Add(time.Hour).UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(store.Dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store.Dir, "broken.json"), []byte(`{"accessToken":"secret",`), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := collectProfiles(ccsDir, active, store, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Providers) != 1 || result.Providers[0].Name != "provider" {
		t.Fatalf("providers = %#v", result.Providers)
	}
	byName := make(map[string]AccountListItem)
	for _, account := range result.Accounts {
		byName[account.Name] = account
	}
	if byName["ready"].Health != TokenReady || !byName["ready"].Active {
		t.Fatalf("ready account = %#v", byName["ready"])
	}
	if byName["legacy"].Health != TokenUnknown || byName["legacy"].Detail != "credential not saved" {
		t.Fatalf("legacy account = %#v", byName["legacy"])
	}
	if byName["broken"].Health != TokenUnknown || byName["broken"].Detail != "credential unreadable" {
		t.Fatalf("broken account = %#v", byName["broken"])
	}
}

func TestProfileListJSONIsSecretFree(t *testing.T) {
	item := ProfileListResult{
		Providers: []ProviderListItem{{Name: "provider"}},
		Accounts:  []AccountListItem{{Name: "work", Email: "work@example.com", Health: TokenReady, Detail: "access token valid"}},
	}
	data, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	output := string(data)
	for _, secret := range []string{"access-secret", "refresh-secret", "accessToken", "refreshToken"} {
		if strings.Contains(output, secret) {
			t.Fatalf("list JSON contains secret field/value %q: %s", secret, output)
		}
	}
}

func TestRenderProfileListDoesNotPrintSecrets(t *testing.T) {
	result := ProfileListResult{
		Accounts: []AccountListItem{{Name: "work", Email: "work@example.com", Health: TokenUnknown, Detail: "credential not saved"}},
	}
	oldStdout := os.Stdout
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writePipe
	renderErr := renderProfileList(result)
	_ = writePipe.Close()
	os.Stdout = oldStdout
	data, _ := io.ReadAll(readPipe)
	_ = readPipe.Close()
	if renderErr != nil {
		t.Fatal(renderErr)
	}
	if bytes.Contains(data, []byte("access-secret")) || bytes.Contains(data, []byte("refresh-secret")) {
		t.Fatalf("human list leaked secret: %s", data)
	}
	if !bytes.Contains(data, []byte("unknown")) {
		t.Fatalf("human list missing health: %s", data)
	}
}
