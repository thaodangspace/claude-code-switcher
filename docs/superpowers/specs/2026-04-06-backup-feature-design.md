# Backup Provider/Account Feature Design

**Date:** 2026-04-06
**Project:** claude-code-switcher (ccs)

## Summary

Add two new commands to `ccs` that allow users to save their current provider environment or OAuth account configuration as reusable profiles in `~/.claude/ccs/`.

## Commands

```
ccs backup-provider <name>    Save current provider env as profile
ccs backup-account <name>     Save current oauthAccount as profile
```

## Usage Examples

```sh
# Save current ANTHROPIC_* env vars as "mykey" profile
ccs backup-provider mykey

# Save current logged-in account as "work" profile
ccs backup-account work
```

## Behavior

### `backup-provider`

1. Load `~/.claude/settings.json`
2. Extract only keys starting with `ANTHROPIC_` from the `env` block
3. If no `ANTHROPIC_*` keys found → error
4. Create profile JSON: `{ "env": { ... extracted keys ... } }`
5. If `~/.claude/ccs/<name>.json` exists → print overwrite warning
6. Save to `~/.claude/ccs/<name>.json`
7. Print success message

### `backup-account`

1. Load `~/.claude.json`
2. Check if `oauthAccount` exists → error if missing
3. Create profile JSON: `{ "oauthAccount": { ... full object ... } }`
4. If `~/.claude/ccs/<name>.json` exists → print overwrite warning
5. Save to `~/.claude/ccs/<name>.json`
6. Print success message

## Output Messages

| Outcome | Message |
|--------|---------|
| Success (provider) | `Saved provider '<name>' to ~/.claude/ccs/<name>.json` |
| Success (account) | `Saved account '<name>' to ~/.claude/ccs/<name>.json` |
| Overwrite warning | `Warning: Overwriting existing profile '<name>'` |
| Missing name arg | `Error: Usage: ccs backup-provider <name>` or `Error: Usage: ccs backup-account <name>` |
| No provider env | `Error: No ANTHROPIC_* env vars found` |
| No OAuth account | `Error: No OAuth account found in ~/.claude.json` |

All errors print to stderr and exit with code 1.

## Implementation

### New File: `backup.go`

```go
// backupProviderCmd saves current ANTHROPIC_* env vars to a named profile.
func backupProviderCmd(claudeDir string, ccsDir string, name string) error

// backupAccountCmd saves current oauthAccount to a named profile.
func backupAccountCmd(ccsDir string, name string) error

// filterAnthropicEnv extracts only ANTHROPIC_* keys from an env map.
func filterAnthropicEnv(env map[string]interface{}) map[string]interface{}
```

### Modified File: `main.go`

Add cases in the switch block:

```go
case "backup-provider":
    if len(args) < 2 {
        fmt.Fprintf(os.Stderr, "Error: Usage: ccs backup-provider <name>\n")
        os.Exit(1)
    }
    backupProviderCmd(claudeDir, ccsDir, args[1])

case "backup-account":
    if len(args) < 2 {
        fmt.Fprintf(os.Stderr, "Error: Usage: ccs backup-account <name>\n")
        os.Exit(1)
    }
    backupAccountCmd(ccsDir, args[1])
```

Update `printUsage()` to include new commands.

## Testing

Manual testing scenarios:

1. Backup provider with valid `ANTHROPIC_*` env → saves correctly
2. Backup provider with no `env` in settings → error
3. Backup provider with `env` but no `ANTHROPIC_*` keys → error
4. Backup provider when profile already exists → warning + saves
5. Backup account with valid `oauthAccount` → saves correctly
6. Backup account with no `oauthAccount` → error
7. Backup account when profile already exists → warning + saves
8. Verify saved profiles work with `ccs <name>` switch command
9. Run without name argument → usage error