package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
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

// OAuthAccount holds the oauthAccount data from ~/.claude.json
type OAuthAccount map[string]interface{}

// ClaudeJson represents ~/.claude.json, preserving all unknown fields
type ClaudeJson struct {
	OAuthAccount OAuthAccount           `json:"oauthAccount,omitempty"`
	Extra        map[string]interface{} `json:"-"`
}

func (c *ClaudeJson) UnmarshalJSON(data []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	c.Extra = make(map[string]interface{})
	for key, value := range raw {
		if key == "oauthAccount" {
			c.OAuthAccount = toMap(value)
		} else {
			c.Extra[key] = value
		}
	}
	return nil
}

func (c *ClaudeJson) MarshalJSON() ([]byte, error) {
	result := make(map[string]interface{})
	for key, value := range c.Extra {
		result[key] = value
	}
	if c.OAuthAccount != nil {
		result["oauthAccount"] = c.OAuthAccount
	}
	return json.Marshal(result)
}

// AccountProfile represents ~/.claude/accounts/<name>.json
type AccountProfile struct {
	OAuthAccount OAuthAccount `json:"oauthAccount"`
}

// getClaudeDir returns the ~/.claude directory path
func getClaudeDir() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}
	return filepath.Join(usr.HomeDir, claudeDir), nil
}

// getClaudeJsonPath returns the ~/.claude.json file path
func getClaudeJsonPath() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}
	return filepath.Join(usr.HomeDir, ".claude.json"), nil
}

// loadClaudeJson reads and parses ~/.claude.json
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

// saveClaudeJson writes ~/.claude.json with proper JSON formatting
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

// getAccountsDir returns the ~/.claude/accounts directory path, creating it if needed
func getAccountsDir(claudeDir string) (string, error) {
	accountsDir := filepath.Join(claudeDir, "accounts")
	if err := os.MkdirAll(accountsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create accounts directory: %w", err)
	}
	return accountsDir, nil
}

// loadAccountProfile reads oauthAccount from ~/.claude/accounts/<name>.json
func loadAccountProfile(name string, claudeDir string) (OAuthAccount, error) {
	accountPath := filepath.Join(claudeDir, "accounts", name+".json")
	data, err := os.ReadFile(accountPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read account profile %s: %w", accountPath, err)
	}

	var profile AccountProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("failed to parse account profile: %w", err)
	}

	if profile.OAuthAccount == nil {
		return nil, fmt.Errorf("account profile %s is missing 'oauthAccount' key", accountPath)
	}

	return profile.OAuthAccount, nil
}

// listProfiles scans for available providers and accounts and prints them
func listProfiles(claudeDir string) error {
	nonProfileFiles := map[string]bool{
		"settings.json":          true,
		"mcp-needs-auth-cache.json": true,
		"stats-cache.json":       true,
	}

	// Scan ~/.claude/*.json for providers
	providerEntries, err := os.ReadDir(claudeDir)
	if err != nil {
		return fmt.Errorf("failed to read claude directory: %w", err)
	}

	var providers []string
	for _, entry := range providerEntries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if nonProfileFiles[entry.Name()] {
			continue
		}
		// Only include files with a top-level "env" key
		data, err := os.ReadFile(filepath.Join(claudeDir, entry.Name()))
		if err != nil {
			continue
		}
		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			continue
		}
		if _, hasEnv := raw["env"]; hasEnv {
			name := strings.TrimSuffix(entry.Name(), ".json")
			providers = append(providers, name)
		}
	}

	// Scan ~/.claude/accounts/*.json for accounts
	accountsDir := filepath.Join(claudeDir, "accounts")
	accountEntries, err := os.ReadDir(accountsDir)
	var accounts []string
	if err == nil {
		for _, entry := range accountEntries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			name := strings.TrimSuffix(entry.Name(), ".json")
			accounts = append(accounts, name)
		}
	}

	fmt.Println("Providers:")
	if len(providers) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, p := range providers {
			fmt.Printf("  %s\n", p)
		}
	}

	fmt.Println("Accounts:")
	if len(accounts) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, a := range accounts {
			fmt.Printf("  %s\n", a)
		}
	}

	return nil
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
	fmt.Println("  ccs                   Reset to default (remove env key)")
	fmt.Println("  ccs <provider>        Switch to API provider (merge env from <name>.json)")
	fmt.Println("  ccs account <name>    Switch to a claude.ai account profile")
	fmt.Println("  ccs account           Reset to default claude.ai account")
	fmt.Println("  ccs list              List available providers and accounts")
	fmt.Println("\nExamples:")
	fmt.Println("  ccs glm               Switch to glm provider")
	fmt.Println("  ccs account personal  Switch to personal claude.ai account")
	fmt.Println("  ccs list              Show all providers and accounts")
	fmt.Println("  ccs                   Reset to default")
	fmt.Println("\nProvider configs: ~/.claude/<name>.json")
	fmt.Println("Account configs:  ~/.claude/accounts/<name>.json")
}

func main() {
	flag.Usage = printUsage
	flag.Parse()

	args := flag.Args()

	claudeDir, err := getClaudeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Handle "ccs list"
	if len(args) == 1 && args[0] == "list" {
		if err := listProfiles(claudeDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Handle "ccs account [name]"
	if len(args) >= 1 && args[0] == "account" {
		settingsPath := filepath.Join(claudeDir, settingsFile)
		settings, err := loadSettings(settingsPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading settings: %v\n", err)
			os.Exit(1)
		}

		if len(args) >= 2 {
			name := args[1]
			oauthAccount, err := loadAccountProfile(name, claudeDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
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
			cj.OAuthAccount = oauthAccount
			if err := saveClaudeJson(claudeJsonPath, cj); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving ~/.claude.json: %v\n", err)
				os.Exit(1)
			}

			removeEnv(settings)
			fmt.Printf("Switched to account '%s'\n", name)
		} else {
			removeEnv(settings)
			fmt.Println("Reset to default account")
		}

		if err := saveSettings(settingsPath, settings); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving settings: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Existing provider / reset behavior
	settingsPath := filepath.Join(claudeDir, settingsFile)
	settings, err := loadSettings(settingsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading settings: %v\n", err)
		os.Exit(1)
	}

	if len(args) > 0 {
		provider := args[0]

		providerEnv, err := loadProviderEnv(provider, claudeDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		mergeEnv(settings, providerEnv)
		fmt.Printf("Switched to provider '%s'\n", provider)
	} else {
		removeEnv(settings)
		fmt.Println("Reset to default (removed env key)")
	}

	if err := saveSettings(settingsPath, settings); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving settings: %v\n", err)
		os.Exit(1)
	}
}
