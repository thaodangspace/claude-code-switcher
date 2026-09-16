package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// loadProfile reads a profile from ~/.claude/ccs/<name>.json.
func loadProfile(name string, ccsDir string) (*Profile, error) {
	if err := validateProfileName(name); err != nil {
		return nil, err
	}
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

type ProviderListItem struct {
	Name string `json:"name"`
}

type AccountListItem struct {
	Name             string            `json:"name"`
	Email            string            `json:"email,omitempty"`
	AccountUUID      string            `json:"accountUuid,omitempty"`
	Active           bool              `json:"active"`
	Health           TokenHealthStatus `json:"health"`
	AccessExpiresAt  *time.Time        `json:"accessExpiresAt,omitempty"`
	RefreshExpiresAt *time.Time        `json:"refreshExpiresAt,omitempty"`
	Detail           string            `json:"detail,omitempty"`
}

type ProfileListResult struct {
	Providers []ProviderListItem `json:"providers"`
	Accounts  []AccountListItem  `json:"accounts"`
}

// collectProfiles gathers list data without reading or mutating the active
// Claude credential store. A bad snapshot affects only its own account.
func collectProfiles(ccsDir string, currentAccount OAuthAccount, profileStore ProfileCredentialStore, now time.Time) (ProfileListResult, error) {
	result := ProfileListResult{
		Providers: make([]ProviderListItem, 0),
		Accounts:  make([]AccountListItem, 0),
	}
	entries, err := os.ReadDir(ccsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return result, fmt.Errorf("failed to read ccs directory: %w", err)
	}

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
			result.Providers = append(result.Providers, ProviderListItem{Name: name})
		}
		if profile.OAuthAccount == nil {
			continue
		}

		item := AccountListItem{
			Name:        name,
			Email:       toString(profile.OAuthAccount["emailAddress"]),
			AccountUUID: toString(profile.OAuthAccount["accountUuid"]),
			Active:      accountIdentityMatches(profile.OAuthAccount, currentAccount),
			Health:      TokenUnknown,
		}
		if profileStore == nil || !profileStore.Exists(name) {
			item.Detail = "credential not saved"
			result.Accounts = append(result.Accounts, item)
			continue
		}
		credential, err := profileStore.Load(name)
		if err != nil {
			item.Detail = "credential unreadable"
			result.Accounts = append(result.Accounts, item)
			continue
		}
		health := EvaluateTokenHealth(credential, now)
		item.Health = health.Status
		item.Detail = health.Detail
		item.AccessExpiresAt = health.AccessExpiresAt
		item.RefreshExpiresAt = health.RefreshExpiresAt
		result.Accounts = append(result.Accounts, item)
	}
	return result, nil
}

func listCurrentAccount(ccsDir string) OAuthAccount {
	claudeJSONPath := claudeJsonPathForClaudeDir(filepath.Dir(ccsDir))
	cj, err := loadClaudeJson(claudeJSONPath)
	if err != nil {
		return nil
	}
	return cj.OAuthAccount
}

// listProfiles scans available profiles and renders offline account health.
func listProfiles(ccsDir string) error {
	now := time.Now()
	result, err := collectProfiles(ccsDir, listCurrentAccount(ccsDir), NewProfileCredentialStore(ccsDir), now)
	if err != nil {
		return err
	}
	return renderProfileListAt(result, now)
}

func listProfilesJSON(ccsDir string) error {
	result, err := collectProfiles(ccsDir, listCurrentAccount(ccsDir), NewProfileCredentialStore(ccsDir), time.Now())
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func renderProfileList(result ProfileListResult) error {
	return renderProfileListAt(result, time.Now())
}

func renderProfileListAt(result ProfileListResult, now time.Time) error {
	fmt.Println("Providers:")
	if len(result.Providers) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, provider := range result.Providers {
			fmt.Printf("  %s\n", provider.Name)
		}
	}

	fmt.Println("Accounts:")
	if len(result.Accounts) == 0 {
		fmt.Println("  (none)")
		return nil
	}
	for _, account := range result.Accounts {
		marker := " "
		if account.Active {
			marker = "*"
		}
		email := account.Email
		if email == "" {
			email = "-"
		}
		detail := account.Detail
		if account.Health == TokenReady && account.AccessExpiresAt != nil {
			detail = "access " + formatTokenDuration(now, *account.AccessExpiresAt)
		} else if account.Health == TokenRefreshNeeded && account.RefreshExpiresAt != nil {
			detail = "refresh " + formatTokenDuration(now, *account.RefreshExpiresAt)
		}
		if detail != "" {
			fmt.Printf("%s %-20s %-28s %-16s %s\n", marker, account.Name, email, account.Health, detail)
		} else {
			fmt.Printf("%s %-20s %-28s %s\n", marker, account.Name, email, account.Health)
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
