//go:build !darwin

package main

// newClaudeCredentialStore selects Claude Code's file credential backend on
// platforms without the macOS Keychain integration. Loading reports a useful
// missing-file error when Claude has not created the store yet.
func newClaudeCredentialStore(claudeDir string) (ClaudeCredentialStore, error) {
	return NewFileClaudeCredentialStore(credentialFilePath(claudeDir)), nil
}
