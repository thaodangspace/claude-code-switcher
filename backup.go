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
	profilePath := filepath.Join(ccsDir, name+".json")

	// Check if profile already exists
	if fileExists(profilePath) {
		fmt.Printf("Warning: Overwriting existing profile '%s'\n", name)
	}

	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal profile: %w", err)
	}

	if err := os.WriteFile(profilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write profile: %w", err)
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

// backupAccountCmd saves current oauthAccount to a named profile.
func backupAccountCmd(ccsDir string, name string) {
	claudeJsonPath, err := getClaudeJsonPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cj, err := loadClaudeJson(claudeJsonPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to load ~/.claude.json: %v\n", err)
		os.Exit(1)
	}

	if cj.OAuthAccount == nil {
		fmt.Fprintf(os.Stderr, "Error: No OAuth account found in ~/.claude.json\n")
		os.Exit(1)
	}

	profile := &Profile{OAuthAccount: cj.OAuthAccount}
	if err := saveProfile(ccsDir, name, profile); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Saved account '%s' to ~/.claude/ccs/%s.json\n", name, name)
}
