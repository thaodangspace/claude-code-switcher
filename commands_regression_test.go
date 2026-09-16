package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAccountIdentityMatchingNeverUsesDisplayName(t *testing.T) {
	if accountIdentityMatches(OAuthAccount{"displayName": "same"}, OAuthAccount{"displayName": "same"}) {
		t.Fatal("display name was used as account identity")
	}
	if !accountIdentityMatches(OAuthAccount{"accountUuid": "a", "emailAddress": "same@example.com"}, OAuthAccount{"emailAddress": "same@example.com"}) {
		t.Fatal("email fallback was not used when one UUID was unavailable")
	}
	if !accountIdentityMatches(OAuthAccount{"emailAddress": "same@example.com"}, OAuthAccount{"emailAddress": "same@example.com"}) {
		t.Fatal("email fallback was not used when UUIDs were unavailable")
	}
}

func TestSwitchAccountRollsBackPersistedStateOnSettingsFailure(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	ccsDir := filepath.Join(claudeDir, "ccs")
	current := OAuthAccount{"accountUuid": "a", "emailAddress": "a@example.com"}
	target := OAuthAccount{"accountUuid": "b", "emailAddress": "b@example.com"}
	writeAccountFixture(t, ccsDir, current, map[string]interface{}{"ANTHROPIC_API_KEY": "provider-secret"})
	if err := writeProfileFile(ccsDir, "a", &Profile{OAuthAccount: current}); err != nil {
		t.Fatal(err)
	}
	if err := writeProfileFile(ccsDir, "b", &Profile{OAuthAccount: target}); err != nil {
		t.Fatal(err)
	}
	profiles := &memoryProfileCredentialStore{credentials: map[string]*ClaudeAIOAuthCredential{
		"a": {AccessToken: "saved-a", RefreshToken: "saved-ra", ExpiresAt: 100},
		"b": {AccessToken: "saved-b", RefreshToken: "saved-rb", ExpiresAt: 200},
	}}
	active := &memoryCredentialStore{container: &CredentialContainer{
		ClaudeAIOAuth: &ClaudeAIOAuthCredential{AccessToken: "active-a", RefreshToken: "active-ra", ExpiresAt: 300},
		Extra:         map[string]json.RawMessage{"mcpOAuth": json.RawMessage(`{"grant":true}`)},
	}}
	settingsPath := filepath.Join(claudeDir, settingsFile)
	active.onFirstSave = func() {
		_ = os.Remove(settingsPath)
		_ = os.Mkdir(settingsPath, 0700)
	}
	profile, err := loadProfile("b", ccsDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := switchAccountProfile("b", profile, claudeDir, ccsDir, active, profiles); err == nil {
		t.Fatal("switch unexpectedly succeeded")
	}
	if active.container.ClaudeAIOAuth.AccessToken != "active-a" {
		t.Fatal("active OAuth credential was not rolled back")
	}
	cj, err := loadClaudeJson(filepath.Join(root, ".claude.json"))
	if err != nil || toString(cj.OAuthAccount["accountUuid"]) != "a" {
		t.Fatalf("account metadata was not rolled back: %v", err)
	}
}
