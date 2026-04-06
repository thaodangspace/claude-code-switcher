# Backup Provider/Account Feature Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `ccs backup-provider` and `ccs backup-account` commands to save current configuration as reusable profiles.

**Architecture:** Two new commands in main.go switch block, backed by functions in new backup.go file. Helper function extracts ANTHROPIC_* keys from env map. Profiles saved to ~/.claude/ccs/<name>.json with overwrite warning.

**Tech Stack:** Go 1.x, standard library (encoding/json, fmt, os, path/filepath, strings)

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `backup.go` | Create | `backupProviderCmd`, `backupAccountCmd`, `filterAnthropicEnv`, `saveProfile` |
| `main.go` | Modify | Add switch cases for new commands, update `printUsage()` |

---

### Task 1: Create backup.go with helper functions

**Files:**
- Create: `backup.go`

- [ ] **Step 1: Create backup.go with package declaration and imports**

```go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)
```

- [ ] **Step 2: Add filterAnthropicEnv helper function**

```go
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
```

- [ ] **Step 3: Add saveProfile helper function**

```go
// saveProfile writes a profile to ~/.claude/ccs/<name>.json.
// Prints overwrite warning if file already exists.
func saveProfile(ccsDir string, name string, profile *Profile) error {
	profilePath := filepath.Join(ccsDir, name + ".json")

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
```

- [ ] **Step 4: Add backupProviderCmd function**

```go
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

	// Get user's home directory for display path
	usr, _ := user.Current()
	displayPath := filepath.Join(usr.HomeDir, ".claude", "ccs", name+".json")
	fmt.Printf("Saved provider '%s' to %s\n", name, displayPath)
}
```

- [ ] **Step 5: Add os/user import for display path**

Add `"os/user"` to imports block:

```go
import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)
```

- [ ] **Step 6: Add backupAccountCmd function**

```go
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

	// Get user's home directory for display path
	usr, _ := user.Current()
	displayPath := filepath.Join(usr.HomeDir, ".claude", "ccs", name+".json")
	fmt.Printf("Saved account '%s' to %s\n", name, displayPath)
}
```

- [ ] **Step 7: Commit backup.go**

```bash
git add backup.go
git commit -m "$(cat <<'EOF'
feat: add backup commands for provider and account profiles

Add backup.go with backupProviderCmd and backupAccountCmd functions
to save current configuration as reusable profiles in ~/.claude/ccs/.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: Update main.go with new commands

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Add backup-provider case to switch block**

Insert after the `case "reset":` block (around line 74):

```go
case "backup-provider":
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Error: Usage: ccs backup-provider <name>\n")
		os.Exit(1)
	}
	backupProviderCmd(claudeDir, ccsDir, args[1])
```

- [ ] **Step 2: Add backup-account case to switch block**

Insert after the `backup-provider` case:

```go
case "backup-account":
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Error: Usage: ccs backup-account <name>\n")
		os.Exit(1)
	}
	backupAccountCmd(ccsDir, args[1])
```

- [ ] **Step 3: Update printUsage() with new commands**

Update the printUsage function to include the new commands (around lines 11-26):

```go
func printUsage() {
	fmt.Println("Claude Code Switcher (ccs)")
	fmt.Println("\nUsage:")
	fmt.Println("  ccs                   Show this help menu")
	fmt.Println("  ccs reset             Reset to default provider and account")
	fmt.Println("  ccs <name>            Switch to a provider or account profile")
	fmt.Println("  ccs list              List available providers and accounts")
	fmt.Println("  ccs current           Show current provider and account")
	fmt.Println("  ccs backup-provider <name>  Save current provider env as profile")
	fmt.Println("  ccs backup-account <name>   Save current OAuth account as profile")
	fmt.Println("  ccs run [names...] [--] [args...]  Run isolated claude session")
	fmt.Println("\nExamples:")
	fmt.Println("  ccs glm               Switch to 'glm' profile globally")
	fmt.Println("  ccs personal          Switch to 'personal' profile globally")
	fmt.Println("  ccs backup-provider mykey   Save current provider as 'mykey'")
	fmt.Println("  ccs backup-account work     Save current account as 'work'")
	fmt.Println("  ccs run glm -p hi     Run glm provider in isolated session with prompt")
	fmt.Println("  ccs list              Show all profiles")
	fmt.Println("  ccs current           Show current provider and account")
	fmt.Println("  ccs reset             Reset to defaults")
	fmt.Println("\nConfig Directory: ~/.claude/ccs/")
}
```

- [ ] **Step 4: Commit main.go changes**

```bash
git add main.go
git commit -m "$(cat <<'EOF'
feat: add backup-provider and backup-account commands to CLI

Update main.go switch block and printUsage() to expose new backup
commands for saving current configuration as reusable profiles.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: Build and test

**Files:**
- None (verification only)

- [ ] **Step 1: Build the binary**

```bash
cd /Users/dt/code/claude-code-switcher && go build -o ccs .
```

Expected: No errors, binary updated.

- [ ] **Step 2: Test help output shows new commands**

```bash
./ccs
```

Expected: Output includes:
```
  ccs backup-provider <name>  Save current provider env as profile
  ccs backup-account <name>   Save current OAuth account as profile
```

- [ ] **Step 3: Test backup-provider without name argument**

```bash
./ccs backup-provider
```

Expected: `Error: Usage: ccs backup-provider <name>` and exit code 1.

- [ ] **Step 4: Test backup-account without name argument**

```bash
./ccs backup-account
```

Expected: `Error: Usage: ccs backup-account <name>` and exit code 1.

- [ ] **Step 5: Test backup-provider with valid env**

First, ensure there's ANTHROPIC_* env in settings, then:

```bash
./ccs backup-provider testkey
```

Expected: `Saved provider 'testkey' to ~/.claude/ccs/testkey.json` (or warning if exists).

- [ ] **Step 6: Test backup-account with valid OAuth**

```bash
./ccs backup-account testaccount
```

Expected: `Saved account 'testaccount' to ~/.claude/ccs/testaccount.json` (or warning if exists).

- [ ] **Step 7: Test saved profile works with switch**

```bash
./ccs testkey
```

Expected: `Switched to provider 'testkey'`

- [ ] **Step 8: Test overwrite warning**

```bash
./ccs backup-provider testkey
```

Expected: `Warning: Overwriting existing profile 'testkey'` followed by success message.

- [ ] **Step 9: Final commit if any fixes needed**

If any issues found and fixed:

```bash
git add -A
git commit -m "$(cat <<'EOF'
fix: resolve issues found during backup feature testing

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```