package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCredentialContainerPreservesUnrelatedFields(t *testing.T) {
	data := []byte(`{"claudeAiOauth":{"accessToken":"access-secret","refreshToken":"refresh-secret","expiresAt":123,"futureOauth":{"x":1}},"mcpOAuth":{"example":{"accessToken":"mcp-secret"}},"futureUnknownKey":{"x":1}}`)
	container, err := DecodeCredentialContainer(data)
	if err != nil {
		t.Fatal(err)
	}
	container.ReplaceClaudeAIOAuth(&ClaudeAIOAuthCredential{AccessToken: "new-access", ExpiresAt: 456})
	encoded, err := EncodeCredentialContainer(container)
	if err != nil {
		t.Fatal(err)
	}
	var got, want map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"mcpOAuth", "futureUnknownKey"} {
		if string(got[key]) != string(want[key]) {
			t.Fatalf("%s changed: got %s want %s", key, got[key], want[key])
		}
	}
	if string(got["claudeAiOauth"]) == string(want["claudeAiOauth"]) {
		t.Fatal("OAuth subtree was not replaced")
	}
	if string(encoded) == "" {
		t.Fatal("empty encoded container")
	}
}

func TestClaudeAIOAuthCredentialPreservesUnknownFields(t *testing.T) {
	input := []byte(`{"accessToken":"access-secret","expiresAt":123,"future":"kept"}`)
	var credential ClaudeAIOAuthCredential
	if err := json.Unmarshal(input, &credential); err != nil {
		t.Fatal(err)
	}
	output, err := json.Marshal(&credential)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(output, &got); err != nil {
		t.Fatal(err)
	}
	if string(got["future"]) != `"kept"` {
		t.Fatalf("unknown OAuth field was not preserved: %s", got["future"])
	}
}

func TestFileClaudeCredentialStoreRoundTripAndMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".claude", ".credentials.json")
	store := NewFileClaudeCredentialStore(path)
	want := &CredentialContainer{
		ClaudeAIOAuth: &ClaudeAIOAuthCredential{AccessToken: "access-secret", ExpiresAt: 123},
		Extra:         map[string]json.RawMessage{"mcpOAuth": json.RawMessage(`{"grant":true}`)},
	}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.ClaudeAIOAuth.AccessToken != want.ClaudeAIOAuth.AccessToken || got.ClaudeAIOAuth.ExpiresAt != 123 {
		t.Fatalf("round trip mismatch: %#v", got.ClaudeAIOAuth)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("credential mode = %o, want 600", info.Mode().Perm())
		}
	}
}
