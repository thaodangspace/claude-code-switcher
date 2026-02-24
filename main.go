package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
)

const (
	claudeDir    = ".claude"
	settingsFile = "settings.json"
)

// Settings represents the Claude settings.json structure
// Uses custom JSON marshaling to preserve all unknown fields
type Settings struct {
	Permissions    map[string]interface{} `json:"permissions,omitempty"`
	Model          string                 `json:"model,omitempty"`
	StatusLine     map[string]interface{} `json:"statusLine,omitempty"`
	EnabledPlugins map[string]interface{} `json:"enabledPlugins,omitempty"`
	Env            map[string]interface{} `json:"env,omitempty"`

	// Extra captures any unknown fields to preserve them
	Extra map[string]interface{} `json:"-"`
}

// UnmarshalJSON handles custom unmarshaling to preserve unknown fields
func (s *Settings) UnmarshalJSON(data []byte) error {
	// First unmarshal into a raw map to capture all fields
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Initialize Extra map if needed
	s.Extra = make(map[string]interface{})

	// Map of known field names to their corresponding field handling
	knownFields := map[string]func(interface{}){
		"permissions":     func(v interface{}) { s.Permissions = toMap(v) },
		"model":           func(v interface{}) { s.Model = toString(v) },
		"statusLine":      func(v interface{}) { s.StatusLine = toMap(v) },
		"enabledPlugins":  func(v interface{}) { s.EnabledPlugins = toMap(v) },
		"env":             func(v interface{}) { s.Env = toMap(v) },
	}

	// Process known fields, store unknown fields in Extra
	for key, value := range raw {
		if handler, known := knownFields[key]; known {
			handler(value)
		} else {
			s.Extra[key] = value
		}
	}

	return nil
}

// MarshalJSON handles custom marshaling to include all fields including Extras
func (s *Settings) MarshalJSON() ([]byte, error) {
	result := make(map[string]interface{})

	if s.Permissions != nil {
		result["permissions"] = s.Permissions
	}
	if s.Model != "" {
		result["model"] = s.Model
	}
	if s.StatusLine != nil {
		result["statusLine"] = s.StatusLine
	}
	if s.EnabledPlugins != nil {
		result["enabledPlugins"] = s.EnabledPlugins
	}
	if s.Env != nil {
		result["env"] = s.Env
	}

	// Add all extra fields
	for key, value := range s.Extra {
		result[key] = value
	}

	return json.Marshal(result)
}

// Helper functions
func toMap(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return nil
}

func toString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// EnvConfig represents the provider-specific env config
type EnvConfig struct {
	Env map[string]interface{} `json:"env"`
}

// getClaudeDir returns the ~/.claude directory path
func getClaudeDir() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}
	return filepath.Join(usr.HomeDir, claudeDir), nil
}

// loadSettings reads and parses settings.json
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

// saveSettings writes settings back to settings.json with proper JSON formatting
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

// loadProviderEnv reads env from ~/.claude/{provider}.json
func loadProviderEnv(provider string, claudeDir string) (map[string]interface{}, error) {
	providerPath := filepath.Join(claudeDir, provider+".json")
	data, err := os.ReadFile(providerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read provider config %s: %w", providerPath, err)
	}

	var config EnvConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse provider config: %w", err)
	}

	if config.Env == nil {
		return nil, fmt.Errorf("provider config %s is missing 'env' key", providerPath)
	}

	return config.Env, nil
}

// removeEnv removes the env key from settings
func removeEnv(settings *Settings) {
	settings.Env = nil
}

// mergeEnv merges provider env into settings
func mergeEnv(settings *Settings, providerEnv map[string]interface{}) {
	if settings.Env == nil {
		settings.Env = make(map[string]interface{})
	}
	for k, v := range providerEnv {
		settings.Env[k] = v
	}
}

// printUsage prints usage information
func printUsage() {
	fmt.Println("Claude Code Switcher (ccs)")
	fmt.Println("\nUsage:")
	fmt.Println("  ccs          Reset to default (remove env key)")
	fmt.Println("  ccs <name>   Switch to provider (merge env from <name>.json)")
	fmt.Println("\nExamples:")
	fmt.Println("  ccs glm      Switch to glm provider")
	fmt.Println("  ccs          Reset to default")
	fmt.Println("\nProvider configs are located at: ~/.claude/<name>.json")
}

func main() {
	flag.Usage = printUsage
	flag.Parse()

	args := flag.Args()
	hasProvider := len(args) > 0

	claudeDir, err := getClaudeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	settingsPath := filepath.Join(claudeDir, settingsFile)

	// Load settings
	settings, err := loadSettings(settingsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading settings: %v\n", err)
		os.Exit(1)
	}

	if hasProvider {
		provider := args[0]

		// Load provider env config
		providerEnv, err := loadProviderEnv(provider, claudeDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Merge env into settings
		mergeEnv(settings, providerEnv)
		fmt.Printf("Switched to provider '%s'\n", provider)
	} else {
		// Remove env key to reset to default
		removeEnv(settings)
		fmt.Println("Reset to default (removed env key)")
	}

	// Save settings
	if err := saveSettings(settingsPath, settings); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving settings: %v\n", err)
		os.Exit(1)
	}
}
