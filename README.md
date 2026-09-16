# Claude Code Switcher (ccs)

`ccs` switches between Claude Code provider profiles and Claude OAuth account profiles.

## Install

```sh
git clone https://github.com/thaodangspace/claude-code-switcher
cd claude-code-switcher
go build -o ccs .
mv ccs /usr/local/bin/ccs
```

## Storage and privacy

Provider/account metadata is stored in:

```text
~/.claude/ccs/<name>.json
```

OAuth secrets are never stored in that normal profile file. Account credential snapshots are stored separately in:

```text
~/.claude/ccs/credentials/<name>.json
```

On Unix, the snapshot directory is `0700` and snapshot files are `0600`. The active Claude credential container is read from `~/.claude/.credentials.json` on Linux/Windows. On macOS, CCS uses Claude Code's existing Keychain item (`Claude Code-credentials`) and only falls back to the file when that item is absent.

CCS preserves unknown credential fields and unrelated data such as `mcpOAuth` when replacing `claudeAiOauth`. Tokens are not printed by `ccs list`, `ccs list --json`, or normal error messages.

## Profiles

### Provider profile

Create `~/.claude/ccs/openrouter.json`:

```json
{
  "env": {
    "ANTHROPIC_API_KEY": "sk-or-v1-...",
    "ANTHROPIC_BASE_URL": "https://openrouter.ai/api/v1"
  }
}
```

### OAuth account profile

Account metadata contains identity information only:

```json
{
  "oauthAccount": {
    "accountUuid": "...",
    "emailAddress": "user@example.com",
    "organizationUuid": "...",
    "displayName": "User Name"
  }
}
```

Create or upgrade an account profile with the active Claude login:

```sh
ccs backup-account work
```

This saves metadata and a private `claudeAiOauth` snapshot separately.

## Usage

```sh
ccs openrouter                 # switch provider
ccs work                      # switch account and restore its OAuth credential
ccs backup-provider local     # save current provider environment
ccs backup-account work       # save current account metadata and credential
ccs list                      # list profiles and local OAuth health
ccs list --json               # machine-readable, secret-free listing
ccs current                   # show the active provider/account
ccs reset                     # clear provider overrides and account metadata
```

Account switching restores both `oauthAccount` metadata and the matching `claudeAiOauth` credential. Before switching away, CCS saves the currently active account's latest credential when its UUID matches a saved profile (email is used only when a UUID is unavailable). Provider switching behavior is unchanged.

## Offline OAuth health

`ccs list` performs no network requests, refreshes no tokens, and does not read the active credential store. It evaluates saved snapshots locally:

- **`ready`** — an access token exists and its local expiry is in the future.
- **`refresh-needed`** — access is missing/expired and a refresh token appears locally usable. CCS does not refresh it.
- **`re-login`** — access has expired and no locally usable refresh path remains.
- **`unknown`** — no snapshot exists or the saved credential cannot be safely parsed.

These are offline/local classifications, not server verification. A future local expiry does not prove that a token has not been revoked or invalidated by rotation.

Human output includes the account email, health state, and a short local expiry duration. JSON output has stable profile data and expiry timestamps, but never includes `accessToken`, `refreshToken`, or raw credential JSON. Example:

```json
{
  "providers": [{"name": "openrouter"}],
  "accounts": [{
    "name": "work",
    "email": "me@company.com",
    "accountUuid": "...",
    "active": true,
    "health": "ready",
    "accessExpiresAt": "2026-09-16T21:30:00Z",
    "detail": "access token valid"
  }]
}
```

## Migrating legacy account profiles

Older profiles may contain only `oauthAccount` metadata. They remain visible in `ccs list` as `unknown` with `credential not saved`, but CCS refuses to switch to them because metadata-only switching can pair one account with another account's token.

Upgrade one after logging into the desired account in Claude Code:

```sh
claude
# /login if needed
ccs backup-account work
```

CCS does not automatically associate an active credential with an arbitrary legacy profile.

## Isolated sessions

Run a provider profile in a temporary isolated configuration:

```sh
ccs run openrouter -- -p "What is the capital of France?"
```

`ccs run` currently supports provider profiles only.
