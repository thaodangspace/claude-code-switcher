package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProfileCredentialStore stores OAuth snapshots separately from normal profile
// metadata files.
type ProfileCredentialStore interface {
	Load(name string) (*ClaudeAIOAuthCredential, error)
	Save(name string, credential *ClaudeAIOAuthCredential) error
	Exists(name string) bool
}

// FileProfileCredentialStore is the private filesystem snapshot backend.
type FileProfileCredentialStore struct {
	Dir string
}

// NewFileProfileCredentialStore creates a snapshot store below ccsDir.
func NewFileProfileCredentialStore(ccsDir string) *FileProfileCredentialStore {
	return &FileProfileCredentialStore{Dir: filepath.Join(ccsDir, "credentials")}
}

// NewProfileCredentialStore is an explicit constructor for the default
// filesystem snapshot backend.
func NewProfileCredentialStore(ccsDir string) ProfileCredentialStore {
	return NewFileProfileCredentialStore(ccsDir)
}

func validateProfileName(name string) error {
	if name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("profile name is not safe")
	}
	if strings.ContainsAny(name, `/\\`) {
		return fmt.Errorf("profile name must not contain path separators")
	}
	if strings.IndexByte(name, 0) >= 0 {
		return fmt.Errorf("profile name must not contain NUL")
	}
	return nil
}

func (s *FileProfileCredentialStore) path(name string) (string, error) {
	if err := validateProfileName(name); err != nil {
		return "", err
	}
	return filepath.Join(s.Dir, name+".json"), nil
}

func (s *FileProfileCredentialStore) Load(name string) (*ClaudeAIOAuthCredential, error) {
	path, err := s.path(name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read saved OAuth credential")
	}
	var credential ClaudeAIOAuthCredential
	if err := json.Unmarshal(data, &credential); err != nil {
		return nil, fmt.Errorf("saved OAuth credential is malformed")
	}
	return &credential, nil
}

func (s *FileProfileCredentialStore) Save(name string, credential *ClaudeAIOAuthCredential) error {
	path, err := s.path(name)
	if err != nil {
		return err
	}
	if credential == nil {
		return s.Delete(name)
	}
	data, err := json.MarshalIndent(credential, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode saved OAuth credential")
	}
	if err := os.MkdirAll(s.Dir, 0700); err != nil {
		return fmt.Errorf("failed to create credential snapshot directory: %w", err)
	}
	if err := os.Chmod(s.Dir, 0700); err != nil {
		return fmt.Errorf("failed to secure credential snapshot directory: %w", err)
	}
	if err := atomicWritePrivateFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write saved OAuth credential: %w", err)
	}
	return nil
}

// Delete removes a snapshot. It is used internally for rollback and is not
// part of the public interface so alternative secure backends remain possible.
func (s *FileProfileCredentialStore) Delete(name string) error {
	path, err := s.path(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove saved OAuth credential")
	}
	return nil
}

func (s *FileProfileCredentialStore) Exists(name string) bool {
	path, err := s.path(name)
	if err != nil {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// profileCredentialDeleter is an optional capability used to make coordinated
// metadata/snapshot writes reversible when the previous snapshot did not exist.
type profileCredentialDeleter interface {
	Delete(name string) error
}

func deleteProfileCredential(store ProfileCredentialStore, name string) error {
	if deleter, ok := store.(profileCredentialDeleter); ok {
		return deleter.Delete(name)
	}
	// Save(nil) is supported by the built-in backend and gives simple test
	// doubles a chance to implement rollback without another interface method.
	return store.Save(name, nil)
}
