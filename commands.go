package main

import (
	"fmt"
	"os"
	"path/filepath"
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

// switchProfile applies the named profile to the global Claude configuration.
func switchProfile(name string, claudeDir string, ccsDir string) {
	profile, err := loadProfile(name, ccsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Profile '%s' not found or invalid: %v\n", name, err)
		os.Exit(1)
	}

	claudeJsonPath, err := getClaudeJsonPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
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

	if profile.OAuthAccount != nil {
		fmt.Printf("Switched to account '%s'\n", name)
	} else if profile.Env != nil {
		fmt.Printf("Switched to provider '%s'\n", name)
	}
}
