# Plan: Account Switching for Claude Code Plans (Pro/Team)

## Context

The tool currently supports switching between third-party **API providers** (e.g., GLM, local LM Studio) by swapping the `env` block in `~/.claude/settings.json`. However, there is no way to switch between native **claude.ai plan accounts** (Pro/Team), which is a distinct use case:

- **Provider switching**: Changes `ANTHROPIC_BASE_URL` + token to route to a different API backend
- **Account switching**: Updates the `oauthAccount` field in `~/.claude.json` to change which claude.ai account is active, then clears any `ANTHROPIC_AUTH_TOKEN` env override so Claude Code falls back to native OAuth

### How Claude Code authentication works

- `~/.claude.json` contains an `oauthAccount` key with the active account's metadata (UUID, email, org, billing type, etc.)
- `~/.claude/settings.json` contains an `env` block that can override auth via `ANTHROPIC_AUTH_TOKEN`
- **If `ANTHROPIC_AUTH_TOKEN` env is set**, Claude Code uses it (API key / provider mode)
- **If `ANTHROPIC_AUTH_TOKEN` env is NOT set**, Claude Code uses the `oauthAccount` from `~/.claude.json` (native OAuth mode)

So account switching = update `oauthAccount` in `~/.claude.json` + clear env in `settings.json`.

## Approach

Extend the CLI with:
1. A new `account` subcommand (`ccs account <name>` / `ccs account`)
2. Account profiles stored at `~/.claude/accounts/<name>.json` containing `oauthAccount` data
3. A `list` subcommand to enumerate available providers and accounts
4. When activating an account: update `~/.claude.json`'s `oauthAccount` field AND clear env from `settings.json`

## Profile Format

**Account profile** (`~/.claude/accounts/<name>.json`):
```json
{
  "oauthAccount": {
    "accountUuid": "9a88e9bb-...",
    "emailAddress": "user@example.com",
    "organizationUuid": "6fa74e61-...",
    "displayName": "Alice",
    "organizationRole": "admin",
    "organizationName": "My Org",
    "hasExtraUsageEnabled": true,
    "billingType": "stripe_subscription",
    "accountCreatedAt": "2025-07-08T02:54:18.587675Z",
    "subscriptionCreatedAt": "2025-09-05T17:00:33.232633Z",
    "workspaceRole": null
  }
}
```

The `oauthAccount` object is copied verbatim from the `oauthAccount` key already present in `~/.claude.json` (capture it once per account, then store as a profile).

## CLI Interface

```
ccs                     # Reset to default (clear all env) — existing behavior
ccs <provider>          # Switch API provider — existing behavior
ccs account <name>      # Switch to a claude.ai account profile (new)
ccs account             # Clear env / fall back to current oauthAccount (new)
ccs list                # List available providers and accounts (new)
```

## Implementation Steps

### 1. Add `getClaudeJsonPath()` helper
Returns `~/.claude.json` (note: file in home dir, not inside `~/.claude/`).

### 2. Add `ClaudeJson` struct with custom marshal/unmarshal
Preserves all unknown fields (similar to `Settings`). Only the `oauthAccount` field needs to be known.

```go
type OAuthAccount map[string]interface{}

type ClaudeJson struct {
    OAuthAccount OAuthAccount           `json:"oauthAccount,omitempty"`
    Extra        map[string]interface{} `json:"-"`
}
// Custom UnmarshalJSON / MarshalJSON to preserve Extra fields
```

### 3. Add `loadClaudeJson()` / `saveClaudeJson()`
Mirror existing `loadSettings()` / `saveSettings()` but for `~/.claude.json`.

### 4. Add `loadAccountProfile()` function
Reads `~/.claude/accounts/<name>.json`, returns the `oauthAccount` map.

```go
type AccountProfile struct {
    OAuthAccount OAuthAccount `json:"oauthAccount"`
}

func loadAccountProfile(name string, claudeDir string) (OAuthAccount, error) {
    // reads ~/.claude/accounts/<name>.json
    // returns the oauthAccount map
}
```

### 5. Add `listProfiles()` function
Scans `~/.claude/*.json` (excluding known non-profile files) for providers and `~/.claude/accounts/*.json` for accounts. Prints grouped output:

```
Providers:
  glm
  local

Accounts:
  personal
  work
```

Non-profile files to skip: `settings.json`, `mcp-needs-auth-cache.json`, `stats-cache.json`.
Only include files that contain a top-level `"env"` key.

### 6. Extend `main()` to handle new subcommands

**Account switch logic (`ccs account <name>`):**
```
1. Load ~/.claude/accounts/<name>.json → get oauthAccount
2. Load ~/.claude.json → set oauthAccount field → save ~/.claude.json
3. Load ~/.claude/settings.json → clear env → save settings.json
4. Print "Switched to account '<name>'"
```

**Account reset (`ccs account`):**
```
1. Load settings.json → clear env → save
2. Print "Reset to default account"
(oauthAccount in ~/.claude.json is left unchanged — user stays on whatever was last active)
```

## Critical Files to Modify

- `/Users/dt/code/claude-code-switcher/main.go` — all changes here

## Key Functions to Reuse / Update

- `loadSettings()` / `saveSettings()` — unchanged, still used for settings.json
- `loadProviderEnv()` — unchanged
- `mergeEnv()` — NOT used for account switching
- `removeEnv()` — reuse to clear env when switching accounts
- `getAccountsDir()` — already added, still needed
- `listProfiles()` — new
- `loadAccountProfile()` — new (replaces old `loadAccountEnv()`)
- `loadClaudeJson()` / `saveClaudeJson()` — new
- `ClaudeJson` struct — new

## Verification

1. **Capture current account as a profile:**
   ```bash
   mkdir -p ~/.claude/accounts
   # Copy oauthAccount from ~/.claude.json into a profile file
   cat ~/.claude.json | python3 -c "import sys,json; d=json.load(sys.stdin); print(json.dumps({'oauthAccount': d['oauthAccount']}, indent=2))" > ~/.claude/accounts/personal.json
   ```

2. **Switch to account:**
   ```bash
   ccs account personal
   # Expected: "Switched to account 'personal'"
   # ~/.claude.json should have updated oauthAccount
   # ~/.claude/settings.json should have no env block (or empty env)
   ```

3. **Verify provider env is cleared when switching accounts:**
   ```bash
   ccs glm              # Activate GLM provider (sets ANTHROPIC_AUTH_TOKEN + ANTHROPIC_BASE_URL in env)
   ccs account personal # Switch to account — should clear env entirely
   cat ~/.claude/settings.json  # env should be absent or empty
   ```

4. **Reset account:**
   ```bash
   ccs account
   # Expected: "Reset to default account"
   # env section cleared in settings.json; oauthAccount in ~/.claude.json unchanged
   ```

5. **List command:**
   ```bash
   ccs list
   # Should show providers (glm, local) and accounts (personal, work)
   ```

6. **Existing provider behavior unchanged:**
   ```bash
   ccs glm   # Should still work as before
   ccs       # Should still reset env
   ```
