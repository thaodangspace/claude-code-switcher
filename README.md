# claude-code-switcher

Switch between API provider configurations for Claude Code.

## Install

```sh
git clone https://github.com/thaodangspace/claude-code-switcher
cd claude-code-switcher
go build -o ccs .
mv ccs /usr/local/bin/ccs
```

## Setup

Create a config file for each provider at `~/.claude/<name>.json`:

```json
{
  "env": {
    "ANTHROPIC_API_KEY": "sk-ant-...",
    "ANTHROPIC_BASE_URL": "https://api.example.com"
  }
}
```

Example: `~/.claude/openrouter.json`, `~/.claude/bedrock.json`

## Usage

```sh
# Switch to a provider
ccs openrouter

# Reset to default (removes env overrides)
ccs

# Switch to a claude.ai account (Pro/Team)
ccs account personal

# Reset to default claude.ai account
ccs account

# List available providers and accounts
ccs list
```

## Provider Setup

Create a config file for each provider at `~/.claude/<name>.json`:

```json
{
  "env": {
    "ANTHROPIC_API_KEY": "sk-ant-...",
    "ANTHROPIC_BASE_URL": "https://api.example.com"
  }
}
```

Example: `~/.claude/openrouter.json`, `~/.claude/bedrock.json`

## Account Setup

Create account profiles at `~/.claude/accounts/<name>.json`. Copy the `oauthAccount` object from `~/.claude.json`:

```json
{
  "oauthAccount": {
    "accountUuid": "9a88e9bb-...",
    "emailAddress": "user@example.com",
    "organizationUuid": "6fa74e61-...",
    "displayName": "Alice",
    "organizationRole": "admin",
    "organizationName": "My Org",
    "billingType": "stripe_subscription"
  }
}
```

## How it works

- **Provider mode**: `ccs <provider>` reads `~/.claude/<name>.json` and merges its `env` block into `~/.claude/settings.json`
- **Account mode**: `ccs account <name>` updates `~/.claude.json`'s `oauthAccount` field and clears env from settings
- **Reset**: Running `ccs` or `ccs account` with no arguments removes the `env` key, restoring default behavior
