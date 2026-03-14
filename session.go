package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// linkClaudeConfigDir symlinks all files and directories from src to dest,
// skipping ccs, runs, and settings files, to preserve global skills/hooks/commands.
func linkClaudeConfigDir(src, dest string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == "ccs" || name == "runs" || strings.HasPrefix(name, "settings.json") {
			continue
		}
		srcPath := filepath.Join(src, name)
		destPath := filepath.Join(dest, name)
		if err := os.Symlink(srcPath, destPath); err != nil && !os.IsExist(err) {
			return err
		}
	}
	return nil
}

// parseRunArgs separates profile names and claude CLI arguments from the combined args slice.
// Profiles that exist in ccsDir are consumed first (provider first, then account),
// and the rest are forwarded to claude as-is.
func parseRunArgs(args []string, ccsDir string) (provider string, account string, claudeArgs []string) {
	for i := 0; i < len(args); {
		arg := args[i]

		// "--" explicitly ends profile names; everything after goes to claude.
		if arg == "--" {
			claudeArgs = append(claudeArgs, args[i+1:]...)
			break
		}

		// A flag-like argument ends profile name parsing too.
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

		// Unrecognised non-flag argument – treat the rest as claude arguments.
		claudeArgs = append(claudeArgs, args[i:]...)
		break
	}
	return
}

// runSession creates an isolated temp directory, writes scoped config files into it,
// and runs claude inside that directory via CLAUDE_CONFIG_DIR.
func runSession(claudeDir string, ccsDir string, args []string) (int, error) {
	provider, account, claudeArgs := parseRunArgs(args, ccsDir)

	if account != "" {
		return 1, fmt.Errorf("'ccs run' only supports provider profiles, not account profile '%s'", account)
	}

	tempDir := filepath.Join(claudeDir, "runs", fmt.Sprintf("session-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(filepath.Join(tempDir, ".claude"), 0755); err != nil {
		return 1, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Symlink all other items from the global claude directory to retain skills, hooks, mcp.json, etc.
	if err := linkClaudeConfigDir(claudeDir, tempDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to link base config items: %v\n", err)
	}
	// Also symlink into the .claude subfolder for fallback compatibility
	if err := linkClaudeConfigDir(claudeDir, filepath.Join(tempDir, ".claude")); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to link base config items (.claude): %v\n", err)
	}

	// Load base claude.json (fall back to empty if missing).
	claudeJsonPath, _ := getClaudeJsonPath()
	cj, err := loadClaudeJson(claudeJsonPath)
	if err != nil || cj == nil {
		cj = &ClaudeJson{Extra: make(map[string]interface{})}
	}

	// Load base settings (fall back to empty if missing).
	settingsPath := filepath.Join(claudeDir, settingsFile)
	settings, err := loadSettings(settingsPath)
	if err != nil || settings == nil {
		settings = &Settings{Extra: make(map[string]interface{})}
	}

	// Overlay provider env if requested.
	if provider != "" {
		profile, err := loadProfile(provider, ccsDir)
		if err != nil {
			return 1, fmt.Errorf("failed to load provider env '%s': %w", provider, err)
		}
		removeEnv(settings)
		mergeEnv(settings, profile.Env)
		cj.OAuthAccount = nil
	}

	// Write scoped config files into the temp directory.
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

	// Start claude and forward signals.
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
