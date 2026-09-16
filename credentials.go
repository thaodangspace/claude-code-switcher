package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// ClaudeCredentialStore reads and writes Claude Code's active credential
// container. Implementations must preserve unrelated container fields.
type ClaudeCredentialStore interface {
	Load() (*CredentialContainer, error)
	Save(*CredentialContainer) error
}

// FileClaudeCredentialStore stores the active credential container in a file.
type FileClaudeCredentialStore struct {
	Path string
}

// NewFileClaudeCredentialStore creates a file-backed active credential store.
func NewFileClaudeCredentialStore(path string) *FileClaudeCredentialStore {
	return &FileClaudeCredentialStore{Path: path}
}

func (s *FileClaudeCredentialStore) Load() (*CredentialContainer, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read Claude credential store: %w", err)
	}
	container, err := DecodeCredentialContainer(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Claude credential store")
	}
	return container, nil
}

func (s *FileClaudeCredentialStore) Save(container *CredentialContainer) error {
	if container == nil {
		return fmt.Errorf("cannot save an empty Claude credential store")
	}
	data, err := EncodeCredentialContainer(container)
	if err != nil {
		return fmt.Errorf("failed to encode Claude credential store")
	}
	if err := atomicWritePrivateFile(s.Path, data, 0600); err != nil {
		return fmt.Errorf("failed to write Claude credential store: %w", err)
	}
	return nil
}

// atomicWritePrivateFile writes data through a same-directory temporary file,
// then renames it into place. It never puts credential contents in an error.
func atomicWritePrivateFile(path string, data []byte, mode os.FileMode) error {
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return err
	}

	temp, err := os.CreateTemp(parent, ".ccs-credential-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if err := temp.Chmod(mode); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		// Windows does not replace an existing destination with Rename. The
		// fallback still uses a private, fully-written file and is only used
		// on platforms where replacement rename is unavailable.
		if runtime.GOOS != "windows" {
			return err
		}
		if removeErr := os.Remove(path); removeErr != nil {
			return err
		}
		if renameErr := os.Rename(tempPath, path); renameErr != nil {
			return renameErr
		}
	}
	// Rename preserves the temporary file's private mode. Chmod also handles
	// platforms/filesystems with unusual CreateTemp defaults.
	return os.Chmod(path, mode)
}

// credentialFilePath returns Claude Code's file-backed credential path.
func credentialFilePath(claudeDir string) string {
	return filepath.Join(claudeDir, ".credentials.json")
}

// fileCredentialStoreExists reports whether a regular credential file exists.
func fileCredentialStoreExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
