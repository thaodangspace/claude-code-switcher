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
```

## How it works

`ccs` reads `~/.claude/<name>.json` and merges its `env` block into `~/.claude/settings.json`. Running `ccs` with no arguments removes the `env` key, restoring default behavior.
