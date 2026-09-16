//go:build darwin

package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
)

const claudeKeychainService = "Claude Code-credentials"

// keychainClaudeCredentialStore updates the existing Claude Code Keychain
// item. The account attribute is discovered from the item before writes so an
// incompatible duplicate is not created.
type keychainClaudeCredentialStore struct {
	service string
	run     func(...string) ([]byte, error)
}

func newKeychainClaudeCredentialStore() *keychainClaudeCredentialStore {
	return &keychainClaudeCredentialStore{
		service: claudeKeychainService,
		run: func(args ...string) ([]byte, error) {
			return exec.Command("/usr/bin/security", args...).CombinedOutput()
		},
	}
}

var keychainAccountPattern = regexp.MustCompile(`acct\s+"([^"]*)"`)

func (s *keychainClaudeCredentialStore) existingAccount() (string, error) {
	output, err := s.run("find-generic-password", "-s", s.service)
	if err != nil {
		return "", fmt.Errorf("Claude Code Keychain item not found")
	}
	match := keychainAccountPattern.FindSubmatch(output)
	if len(match) == 2 {
		return string(match[1]), nil
	}
	// Some security versions omit the account attribute from the printed
	// summary. An empty account still lets security select by service.
	return "", nil
}

func (s *keychainClaudeCredentialStore) itemExists() bool {
	_, err := s.run("find-generic-password", "-s", s.service)
	return err == nil
}

func (s *keychainClaudeCredentialStore) Load() (*CredentialContainer, error) {
	account, err := s.existingAccount()
	if err != nil {
		return nil, err
	}
	args := []string{"find-generic-password", "-s", s.service}
	if account != "" {
		args = append(args, "-a", account)
	}
	args = append(args, "-w")
	output, err := s.run(args...)
	if err != nil {
		return nil, fmt.Errorf("failed to read Claude Code Keychain credential")
	}
	output = bytes.TrimSpace(output)
	container, err := DecodeCredentialContainer(output)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Claude Code Keychain credential")
	}
	return container, nil
}

func (s *keychainClaudeCredentialStore) Save(container *CredentialContainer) error {
	if container == nil {
		return fmt.Errorf("cannot save an empty Claude credential store")
	}
	account, err := s.existingAccount()
	if err != nil {
		return err
	}
	data, err := EncodeCredentialContainer(container)
	if err != nil {
		return fmt.Errorf("failed to encode Claude Code Keychain credential")
	}
	args := []string{"add-generic-password", "-U", "-s", s.service}
	if account != "" {
		args = append(args, "-a", account)
	}
	args = append(args, "-w", string(data))
	if _, err := s.run(args...); err != nil {
		return fmt.Errorf("failed to update Claude Code Keychain credential")
	}
	return nil
}

// newClaudeCredentialStore prefers the existing Keychain item. A file is a
// fallback only when no expected Keychain item exists and Claude has a file
// credential store available.
func newClaudeCredentialStore(claudeDir string) (ClaudeCredentialStore, error) {
	keychain := newKeychainClaudeCredentialStore()
	if keychain.itemExists() {
		return keychain, nil
	}
	path := credentialFilePath(claudeDir)
	if fileCredentialStoreExists(path) {
		return NewFileClaudeCredentialStore(path), nil
	}
	return nil, fmt.Errorf("Claude OAuth credential store not found")
}
