//go:build !darwin

package main

import "fmt"

// newClaudeCredentialStore selects Claude Code's file credential backend on
// platforms without the macOS Keychain integration.
func newClaudeCredentialStore(claudeDir string) (ClaudeCredentialStore, error) {
	path := credentialFilePath(claudeDir)
	if !fileCredentialStoreExists(path) {
		return nil, fmt.Errorf("Claude OAuth credential store not found")
	}
	return NewFileClaudeCredentialStore(path), nil
}
