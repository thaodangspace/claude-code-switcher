package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestProfileCredentialStoreRoundTripAndOverwrite(t *testing.T) {
	store := NewFileProfileCredentialStore(t.TempDir())
	first := &ClaudeAIOAuthCredential{AccessToken: "old-secret", ExpiresAt: 100, Extra: map[string]json.RawMessage{}}
	if err := store.Save("personal", first); err != nil {
		t.Fatal(err)
	}
	second := &ClaudeAIOAuthCredential{RefreshToken: "new-secret", RefreshTokenExpiresAt: 200}
	if err := store.Save("personal", second); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load("personal")
	if err != nil {
		t.Fatal(err)
	}
	if got.RefreshToken != second.RefreshToken || got.AccessToken != "" {
		t.Fatalf("overwrite did not replace snapshot: %#v", got)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(store.Dir, "personal.json"))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("snapshot mode = %o, want 600", info.Mode().Perm())
		}
		dirInfo, err := os.Stat(store.Dir)
		if err != nil {
			t.Fatal(err)
		}
		if dirInfo.Mode().Perm() != 0700 {
			t.Fatalf("snapshot directory mode = %o, want 700", dirInfo.Mode().Perm())
		}
	}
}

func TestProfileCredentialStoreCorruptSnapshotIsSafe(t *testing.T) {
	store := NewFileProfileCredentialStore(t.TempDir())
	if err := os.MkdirAll(store.Dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store.Dir, "broken.json"), []byte(`{"accessToken":"do-not-leak",`), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := store.Load("broken")
	if err == nil || strings.Contains(err.Error(), "do-not-leak") {
		t.Fatalf("unsafe corrupt snapshot error: %v", err)
	}
}

func TestValidateProfileName(t *testing.T) {
	for _, name := range []string{"", ".", "..", "a/b", `a\b`, "a\x00b"} {
		if err := validateProfileName(name); err == nil {
			t.Errorf("validateProfileName(%q) accepted unsafe name", name)
		}
	}
	if err := validateProfileName("work-account"); err != nil {
		t.Fatal(err)
	}
}
