---
name: "codegraph"
description: "Install CodeGraph (colbymchenry/codegraph), index a codebase, and use its graph/MCP tools for structural questions: callers, call graphs, impact of a change, architecture."
---

# CodeGraph: install, index, query

CodeGraph builds a local, pre-indexed knowledge graph of a repo (symbols, call graph, imports, structure) and serves it to agents over MCP. Use it to answer structural questions in one call instead of many grep/read rounds.

There are several unrelated projects named "codegraph". This skill targets **colbymchenry/codegraph** (npm `@colbymchenry/codegraph`, index in `<repo>/.codegraph/`). If `codegraph --version` or `codegraph --help` shows a different tool, stop and tell the user rather than guessing at commands.

## When to use
- "Who calls X?", "what does X depend on?", "what breaks if I change X?"
- Getting oriented in an unfamiliar or large repo
- Planning a refactor or rename across many files
- Tracing a request/data flow end to end

Don't bother for a one-file edit, a known path, or a literal string search — plain grep/read is faster there.

## 1. Check what's already there
```bash
command -v codegraph && codegraph --version
ls -d .codegraph 2>/dev/null   # run in repo root
```
- CLI present + `.codegraph/` present → skip to step 4.
- CLI present, no index → step 3.
- No CLI → step 2.

## 2. Install the CLI (ask before installing)
Installing software is a change to the user's machine — confirm first, then pick one:
```bash
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/colbymchenry/codegraph/main/install.sh | sh
# Windows (PowerShell)
irm https://raw.githubusercontent.com/colbymchenry/codegraph/main/install.ps1 | iex
# or via npm
npm install -g @colbymchenry/codegraph
```
Then wire it into the agent(s):
```bash
codegraph install --yes                         # auto-detect agents, global
codegraph install --target=claude --yes         # Claude Code only
codegraph install --target=auto --location=local  # project-local config
codegraph install --print-config claude         # show snippet, write nothing
```
`codegraph install` only configures the MCP server, instructions and permissions; it does **not** index code. Prefer `--print-config` if the user wants to review config changes before anything is written.

Manual fallback (Claude Code): add an MCP server named `codegraph` running `codegraph serve --mcp`, e.g.
```bash
claude mcp add --transport stdio codegraph -- codegraph serve --mcp
```
The agent must be **restarted** before the MCP tools appear. Tell the user this explicitly.

## 3. Index the repo
From the repo root:
```bash
codegraph init -i      # create .codegraph/ and build the initial graph
```
- `codegraph init` alone only creates the directory; `-i` builds the graph.
- Large repos take a while — run it in the background and report when done.
- Add `.codegraph/` to `.gitignore` unless the user wants it committed.
- Optional: `codegraph hooks install` for a git auto-sync hook. The MCP server also watches files and re-syncs while running.

## 4. Use it
**Prefer the MCP tools** (they appear as `codegraph`-prefixed tools after restart). List them and read their descriptions rather than assuming names — they change between versions.

Working pattern:
1. Start broad: ask the graph for the symbol/entry point, its callers and callees.
2. Use impact/blast-radius style queries before editing a widely used symbol.
3. Read only the files the graph points to — don't re-grep what the graph already answered.
4. After edits, trust the watcher/hook to resync; if results look stale, re-run indexing.
5. If the graph has no result (dynamic dispatch, reflection, string-based routing, generated code), fall back to grep and say so — the graph is static.

CLI only (no MCP available): run `codegraph --help` and use its query subcommands; parse output, don't paste huge dumps into the reply.

## 5. Report
Answer the user's question with file:line references from the graph. Mention any gaps (unindexed languages, dynamic calls) that could make the answer incomplete.

## Troubleshooting
- MCP tools missing → agent not restarted, or config written to another scope; run `codegraph install --print-config claude` and compare.
- `command not found` after install → install dir not on PATH; open a new shell or add it.
- Stale/odd results → re-run `codegraph init -i`.
- Remove everything: `codegraph uninstall` (add `--keep-cli` to keep the binary). Confirm with the user first.
