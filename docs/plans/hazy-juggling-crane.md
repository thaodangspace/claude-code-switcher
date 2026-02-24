# Plan: Fix plansDirectory Setting Being Lost

## Context

When running `ccs glm`, the `plansDirectory` setting (`"plansDirectory": "./docs/plans"`) is being removed from `~/.claude/settings.json`. This happens because the `Settings` struct in `main.go` does not include the `plansDirectory` field, so JSON unmarshaling silently drops it and marshaling doesn't preserve it.

## Implementation

Add the missing `plansDirectory` field to the `Settings` struct.

### File to Modify

**main.go** (lines 18-24)

Change the `Settings` struct from:
```go
type Settings struct {
	Permissions     map[string]interface{} `json:"permissions,omitempty"`
	Model           string                 `json:"model,omitempty"`
	StatusLine      map[string]interface{} `json:"statusLine,omitempty"`
	EnabledPlugins  map[string]interface{} `json:"enabledPlugins,omitempty"`
	Env             map[string]interface{} `json:"env,omitempty"`
}
```

To:
```go
type Settings struct {
	Permissions      map[string]interface{} `json:"permissions,omitempty"`
	Model            string                 `json:"model,omitempty"`
	StatusLine       map[string]interface{} `json:"statusLine,omitempty"`
	EnabledPlugins   map[string]interface{} `json:"enabledPlugins,omitempty"`
	Env              map[string]interface{} `json:"env,omitempty"`
	PlansDirectory   string                 `json:"plansDirectory,omitempty"`
}
```

## Verification

1. Build the project: `go build`
2. Ensure `~/.claude/settings.json` contains `"plansDirectory": "./docs/plans"`
3. Run `ccs glm`
4. Check `~/.claude/settings.json` - `plansDirectory` should still be present
5. Run `ccs` (reset)
6. Check `~/.claude/settings.json` - `plansDirectory` should still be present
