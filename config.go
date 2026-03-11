package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
)

const (
	claudeDir    = ".claude"
	settingsFile = "settings.json"
)

// getClaudeDir returns the ~/.claude directory path.
func getClaudeDir() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}
	return filepath.Join(usr.HomeDir, claudeDir), nil
}

// getClaudeJsonPath returns the ~/.claude.json file path.
func getClaudeJsonPath() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}
	return filepath.Join(usr.HomeDir, ".claude.json"), nil
}

// getCcsDir returns the ~/.claude/ccs directory path, creating it if needed.
func getCcsDir(claudeDir string) (string, error) {
	ccsDir := filepath.Join(claudeDir, "ccs")
	if err := os.MkdirAll(ccsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create ccs directory: %w", err)
	}
	return ccsDir, nil
}

// loadClaudeJson reads and parses ~/.claude.json.
func loadClaudeJson(path string) (*ClaudeJson, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	var cj ClaudeJson
	if err := json.Unmarshal(data, &cj); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return &cj, nil
}

// saveClaudeJson writes ~/.claude.json with proper JSON formatting.
func saveClaudeJson(path string, cj *ClaudeJson) error {
	data, err := json.MarshalIndent(cj, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal claude.json: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

// loadSettings reads and parses settings.json.
func loadSettings(path string) (*Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read settings file: %w", err)
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("failed to parse settings JSON: %w", err)
	}

	return &settings, nil
}

// saveSettings writes settings back to settings.json with proper JSON formatting.
func saveSettings(path string, settings *Settings) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write settings file: %w", err)
	}

	return nil
}

// removeEnv removes the env key from settings.
func removeEnv(settings *Settings) {
	settings.Env = nil
}

// mergeEnv merges provider env into settings.
func mergeEnv(settings *Settings, providerEnv map[string]interface{}) {
	if settings.Env == nil {
		settings.Env = make(map[string]interface{})
	}
	for k, v := range providerEnv {
		settings.Env[k] = v
	}
}
