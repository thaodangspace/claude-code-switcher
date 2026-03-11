package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// loadProfile reads a profile from ~/.claude/ccs/<name>.json.
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

// listProfiles scans for available providers and accounts in ~/.claude/ccs and prints them.
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

// detectCurrentProvider tries to identify the current provider by matching env from ~/.claude/ccs.
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

// envMapsEqual compares two env maps for equality.
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

// fileExists reports whether a regular file exists at filename.
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
