package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type memoryCredentialStore struct {
	container *CredentialContainer
	failSaves int
}

func (s *memoryCredentialStore) Load() (*CredentialContainer, error) {
	return cloneCredentialContainer(s.container)
}

func (s *memoryCredentialStore) Save(container *CredentialContainer) error {
	if s.failSaves > 0 {
		s.failSaves--
		return errors.New("injected save failure")
	}
	clone, err := cloneCredentialContainer(container)
	if err != nil {
		return err
	}
	s.container = clone
	return nil
}

type memoryProfileCredentialStore struct {
	credentials map[string]*ClaudeAIOAuthCredential
	failSave    bool
}

func (s *memoryProfileCredentialStore) Load(name string) (*ClaudeAIOAuthCredential, error) {
	credential, ok := s.credentials[name]
	if !ok {
		return nil, errors.New("not found")
	}
	data, err := json.Marshal(credential)
	if err != nil {
		return nil, err
	}
	var clone ClaudeAIOAuthCredential
	if err := json.Unmarshal(data, &clone); err != nil {
		return nil, err
	}
	return &clone, nil
}

func (s *memoryProfileCredentialStore) Save(name string, credential *ClaudeAIOAuthCredential) error {
	if s.failSave {
		return errors.New("injected snapshot failure")
	}
	if credential == nil {
		delete(s.credentials, name)
		return nil
	}
	loaded, err := s.LoadValue(credential)
	if err != nil {
		return err
	}
	s.credentials[name] = loaded
	return nil
}

func (s *memoryProfileCredentialStore) LoadValue(credential *ClaudeAIOAuthCredential) (*ClaudeAIOAuthCredential, error) {
	data, err := json.Marshal(credential)
	if err != nil {
		return nil, err
	}
	var clone ClaudeAIOAuthCredential
	if err := json.Unmarshal(data, &clone); err != nil {
		return nil, err
	}
	return &clone, nil
}

func (s *memoryProfileCredentialStore) Exists(name string) bool {
	_, ok := s.credentials[name]
	return ok
}

func writeAccountFixture(t *testing.T, ccsDir string, account OAuthAccount, env map[string]interface{}) {
	t.Helper()
	if err := os.MkdirAll(ccsDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := saveClaudeJson(filepath.Join(filepath.Dir(filepath.Dir(ccsDir)), ".claude.json"), &ClaudeJson{OAuthAccount: account, Extra: map[string]interface{}{"other": true}}); err != nil {
		t.Fatal(err)
	}
	if err := saveSettings(filepath.Join(filepath.Dir(ccsDir), settingsFile), &Settings{Env: env}); err != nil {
		t.Fatal(err)
	}
}

func TestBackupAccountSavesMetadataAndCredentialSeparately(t *testing.T) {
	root := t.TempDir()
	ccsDir := filepath.Join(root, ".claude", "ccs")
	account := OAuthAccount{"accountUuid": "a", "emailAddress": "a@example.com"}
	writeAccountFixture(t, ccsDir, account, nil)
	active := &memoryCredentialStore{container: &CredentialContainer{ClaudeAIOAuth: &ClaudeAIOAuthCredential{AccessToken: "access-secret", RefreshToken: "refresh-secret", ExpiresAt: 100}}}
	profiles := NewFileProfileCredentialStore(ccsDir)
	if err := backupAccount(ccsDir, "account-a", active, profiles); err != nil {
		t.Fatal(err)
	}
	profileData, err := os.ReadFile(filepath.Join(ccsDir, "account-a.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(profileData), "access-secret") || strings.Contains(string(profileData), "refresh-secret") {
		t.Fatal("normal profile contains OAuth secret")
	}
	snapshot, err := profiles.Load("account-a")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.AccessToken != "access-secret" || snapshot.RefreshToken != "refresh-secret" {
		t.Fatalf("snapshot missing credential: %#v", snapshot)
	}
}

func TestBackupAccountRequiresActiveOAuthCredential(t *testing.T) {
	root := t.TempDir()
	ccsDir := filepath.Join(root, ".claude", "ccs")
	writeAccountFixture(t, ccsDir, OAuthAccount{"emailAddress": "a@example.com"}, nil)
	active := &memoryCredentialStore{container: &CredentialContainer{Extra: map[string]json.RawMessage{"mcpOAuth": json.RawMessage(`{}`)}}}
	err := backupAccount(ccsDir, "account-a", active, NewFileProfileCredentialStore(ccsDir))
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("unexpected backup error: %v", err)
	}
}

func TestSwitchAccountRestoresOAuthAndPreservesUnrelatedCredentialData(t *testing.T) {
	root := t.TempDir()
	ccsDir := filepath.Join(root, ".claude", "ccs")
	current := OAuthAccount{"accountUuid": "a", "emailAddress": "a@example.com", "displayName": "same"}
	target := OAuthAccount{"accountUuid": "b", "emailAddress": "b@example.com", "displayName": "same"}
	writeAccountFixture(t, ccsDir, current, map[string]interface{}{"ANTHROPIC_API_KEY": "provider-secret"})
	if err := writeProfileFile(ccsDir, "account-a", &Profile{OAuthAccount: current}); err != nil {
		t.Fatal(err)
	}
	if err := writeProfileFile(ccsDir, "account-b", &Profile{OAuthAccount: target}); err != nil {
		t.Fatal(err)
	}
	profiles := &memoryProfileCredentialStore{credentials: map[string]*ClaudeAIOAuthCredential{
		"account-a": {AccessToken: "rotated-access", RefreshToken: "rotated-refresh", ExpiresAt: 200},
		"account-b": {AccessToken: "target-access", RefreshToken: "target-refresh", ExpiresAt: 300},
	}}
	active := &memoryCredentialStore{container: &CredentialContainer{
		ClaudeAIOAuth: &ClaudeAIOAuthCredential{AccessToken: "latest-access", RefreshToken: "latest-refresh", ExpiresAt: 250},
		Extra: map[string]json.RawMessage{
			"mcpOAuth":         json.RawMessage(`{"grant":{"accessToken":"mcp-secret"}}`),
			"futureUnknownKey": json.RawMessage(`{"value":1}`),
		},
	}}
	profile, err := loadProfile("account-b", ccsDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := switchAccountProfile("account-b", profile, filepath.Join(root, ".claude"), ccsDir, active, profiles); err != nil {
		t.Fatal(err)
	}
	if active.container.ClaudeAIOAuth.AccessToken != "target-access" {
		t.Fatalf("target OAuth was not restored: %#v", active.container.ClaudeAIOAuth)
	}
	if string(active.container.Extra["mcpOAuth"]) != `{"grant":{"accessToken":"mcp-secret"}}` || string(active.container.Extra["futureUnknownKey"]) != `{"value":1}` {
		t.Fatal("unrelated credential data changed")
	}
	rotated, err := profiles.Load("account-a")
	if err != nil {
		t.Fatal(err)
	}
	if rotated.AccessToken != "latest-access" {
		t.Fatal("rotated current credential was not saved")
	}
	cj, err := loadClaudeJson(filepath.Join(root, ".claude.json"))
	if err != nil || toString(cj.OAuthAccount["accountUuid"]) != "b" {
		t.Fatalf("target account metadata was not restored: %v", err)
	}
	settings, err := loadSettings(filepath.Join(root, ".claude", settingsFile))
	if err != nil || settings.Env != nil {
		t.Fatalf("provider env was not cleared: %v", err)
	}
}

func TestSwitchAccountRefusesLegacyProfile(t *testing.T) {
	root := t.TempDir()
	ccsDir := filepath.Join(root, ".claude", "ccs")
	account := OAuthAccount{"accountUuid": "a", "emailAddress": "a@example.com"}
	writeAccountFixture(t, ccsDir, account, nil)
	if err := writeProfileFile(ccsDir, "legacy", &Profile{OAuthAccount: account}); err != nil {
		t.Fatal(err)
	}
	profile, _ := loadProfile("legacy", ccsDir)
	err := switchAccountProfile("legacy", profile, filepath.Join(root, ".claude"), ccsDir, &memoryCredentialStore{}, &memoryProfileCredentialStore{credentials: map[string]*ClaudeAIOAuthCredential{}})
	if err == nil || !strings.Contains(err.Error(), "ccs backup-account legacy") || strings.Contains(err.Error(), "access-secret") {
		t.Fatalf("unexpected legacy error: %v", err)
	}
}
