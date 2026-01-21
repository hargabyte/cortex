# Command Reference

Complete reference for all Cortex (cx) commands.

## Context Assembly

| Command | Purpose |
|---------|---------|
| `cx context` | Session recovery / orientation |
| `cx context --smart "task description" --budget N` | Task-focused context (most useful) |
| `cx context --diff` | Context for uncommitted changes |
| `cx context <entity> --hops 2` | Entity-focused context |

## Discovery

| Command | Purpose |
|---------|---------|
| `cx find <name>` | Search by name (prefix match) |
| `cx find "concept query"` | Full-text concept search |
| `cx find --semantic "query"` | Semantic search via embeddings |
| `cx find --keystones` | Most-depended-on entities |
| `cx find --important --top N` | Top N by PageRank |
| `cx find --type F\|T\|M\|C` | Filter by type (Function/Type/Method/Constant) |
| `cx find --lang <language>` | Filter by language |

## Analysis

| Command | Purpose |
|---------|---------|
| `cx show <entity>` | Entity details + dependencies |
| `cx show <entity> --related` | + neighborhood exploration |
| `cx show <entity> --graph --hops N` | Dependency graph visualization |
| `cx safe <file>` | Pre-flight safety assessment |
| `cx safe --quick` | Just blast radius |
| `cx safe --coverage --keystones-only` | Coverage gaps in critical code |
| `cx trace <from> <to>` | Find call path between entities |
| `cx dead` | Find unreachable code |

## Project Overview

| Command | Purpose |
|---------|---------|
| `cx map` | Project skeleton (~10k tokens) |
| `cx map <path>` | Skeleton of specific directory |
| `cx map --filter F` | Just functions |
| `cx db info` | Database statistics |
| `cx status` | Daemon and graph status |

## Testing

| Command | Purpose |
|---------|---------|
| `cx test --diff` | Show tests affected by changes |
| `cx test --diff --run` | Run affected tests |
| `cx coverage import <file>` | Import coverage data |

## Reports

| Command | Purpose |
|---------|---------|
| `cx report overview --data` | System architecture with D2 diagram |
| `cx report feature <query> --data` | Feature deep-dive with call flow |
| `cx report changes --since <ref> --data` | What changed (Dolt time-travel) |
| `cx report health --data` | Risk analysis and recommendations |
| `cx report --init-skill` | Generate Claude Code skill for reports |
| `cx render <file.d2> -o <file.svg>` | Render D2 diagram to SVG |

See [Report Generation](reports.md) for detailed usage.

## Maintenance

| Command | Purpose |
|---------|---------|
| `cx scan` | Build/update the code graph |
| `cx scan --force` | Full rescan |
| `cx scan --tag <name>` | Tag this scan for future reference |
| `cx doctor` | Health check |
| `cx doctor --fix` | Auto-fix issues |
| `cx reset` | Reset database |

## Version Control (Dolt-Powered)

| Command | Purpose |
|---------|---------|
| `cx history` | View scan commit history |
| `cx history --limit N` | Last N scans |
| `cx diff` | Show uncommitted changes |
| `cx diff HEAD~1` | Changes since previous scan |
| `cx diff HEAD~5 HEAD` | Changes over last 5 scans |
| `cx diff --entity <name>` | Filter to specific entity |
| `cx blame <entity>` | When/why entity changed |
| `cx branch` | List Dolt branches |
| `cx branch <name>` | Create branch |
| `cx branch -c <name>` | Checkout branch |
| `cx sql "<query>"` | Direct SQL passthrough |
| `cx rollback` | Undo last scan |
| `cx rollback HEAD~N` | Rollback to specific point |

## Time Travel

| Command | Purpose |
|---------|---------|
| `cx show <entity> --at HEAD~5` | Entity state 5 commits ago |
| `cx show <entity> --at <tag>` | Entity at tagged release |
| `cx show <entity> --history` | Entity evolution over time |
| `cx find <name> --at <ref>` | Search at historical point |
| `cx safe <file> --trend` | Blast radius trend over time |

## Agent Optimization

| Command | Purpose |
|---------|---------|
| `cx stale` | Check if graph needs refresh |
| `cx stale --scans N` | Entities unchanged for N+ scans |
| `cx catchup` | Rescan and show what changed |
| `cx catchup --summary` | Brief change summary |

## Entity Tagging

| Command | Purpose |
|---------|---------|
| `cx tag add <entity> <tags...>` | Tag an entity |
| `cx tag find <tag>` | Find tagged entities |
| `cx tag find <tag1> <tag2> --all` | Find entities with ALL tags |
| `cx tag find <tag1> <tag2> --any` | Find entities with ANY tag |
| `cx link <entity> <url>` | Link to external system |

## Output Formats

```bash
--format yaml    # Default, human-readable
--format json    # Structured, for parsing
--format jsonl   # Line-delimited JSON

--density sparse   # Minimal (50-100 tokens per entity)
--density medium   # Default (200-300 tokens)
--density dense    # Full detail (400-600 tokens)
```
