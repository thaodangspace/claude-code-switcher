package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// showCurrent displays the active provider or account from the global Claude config.
func showCurrent(claudeDir string, ccsDir string) error {
	settingsPath := filepath.Join(claudeDir, settingsFile)
	settings, err := loadSettings(settingsPath)
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	fmt.Println("Claude Code:")
	if settings.Env != nil {
		provider := detectCurrentProvider(ccsDir, settings.Env)
		if provider != "" {
			fmt.Printf("  %s\n", provider)
			return nil
		}
	}

	claudeJsonPath, err := getClaudeJsonPath()
	if err != nil {
		return fmt.Errorf("failed to get claude.json path: %w", err)
	}
	cj, err := loadClaudeJson(claudeJsonPath)
	if err != nil {
		return fmt.Errorf("failed to load claude.json: %w", err)
	}

	if cj.OAuthAccount == nil {
		fmt.Println("  default")
	} else {
		email := toString(cj.OAuthAccount["emailAddress"])
		name := toString(cj.OAuthAccount["displayName"])
		if email != "" {
			fmt.Printf("  %s (%s)\n", name, email)
		} else {
			fmt.Println("  default")
		}
	}

	return nil
}

// resetCmd clears the active provider env and OAuth account, reverting to defaults.
func resetCmd(claudeDir string) {
	settingsPath := filepath.Join(claudeDir, settingsFile)
	settings, err := loadSettings(settingsPath)
	if err == nil {
		removeEnv(settings)
		saveSettings(settingsPath, settings) //nolint:errcheck
	}

	claudeJsonPath, err := getClaudeJsonPath()
	if err == nil {
		cj, err := loadClaudeJson(claudeJsonPath)
		if err == nil {
			cj.OAuthAccount = nil
			saveClaudeJson(claudeJsonPath, cj) //nolint:errcheck
		}
	}

	fmt.Println("Reset to default provider and account")
}

// accountIdentityMatches compares stable account identifiers. Display names are
// intentionally excluded because they are not unique.
func accountIdentityMatches(a, b OAuthAccount) bool {
	aUUID := toString(a["accountUuid"])
	bUUID := toString(b["accountUuid"])
	if aUUID != "" && bUUID != "" {
		return aUUID == bUUID
	}
	// Email is the compatibility fallback when either record lacks a UUID.
	aEmail := toString(a["emailAddress"])
	bEmail := toString(b["emailAddress"])
	return aEmail != "" && bEmail != "" && aEmail == bEmail
}

// findProfileForOAuthAccount finds the saved profile for an active account,
// using UUID first and email only when a UUID is unavailable.
func findProfileForOAuthAccount(ccsDir string, account OAuthAccount) (string, bool) {
	entries, err := os.ReadDir(ccsDir)
	if err != nil {
		return "", false
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		profile, err := loadProfile(name, ccsDir)
		if err == nil && profile.OAuthAccount != nil && accountIdentityMatches(profile.OAuthAccount, account) {
			return name, true
		}
	}
	return "", false
}

func restoreRotatedSnapshot(store ProfileCredentialStore, name string, existed bool, old *ClaudeAIOAuthCredential) error {
	if existed {
		return store.Save(name, old)
	}
	return deleteProfileCredential(store, name)
}

// switchAccountProfile applies an account profile as a rollback-safe
// transaction, including the matching active Claude OAuth credential.
func switchAccountProfile(name string, profile *Profile, claudeDir string, ccsDir string, activeStore ClaudeCredentialStore, profileStore ProfileCredentialStore) error {
	if activeStore == nil || profileStore == nil {
		return fmt.Errorf("credential stores are unavailable")
	}
	if profile == nil || profile.OAuthAccount == nil {
		return fmt.Errorf("profile is not an account profile")
	}
	if !profileStore.Exists(name) {
		return fmt.Errorf("Account profile '%s' has no saved OAuth credential. Log in to that account and run: ccs backup-account %s", name, name)
	}
	targetCredential, err := profileStore.Load(name)
	if err != nil || targetCredential == nil {
		return fmt.Errorf("saved OAuth credential for '%s' is malformed", name)
	}

	claudeJsonPath := claudeJsonPathForClaudeDir(claudeDir)
	cj, err := loadClaudeJson(claudeJsonPath)
	if err != nil {
		return fmt.Errorf("failed to load ~/.claude.json")
	}
	settingsPath := filepath.Join(claudeDir, settingsFile)
	settings, err := loadSettings(settingsPath)
	if err != nil {
		return fmt.Errorf("failed to load settings")
	}
	activeContainer, err := activeStore.Load()
	if err != nil {
		return err
	}
	if activeContainer == nil || activeContainer.ClaudeAIOAuth == nil {
		return fmt.Errorf("Claude OAuth credential is missing")
	}

	// Save any rotated credential for the account we are leaving before
	// applying the target. The old snapshot is retained for transaction rollback.
	currentProfileName, hasCurrentProfile := findProfileForOAuthAccount(ccsDir, cj.OAuthAccount)
	rotatedSnapshotExisted := false
	var rotatedSnapshotOld *ClaudeAIOAuthCredential
	if hasCurrentProfile {
		rotatedSnapshotExisted = profileStore.Exists(currentProfileName)
		if rotatedSnapshotExisted {
			rotatedSnapshotOld, err = profileStore.Load(currentProfileName)
			if err != nil {
				return fmt.Errorf("current account OAuth credential snapshot is malformed")
			}
		}
		if err := profileStore.Save(currentProfileName, activeContainer.ClaudeAIOAuth); err != nil {
			_ = restoreRotatedSnapshot(profileStore, currentProfileName, rotatedSnapshotExisted, rotatedSnapshotOld)
			return fmt.Errorf("failed to save current account OAuth credential")
		}
	}

	// If switching to the currently active account, use the just-saved current
	// credential rather than restoring a stale pre-rotation snapshot.
	if hasCurrentProfile && currentProfileName == name {
		targetCredential = activeContainer.ClaudeAIOAuth
	}

	originalContainer, err := cloneCredentialContainer(activeContainer)
	if err != nil {
		return fmt.Errorf("failed to retain current Claude credential")
	}
	originalAccount := cj.OAuthAccount
	originalEnv := settings.Env
	originalSettings := &Settings{Permissions: settings.Permissions, Model: settings.Model, StatusLine: settings.StatusLine, EnabledPlugins: settings.EnabledPlugins, Env: originalEnv, Extra: settings.Extra}
	originalCJ := &ClaudeJson{OAuthAccount: originalAccount, Extra: cj.Extra}

	activeContainer.ReplaceClaudeAIOAuth(targetCredential)
	cj.OAuthAccount = profile.OAuthAccount
	removeEnv(settings)
	if profile.Env != nil {
		mergeEnv(settings, profile.Env)
	}
	return persistAccountSwitch(activeStore, profileStore, claudeJsonPath, settingsPath, activeContainer, originalContainer, cj, originalCJ, settings, originalSettings, hasCurrentProfile, currentProfileName, rotatedSnapshotExisted, rotatedSnapshotOld)
}

func cloneCredentialContainer(container *CredentialContainer) (*CredentialContainer, error) {
	data, err := EncodeCredentialContainer(container)
	if err != nil {
		return nil, err
	}
	return DecodeCredentialContainer(data)
}

// persistAccountSwitch performs the writes after all account preconditions
// have passed and restores every already-written document on failure.
func persistAccountSwitch(activeStore ClaudeCredentialStore, profileStore ProfileCredentialStore, claudeJsonPath, settingsPath string, container, originalContainer *CredentialContainer, cj, originalCJ *ClaudeJson, settings, originalSettings *Settings, hasCurrentProfile bool, currentProfileName string, rotatedExisted bool, rotatedOld *ClaudeAIOAuthCredential) error {
	restore := func() {
		_ = activeStore.Save(originalContainer)
		_ = saveClaudeJson(claudeJsonPath, originalCJ)
		_ = saveSettings(settingsPath, originalSettings)
		if hasCurrentProfile {
			_ = restoreRotatedSnapshot(profileStore, currentProfileName, rotatedExisted, rotatedOld)
		}
	}

	if err := activeStore.Save(container); err != nil {
		restore()
		return fmt.Errorf("failed to save Claude OAuth credential")
	}
	if err := saveClaudeJson(claudeJsonPath, cj); err != nil {
		restore()
		return fmt.Errorf("failed to save ~/.claude.json")
	}
	if err := saveSettings(settingsPath, settings); err != nil {
		restore()
		return fmt.Errorf("failed to save settings")
	}
	return nil
}

// switchProfile applies the named profile to the global Claude configuration.
func switchProfile(name string, claudeDir string, ccsDir string) {
	profile, err := loadProfile(name, ccsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Profile '%s' not found or invalid: %v\n", name, err)
		os.Exit(1)
	}

	if profile.OAuthAccount != nil {
		activeStore, err := newClaudeCredentialStore(claudeDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		profileStore := NewProfileCredentialStore(ccsDir)
		if err := switchAccountProfile(name, profile, claudeDir, ccsDir, activeStore, profileStore); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Switched to account '%s'\n", name)
		return
	}

	// Provider switching retains the original behavior and does not touch the
	// active OAuth credential store.
	claudeJsonPath := claudeJsonPathForClaudeDir(claudeDir)
	cj, err := loadClaudeJson(claudeJsonPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading ~/.claude.json: %v\n", err)
		os.Exit(1)
	}
	cj.OAuthAccount = profile.OAuthAccount
	if err := saveClaudeJson(claudeJsonPath, cj); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving ~/.claude.json: %v\n", err)
		os.Exit(1)
	}

	settingsPath := filepath.Join(claudeDir, settingsFile)
	settings, err := loadSettings(settingsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading settings: %v\n", err)
		os.Exit(1)
	}
	removeEnv(settings)
	if profile.Env != nil {
		mergeEnv(settings, profile.Env)
	}
	if err := saveSettings(settingsPath, settings); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving settings: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Switched to provider '%s'\n", name)
}
