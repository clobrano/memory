# memory

A terminal-based spaced-repetition tool for Obsidian-style markdown vaults.

Tag any note with `#study` and `memory` will schedule it for review using the SM-2 algorithm. An optional AI mode generates questions from the note content and evaluates your answers automatically.

## Install

```
go install github.com/clobrano/memory@latest
```

## Quick start

1. Edit the config file (created on first run at `~/.config/memory/config.toml`):

```toml
notes_dirs = ["~/notes"]
study_tags = ["#study"]
daily_limit = 20
```

2. Add `#study` to any markdown note in your vault.

3. Start a session:

```
memory study
```

## Commands

| Command | Description |
|---|---|
| `memory study [keywords]` | Start a study session (optional keyword filter) |
| `memory list [keywords]` | List all tracked notes with their next due date |
| `memory stats` | Show streak, retention rate, and a 14-day review chart |
| `memory sync` | Sync vault notes with the database without starting a session |
| `memory prompts reset` | Restore the built-in AI prompt templates |

`study` runs a sync automatically before each session, so `sync` is only needed outside of sessions.

## Review scheduling (SM-2)

Cards are scheduled using a variant of the SM-2 spaced-repetition algorithm. After each review you grade yourself:

| Grade | Effect |
|---|---|
| **All correct** | Interval grows (×ease factor), ease factor +0.1 |
| **Partially correct** | Interval grows, ease factor −0.15 |
| **Needs review** | Interval resets to 1 day, ease factor −0.2, rep count resets |

The first two reviews of a new card use fixed intervals (1 day, then 6 days for "all correct"; 1 day, then 4 days for "partially correct"). From the third review onward the interval is multiplied by the ease factor, which starts at 2.5 and is clamped to a minimum of 1.3.

A card appears in a session when its next due date is today or in the past.

## Session flow

```
Pre-session screen
  ↓ [Enter] start (capped to daily_limit)
  ↓ [a] review all  [Esc] quit

Recall — card title shown, try to recall the content
  ↓ [Enter] reveal note  [Esc] skip/quit

Reveal — full note content
  ↓ [Enter] grade  [Esc] skip/quit

Grading — self-grade: [1] all correct  [2] partially  [3] needs review
  ↓ next card (or session summary when done)
```

Pressing **Esc** at any point shows a prompt: `[y] Quit  [s] Skip card  [any] Cancel`. Skipped cards are not graded and do not affect scheduling.

## AI mode

Set `ai.binary` in the config to any CLI that reads a prompt from stdin and writes to stdout:

```toml
[ai]
binary = "claude"
args   = ["--model", "claude-haiku-4-5-20251001", "--print"]
```

With AI enabled the session flow changes:

```
Recall (AI questions shown instead of blank recall screen)
  → type your answers in the text area
  → [Enter] submit  [Esc] skip/quit

Reveal — note content shown; AI evaluates your answers in the background

Grading — AI suggests a grade with rationale
  → [a] accept  [o] override (manual grading)  [Esc] skip/quit
```

The AI also suggests note improvements below the questions when it finds gaps or ambiguities in the note content.

### Customising prompts

Prompt templates are written to `~/.config/memory/prompts/` on first run. Edit them to change how questions are generated or how answers are evaluated. To restore the defaults:

```
memory prompts reset
```

## Config reference

```toml
# Directories to scan for markdown notes. Required.
notes_dirs = ["~/notes"]

# Tags that mark a note as a study card. Default: ["#study"]
study_tags = ["#study"]

# Maximum cards per session. Default: 20
daily_limit = 20

[ai]
# CLI binary to use for AI mode. Leave unset to disable.
# binary = "claude"
# args   = ["--model", "claude-haiku-4-5-20251001", "--print"]

# Override the default prompt templates.
# question_prompt_file = ""
# evaluate_prompt_file = ""
```

## Data files

| Path | Contents |
|---|---|
| `~/.config/memory/config.toml` | Configuration |
| `~/.config/memory/prompts/` | AI prompt templates |
| `~/.local/share/memory/db.sqlite` | Card schedules and review history |

Both paths can be overridden with `--config` and `--db` flags.
