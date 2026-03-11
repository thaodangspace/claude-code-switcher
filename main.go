package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"time"
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
		"permissions":    func(v interface{}) { s.Permissions = toMap(v) },
		"model":          func(v interface{}) { s.Model = toString(v) },
		"statusLine":     func(v interface{}) { s.StatusLine = toMap(v) },
		"enabledPlugins": func(v interface{}) { s.EnabledPlugins = toMap(v) },
		"env":            func(v interface{}) { s.Env = toMap(v) },
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

// Profile represents ~/.claude/ccs/<name>.json which holds either provider env or oauthAccount
type Profile struct {
	Env          map[string]interface{} `json:"env,omitempty"`
	OAuthAccount OAuthAccount           `json:"oauthAccount,omitempty"`
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

// getCcsDir returns the ~/.claude/ccs directory path, creating it if needed
func getCcsDir(claudeDir string) (string, error) {
	ccsDir := filepath.Join(claudeDir, "ccs")
	if err := os.MkdirAll(ccsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create ccs directory: %w", err)
	}
	return ccsDir, nil
}

// loadProfile reads a profile from ~/.claude/ccs/<name>.json
func loadProfile(name string, ccsDir string) (*Profile, error) {
	profilePath := filepath.Join(ccsDir, name+".json")
	data, err := os.ReadFile(profilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile %s: %w", profilePath, err)
	}

	var profile Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("failed to parse profile: %w", err)
	}

	if profile.Env == nil && profile.OAuthAccount == nil {
		return nil, fmt.Errorf("profile %s has neither 'env' nor 'oauthAccount' keys", profilePath)
	}

	return &profile, nil
}

// listProfiles scans for available providers and accounts in ~/.claude/ccs and prints them
func listProfiles(ccsDir string) error {
	entries, err := os.ReadDir(ccsDir)
	if err != nil {
		if os.IsNotExist(err) {
			entries = []os.DirEntry{}
		} else {
			return fmt.Errorf("failed to read ccs directory: %w", err)
		}
	}

	var providers []string
	var accounts []string

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		
		name := strings.TrimSuffix(entry.Name(), ".json")
		profile, err := loadProfile(name, ccsDir)
		if err != nil {
			continue
		}

		if profile.Env != nil {
			providers = append(providers, name)
		}
		if profile.OAuthAccount != nil {
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

// showCurrent displays the current provider and account
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

// detectCurrentProvider tries to identify the current provider by matching env from ~/.claude/ccs
func detectCurrentProvider(ccsDir string, currentEnv map[string]interface{}) string {
	entries, err := os.ReadDir(ccsDir)
	if err != nil {
		return "custom"
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".json")
		profile, err := loadProfile(name, ccsDir)
		if err != nil || profile.Env == nil {
			continue
		}

		if envMapsEqual(profile.Env, currentEnv) {
			return name
		}
	}

	var envStrs []string
	for k := range currentEnv {
		envStrs = append(envStrs, k)
	}
	return fmt.Sprintf("custom (%s)", strings.Join(envStrs, ", "))
}

// envMapsEqual compares two env maps for equality
func envMapsEqual(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// printUsage prints usage information
func printUsage() {
	fmt.Println("Claude Code Switcher (ccs)")
	fmt.Println("\nUsage:")
	fmt.Println("  ccs                   Show this help menu")
	fmt.Println("  ccs reset             Reset to default provider and account")
	fmt.Println("  ccs <name>            Switch to a provider or account profile")
	fmt.Println("  ccs list              List available providers and accounts")
	fmt.Println("  ccs current           Show current provider and account")
	fmt.Println("  ccs run [names...] [--] [args...]  Run isolated claude session")
	fmt.Println("\nExamples:")
	fmt.Println("  ccs glm               Switch to 'glm' profile globally")
	fmt.Println("  ccs personal          Switch to 'personal' profile globally")
	fmt.Println("  ccs run glm -p hi     Run glm provider in isolated session with prompt")
	fmt.Println("  ccs list              Show all profiles")
	fmt.Println("  ccs current           Show current provider and account")
	fmt.Println("  ccs reset             Reset to defaults")
	fmt.Println("\nConfig Directory: ~/.claude/ccs/")
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func parseRunArgs(args []string, ccsDir string) (provider string, account string, claudeArgs []string) {
	for i := 0; i < len(args); {
		arg := args[i]
		if arg == "--" {
			claudeArgs = append(claudeArgs, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "-") {
			claudeArgs = append(claudeArgs, args[i:]...)
			break
		}

		if fileExists(filepath.Join(ccsDir, arg+".json")) {
			profile, err := loadProfile(arg, ccsDir)
			if err == nil {
				if profile.Env != nil && provider == "" {
					provider = arg
					i++
					continue
				} else if profile.OAuthAccount != nil && account == "" {
					account = arg
					i++
					continue
				}
			}
		}

		// Unrecognized un-dashed argument -> assume it's part of claude command (e.g., the prompt string)
		claudeArgs = append(claudeArgs, args[i:]...)
		break
	}
	return
}

func runSession(claudeDir string, ccsDir string, args []string) (int, error) {
	provider, account, claudeArgs := parseRunArgs(args, ccsDir)

	tempDir := filepath.Join(claudeDir, "runs", fmt.Sprintf("session-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(filepath.Join(tempDir, ".claude"), 0755); err != nil {
		return 1, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	claudeJsonPath, _ := getClaudeJsonPath()
	cj, err := loadClaudeJson(claudeJsonPath)
	if err != nil || cj == nil {
		cj = &ClaudeJson{Extra: make(map[string]interface{})}
	}

	settingsPath := filepath.Join(claudeDir, settingsFile)
	settings, err := loadSettings(settingsPath)
	if err != nil || settings == nil {
		settings = &Settings{Extra: make(map[string]interface{})}
	}

	if account != "" {
		profile, err := loadProfile(account, ccsDir)
		if err != nil {
			return 1, fmt.Errorf("failed to load account profile '%s': %w", account, err)
		}
		cj.OAuthAccount = profile.OAuthAccount
	}

	if provider != "" {
		profile, err := loadProfile(provider, ccsDir)
		if err != nil {
			return 1, fmt.Errorf("failed to load provider env '%s': %w", provider, err)
		}
		removeEnv(settings)
		mergeEnv(settings, profile.Env)
	}

	tempClaudeJson := filepath.Join(tempDir, ".claude.json")
	if err := saveClaudeJson(tempClaudeJson, cj); err != nil {
		return 1, fmt.Errorf("failed to write temp .claude.json: %w", err)
	}

	tempSettings := filepath.Join(tempDir, ".claude", settingsFile)
	if err := saveSettings(tempSettings, settings); err != nil {
		return 1, fmt.Errorf("failed to write temp settings.json: %w", err)
	}

	if err := saveSettings(filepath.Join(tempDir, settingsFile), settings); err != nil {
		return 1, fmt.Errorf("failed to write fallback root settings.json: %w", err)
	}

	cmd := exec.Command("claude", claudeArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "CLAUDE_CONFIG_DIR="+tempDir)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for sig := range sigChan {
			if cmd.Process != nil {
				_ = cmd.Process.Signal(sig)
			}
		}
	}()

	runErr := cmd.Run()
	signal.Stop(sigChan)
	close(sigChan)

	if runErr != nil {
		if exitError, ok := runErr.(*exec.ExitError); ok {
			return exitError.ExitCode(), nil
		}
		return 1, fmt.Errorf("claude execution failed: %w", runErr)
	}
	return 0, nil
}

func main() {
	flag.Usage = printUsage
	flag.Parse()

	args := flag.Args()

	if len(args) == 0 {
		printUsage()
		return
	}

	claudeDir, err := getClaudeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	ccsDir, err := getCcsDir(claudeDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Handle "ccs list"
	if args[0] == "list" {
		if err := listProfiles(ccsDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Handle "ccs run"
	if args[0] == "run" {
		exitCode, err := runSession(claudeDir, ccsDir, args[1:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(exitCode)
	}

	// Handle "ccs current"
	if args[0] == "current" {
		if err := showCurrent(claudeDir, ccsDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Handle "ccs reset"
	if args[0] == "reset" {
		settingsPath := filepath.Join(claudeDir, settingsFile)
		settings, err := loadSettings(settingsPath)
		if err == nil {
			removeEnv(settings)
			saveSettings(settingsPath, settings)
		}

		claudeJsonPath, err := getClaudeJsonPath()
		if err == nil {
			cj, err := loadClaudeJson(claudeJsonPath)
			if err == nil {
				cj.OAuthAccount = nil
				saveClaudeJson(claudeJsonPath, cj)
			}
		}

		fmt.Println("Reset to default provider and account")
		return
	}

	// Handle switching to a specific profile
	name := args[0]
	profile, err := loadProfile(name, ccsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Profile '%s' not found or invalid: %v\n", name, err)
		os.Exit(1)
	}

	if profile.OAuthAccount != nil {
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
		fmt.Printf("Switched to account '%s'\n", name)
	}

	if profile.Env != nil {
		settingsPath := filepath.Join(claudeDir, settingsFile)
		settings, err := loadSettings(settingsPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading settings: %v\n", err)
			os.Exit(1)
		}
		removeEnv(settings)
		mergeEnv(settings, profile.Env)
		if err := saveSettings(settingsPath, settings); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving settings: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Switched to provider '%s'\n", name)
	}
}
