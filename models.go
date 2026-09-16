package main

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Settings represents the Claude settings.json structure.
// Uses custom JSON marshaling to preserve all unknown fields.
type Settings struct {
	Permissions    map[string]interface{} `json:"permissions,omitempty"`
	Model          string                 `json:"model,omitempty"`
	StatusLine     map[string]interface{} `json:"statusLine,omitempty"`
	EnabledPlugins map[string]interface{} `json:"enabledPlugins,omitempty"`
	Env            map[string]interface{} `json:"env,omitempty"`

	// Extra captures any unknown fields to preserve them
	Extra map[string]interface{} `json:"-"`
}

// UnmarshalJSON handles custom unmarshaling to preserve unknown fields.
func (s *Settings) UnmarshalJSON(data []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	s.Extra = make(map[string]interface{})

	knownFields := map[string]func(interface{}){
		"permissions":    func(v interface{}) { s.Permissions = toMap(v) },
		"model":          func(v interface{}) { s.Model = toString(v) },
		"statusLine":     func(v interface{}) { s.StatusLine = toMap(v) },
		"enabledPlugins": func(v interface{}) { s.EnabledPlugins = toMap(v) },
		"env":            func(v interface{}) { s.Env = toMap(v) },
	}

	for key, value := range raw {
		if handler, known := knownFields[key]; known {
			handler(value)
		} else {
			s.Extra[key] = value
		}
	}

	return nil
}

// MarshalJSON handles custom marshaling to include all fields including Extras.
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

	for key, value := range s.Extra {
		result[key] = value
	}

	return json.Marshal(result)
}

// Profile represents ~/.claude/ccs/<name>.json which holds either provider env or oauthAccount.
type Profile struct {
	Env          map[string]interface{} `json:"env,omitempty"`
	OAuthAccount OAuthAccount           `json:"oauthAccount,omitempty"`
}

// OAuthAccount holds the oauthAccount data from ~/.claude.json.
type OAuthAccount map[string]interface{}

// ClaudeJson represents ~/.claude.json, preserving all unknown fields.
type ClaudeJson struct {
	OAuthAccount OAuthAccount           `json:"oauthAccount,omitempty"`
	Extra        map[string]interface{} `json:"-"`
}

// UnmarshalJSON handles custom unmarshaling for ClaudeJson.
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

// MarshalJSON handles custom marshaling for ClaudeJson.
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

// ClaudeAIOAuthCredential is the Claude Code OAuth credential. Unknown fields
// are retained so CCS can round-trip fields added by future Claude Code versions.
type ClaudeAIOAuthCredential struct {
	AccessToken           string
	RefreshToken          string
	ExpiresAt             int64
	RefreshTokenExpiresAt int64
	Extra                 map[string]json.RawMessage
}

// UnmarshalJSON decodes known OAuth fields while preserving unknown fields.
// Expiry values are deliberately restricted to integer JSON numbers: accepting
// an unknown format would make local health classification unsafe.
func (c *ClaudeAIOAuthCredential) UnmarshalJSON(data []byte) error {
	if trimmed := bytes.TrimSpace(data); len(trimmed) == 0 || trimmed[0] != '{' {
		return fmt.Errorf("OAuth credential must be a JSON object")
	}
	c.AccessToken = ""
	c.RefreshToken = ""
	c.ExpiresAt = 0
	c.RefreshTokenExpiresAt = 0
	c.Extra = nil
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	c.Extra = make(map[string]json.RawMessage)

	for key, value := range raw {
		switch key {
		case "accessToken":
			if err := json.Unmarshal(value, &c.AccessToken); err != nil {
				return fmt.Errorf("invalid access token field")
			}
		case "refreshToken":
			if err := json.Unmarshal(value, &c.RefreshToken); err != nil {
				return fmt.Errorf("invalid refresh token field")
			}
		case "expiresAt":
			if err := json.Unmarshal(value, &c.ExpiresAt); err != nil {
				return fmt.Errorf("invalid expiresAt field")
			}
		case "refreshTokenExpiresAt":
			if err := json.Unmarshal(value, &c.RefreshTokenExpiresAt); err != nil {
				return fmt.Errorf("invalid refreshTokenExpiresAt field")
			}
		default:
			c.Extra[key] = append(json.RawMessage(nil), value...)
		}
	}
	return nil
}

// MarshalJSON encodes known OAuth fields and all preserved unknown fields.
func (c *ClaudeAIOAuthCredential) MarshalJSON() ([]byte, error) {
	result := make(map[string]json.RawMessage, len(c.Extra)+4)
	for key, value := range c.Extra {
		result[key] = append(json.RawMessage(nil), value...)
	}
	if c.AccessToken != "" {
		value, err := json.Marshal(c.AccessToken)
		if err != nil {
			return nil, err
		}
		result["accessToken"] = value
	}
	if c.RefreshToken != "" {
		value, err := json.Marshal(c.RefreshToken)
		if err != nil {
			return nil, err
		}
		result["refreshToken"] = value
	}
	if c.ExpiresAt != 0 {
		value, err := json.Marshal(c.ExpiresAt)
		if err != nil {
			return nil, err
		}
		result["expiresAt"] = value
	}
	if c.RefreshTokenExpiresAt != 0 {
		value, err := json.Marshal(c.RefreshTokenExpiresAt)
		if err != nil {
			return nil, err
		}
		result["refreshTokenExpiresAt"] = value
	}
	return json.Marshal(result)
}

// CredentialContainer is the active Claude credential document. Only the
// Claude OAuth subtree is modeled; every other top-level field is retained.
type CredentialContainer struct {
	ClaudeAIOAuth *ClaudeAIOAuthCredential
	Extra         map[string]json.RawMessage
}

// UnmarshalJSON decodes the active credential document without dropping
// unrelated credential data such as mcpOAuth.
func (c *CredentialContainer) UnmarshalJSON(data []byte) error {
	if trimmed := bytes.TrimSpace(data); len(trimmed) == 0 || trimmed[0] != '{' {
		return fmt.Errorf("credential container must be a JSON object")
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	c.Extra = make(map[string]json.RawMessage)
	c.ClaudeAIOAuth = nil
	for key, value := range raw {
		if key == "claudeAiOauth" {
			if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				continue
			}
			var oauth ClaudeAIOAuthCredential
			if err := json.Unmarshal(value, &oauth); err != nil {
				return fmt.Errorf("invalid claudeAiOauth credential")
			}
			c.ClaudeAIOAuth = &oauth
			continue
		}
		c.Extra[key] = append(json.RawMessage(nil), value...)
	}
	return nil
}

// MarshalJSON encodes the complete credential document.
func (c *CredentialContainer) MarshalJSON() ([]byte, error) {
	result := make(map[string]json.RawMessage, len(c.Extra)+1)
	for key, value := range c.Extra {
		result[key] = append(json.RawMessage(nil), value...)
	}
	if c.ClaudeAIOAuth != nil {
		value, err := json.Marshal(c.ClaudeAIOAuth)
		if err != nil {
			return nil, err
		}
		result["claudeAiOauth"] = value
	}
	return json.Marshal(result)
}

// ReplaceClaudeAIOAuth replaces only the Claude OAuth subtree.
func (c *CredentialContainer) ReplaceClaudeAIOAuth(credential *ClaudeAIOAuthCredential) {
	c.ClaudeAIOAuth = credential
}

// DecodeCredentialContainer decodes an active credential document.
func DecodeCredentialContainer(data []byte) (*CredentialContainer, error) {
	var container CredentialContainer
	if err := json.Unmarshal(data, &container); err != nil {
		return nil, err
	}
	return &container, nil
}

// EncodeCredentialContainer encodes an active credential document.
func EncodeCredentialContainer(container *CredentialContainer) ([]byte, error) {
	return json.Marshal(container)
}

// toMap safely casts an interface{} to map[string]interface{}.
func toMap(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return nil
}

// toString safely casts an interface{} to string.
func toString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
