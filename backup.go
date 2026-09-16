package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// filterAnthropicEnv extracts only keys starting with ANTHROPIC_ from an env map.
func filterAnthropicEnv(env map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for key, value := range env {
		if strings.HasPrefix(key, "ANTHROPIC_") {
			result[key] = value
		}
	}
	return result
}

// saveProfile writes a profile to ~/.claude/ccs/<name>.json.
// Prints overwrite warning if file already exists.
func saveProfile(ccsDir string, name string, profile *Profile) error {
	if err := validateProfileName(name); err != nil {
		return err
	}
	profilePath := filepath.Join(ccsDir, name+".json")
	if fileExists(profilePath) {
		fmt.Printf("Warning: Overwriting existing profile '%s'\n", name)
	}
	return writeProfileFile(ccsDir, name, profile)
}

func writeProfileFile(ccsDir string, name string, profile *Profile) error {
	if err := validateProfileName(name); err != nil {
		return err
	}
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal profile: %w", err)
	}
	if err := os.WriteFile(filepath.Join(ccsDir, name+".json"), data, 0644); err != nil {
		return fmt.Errorf("failed to write profile: %w", err)
	}
	return nil
}

func restoreProfileFile(ccsDir, name string, old []byte, existed bool) error {
	path := filepath.Join(ccsDir, name+".json")
	if !existed {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to roll back account profile")
		}
		return nil
	}
	if err := os.WriteFile(path, old, 0644); err != nil {
		return fmt.Errorf("failed to roll back account profile")
	}
	return nil
}

// backupProviderCmd saves current ANTHROPIC_* env vars to a named profile.
func backupProviderCmd(claudeDir string, ccsDir string, name string) {
	settingsPath := filepath.Join(claudeDir, settingsFile)
	settings, err := loadSettings(settingsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to load settings: %v\n", err)
		os.Exit(1)
	}

	if settings.Env == nil {
		fmt.Fprintf(os.Stderr, "Error: No ANTHROPIC_* env vars found\n")
		os.Exit(1)
	}

	anthropicEnv := filterAnthropicEnv(settings.Env)
	if len(anthropicEnv) == 0 {
		fmt.Fprintf(os.Stderr, "Error: No ANTHROPIC_* env vars found\n")
		os.Exit(1)
	}

	profile := &Profile{Env: anthropicEnv}
	if err := saveProfile(ccsDir, name, profile); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Saved provider '%s' to ~/.claude/ccs/%s.json\n", name, name)
}

// backupAccount saves account metadata and its active OAuth credential as one
// coordinated operation. The credential snapshot is never written to the
// normal profile file.
func backupAccount(ccsDir string, name string, activeStore ClaudeCredentialStore, profileStore ProfileCredentialStore) error {
	if err := validateProfileName(name); err != nil {
		return err
	}
	claudeJsonPath := claudeJsonPathForClaudeDir(filepath.Dir(ccsDir))
	cj, err := loadClaudeJson(claudeJsonPath)
	if err != nil {
		return fmt.Errorf("failed to load ~/.claude.json")
	}
	if cj.OAuthAccount == nil {
		return fmt.Errorf("no OAuth account found in ~/.claude.json")
	}

	container, err := activeStore.Load()
	if err != nil {
		return err
	}
	if container == nil || container.ClaudeAIOAuth == nil {
		return fmt.Errorf("Claude OAuth credential is missing")
	}
	credential := container.ClaudeAIOAuth
	if credential.AccessToken == "" && credential.RefreshToken == "" {
		return fmt.Errorf("Claude OAuth credential has no access or refresh token")
	}

	profilePath := filepath.Join(ccsDir, name+".json")
	oldMetadata, metadataExisted := []byte(nil), false
	if data, readErr := os.ReadFile(profilePath); readErr == nil {
		oldMetadata, metadataExisted = data, true
	} else if !os.IsNotExist(readErr) {
		return fmt.Errorf("failed to read existing account profile")
	}

	var oldCredential *ClaudeAIOAuthCredential
	credentialExisted := profileStore.Exists(name)
	if credentialExisted {
		oldCredential, err = profileStore.Load(name)
		if err != nil {
			return fmt.Errorf("existing OAuth credential snapshot is malformed")
		}
	}

	rollbackCredential := func() error {
		if credentialExisted {
			return profileStore.Save(name, oldCredential)
		}
		return deleteProfileCredential(profileStore, name)
	}

	if err := profileStore.Save(name, credential); err != nil {
		_ = rollbackCredential()
		return err
	}
	profile := &Profile{OAuthAccount: cj.OAuthAccount}
	if err := writeProfileFile(ccsDir, name, profile); err != nil {
		credentialRollbackErr := rollbackCredential()
		metadataRollbackErr := restoreProfileFile(ccsDir, name, oldMetadata, metadataExisted)
		if credentialRollbackErr != nil || metadataRollbackErr != nil {
			return fmt.Errorf("failed to save account profile (rollback failed)")
		}
		return err
	}
	return nil
}

// backupAccountCmd retains the CLI's exit-code behavior around the testable
// backupAccount operation.
func backupAccountCmd(ccsDir string, name string) {
	activeStore, err := newClaudeCredentialStore(filepath.Dir(ccsDir))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if err := validateProfileName(name); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	profileStore := NewProfileCredentialStore(ccsDir)
	if fileExists(filepath.Join(ccsDir, name+".json")) {
		fmt.Printf("Warning: Overwriting existing profile '%s'\n", name)
	}
	if err := backupAccount(ccsDir, name, activeStore, profileStore); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Saved account '%s' to ~/.claude/ccs/%s.json\n", name, name)
}
