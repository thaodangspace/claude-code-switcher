# Claude Code Switcher (ccs)

`ccs` is a CLI tool to effortlessly switch between API provider configurations and OAuth accounts for [Claude Code](https://claude.ai/code). It allows you to maintain multiple profiles and apply them globally or use them in isolated sessions.

## Features

- **Consolidated Management**: Store all provider and account profiles in `~/.claude/ccs/`.
- **Global Switching**: Switch your primary Claude configuration with a single command.
- **Isolated Sessions**: Run `claude` with a specific provider configuration without affecting your global settings.
- **Auto-Detection**: Distinguishes between API providers (Bedrock, OpenRouter, etc.) and claude.ai OAuth accounts automatically.

## Install

```sh
# Clone the repository
git clone https://github.com/thaodangspace/claude-code-switcher
cd claude-code-switcher

# Build and install
go build -o ccs .
mv ccs /usr/local/bin/ccs
```

## Setup

Create a profile JSON file for each provider or account in `~/.claude/ccs/`.

### Provider Profile
For non-standard API providers, create a profile with an `env` block.
Example: `~/.claude/ccs/openrouter.json`

```json
{
  "env": {
    "ANTHROPIC_API_KEY": "sk-or-v1-...",
    "ANTHROPIC_BASE_URL": "https://openrouter.ai/api/v1"
  }
}
```

### Account Profile
For switching between different `claude.ai` accounts (e.g., Personal vs. Work), create a profile with an `oauthAccount` block. You can find your current account data in `~/.claude.json`.
Example: `~/.claude/ccs/personal.json`

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

## Usage

### Global Switch
Switch your global Claude configuration to a specific profile:

```sh
# Switch to a provider profile
ccs openrouter

# Switch to an account profile
ccs personal
```

### Isolated Session
Run a parallel Claude session with a specific provider configuration. This creates a temporary sandbox for Claude that is automatically cleaned up after exit.

```sh
# Run with openrouter provider and pass arguments to claude
ccs run openrouter -- -p "What is the capital of France?"
```
*Note: `ccs run` currently only supports provider profiles.*

### Management
```sh
# List all available profiles
ccs list

# Show the currently active profile
ccs current

# Reset to default (removes env overrides and clears account)
ccs reset
```

## How it works

- **Provider Mode**: When switching to a provider, `ccs` merges the `env` block into `~/.claude/settings.json` and clears the OAuth account in `~/.claude.json`.
- **Account Mode**: When switching to an account, `ccs` updates `~/.claude.json` with the `oauthAccount` data and clears any `env` overrides in `~/.claude/settings.json`.
- **Run Mode**: Creates a temporary directory, populates it with the target configuration, and executes `claude` with `CLAUDE_CONFIG_DIR` pointing to the temporary path.
