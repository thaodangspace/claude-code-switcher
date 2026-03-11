package main

import "encoding/json"

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
