# arc-sessions

Manage AI agent sessions for the Arc toolkit.

## Features

- **list** - List all sessions
- **find** - Find sessions by criteria
- **show** - Show session details
- **archive** - Archive old sessions

## Installation

```bash
go install github.com/mtreilly/arc-sessions@latest
```

## Usage

```bash
# List recent sessions
arc-sessions list

# List sessions for a specific agent
arc-sessions list --agent claude

# Find sessions
arc-sessions find --query "refactor"

# Show session details
arc-sessions show <session-id>

# Archive old sessions
arc-sessions archive --older-than 30d
```

## Description

Sessions are indexed conversations with Claude, Codex, or other AI agents. Use this to find previous sessions, resume conversations, or archive old ones.

## License

MIT
