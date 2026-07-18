# AI harness integration

`prompt-cli` is an interactive terminal editor. Put its executable on `PATH`, then configure a harness to invoke it as an external editor where that harness supports one.

```sh
export VISUAL="prompt-cli --wait"
export EDITOR="prompt-cli --wait"
```

`--wait` is accepted for compatibility; `prompt-cli` always waits for the editing session to finish. The editor needs an interactive terminal. `Ctrl+S` saves and exits successfully; `Ctrl+X` or `Ctrl+C` ask for confirmation before discarding changes.

> **File-name requirement:** file-backed editing intentionally accepts only `.md` and `.prompt` files. Before relying on an external-composer integration, check that the installed harness gives its temporary draft one of those suffixes. Otherwise, open a compatible prompt file directly with `prompt-cli` instead.

The external-editor flow below is not LSP integration. `prompt-cli lsp --workspace DIR` starts a language server for an LSP client; it does not replace a harness's chat composer. See [LSP integration](lsp.md).

## Claude Code

Claude Code can open a draft in the configured external editor. Set `VISUAL` and `EDITOR` **before** starting Claude Code:

```sh
export VISUAL="prompt-cli --wait"
export EDITOR="prompt-cli --wait"
claude
```

In a session, use `Ctrl+G` when Claude Code offers to edit a plan or prompt in an editor. This edits the composer/plan draft; it does not change how Claude Code's Edit and Write tools modify repository files.

On Windows PowerShell, use an absolute executable path when it is not on `PATH`:

```powershell
$env:VISUAL = "C:\Tools\prompt-cli\prompt-cli.exe --wait"
$env:EDITOR = $env:VISUAL
claude
```

References: [Claude Code interactive mode](https://code.claude.com/docs/en/interactive-mode) and [environment variables](https://code.claude.com/docs/en/env-vars).

## Codex CLI

Codex CLI can hand the current composer draft to an external editor with `Ctrl+G`. Configure the conventional editor variables before launching it:

```sh
export VISUAL="prompt-cli --wait"
export EDITOR="prompt-cli --wait"
codex
```

This affects the composer handoff only. It does not configure the Codex IDE extension or turn `prompt-cli` into a Codex language server. Apply the [file-name requirement](#ai-harness-integration) above to temporary drafts.

References: [Codex CLI](https://developers.openai.com/codex/cli) and [command reference](https://developers.openai.com/codex/cli/reference).

## OpenCode

OpenCode explicitly uses `EDITOR` for external editing. Configure it before launching OpenCode:

```sh
export EDITOR="prompt-cli --wait"
opencode
```

Then use `/editor` (or `Ctrl+X E`) to compose a message in `prompt-cli`. `/export` also uses `EDITOR`, but opens an exported conversation rather than a new message draft.

Windows setup:

```cmd
set EDITOR=C:\Tools\prompt-cli\prompt-cli.exe --wait
opencode
```

```powershell
$env:EDITOR = "C:\Tools\prompt-cli\prompt-cli.exe --wait"
opencode
```

Reference: [OpenCode TUI editor setup](https://opencode.ai/docs/tui/#editor-setup).

## Cursor

Cursor is itself an editor and does not document a generic setting that replaces its Agent composer with an arbitrary terminal editor. The supported, simple workflow is to run `prompt-cli` in Cursor's integrated terminal:

```sh
prompt-cli prompts/review.prompt.md
```

Save the prompt, then reference, paste, or otherwise use the saved file in Cursor. `cursor --wait` is the opposite integration: it lets another program wait for Cursor, not for `prompt-cli`.

The built-in `prompt-cli lsp` server can be used by an LSP client, but the project does not ship a Cursor/VS Code extension. Do not add an invented Cursor settings entry: registering a stdio server requires an extension or another LSP client integration.

References: [Cursor documentation](https://cursor.com/docs) and [Cursor CLI](https://cursor.com/docs/cli).

## pnex

pnex is the terminal host, so no editor-plugin configuration is necessary. Run `prompt-cli` in a pnex terminal:

```sh
prompt-cli prompts/review.prompt.md
```

`prompt-cli` keeps the real VT cursor aligned with the logical caret, allowing pnex Liquid Cursor to follow it. pnex normally provides `PNEX=1`; setting that variable manually is only useful when running the repository's smoke-test path and does not enable Liquid Cursor by itself:

```sh
PNEX=1 prompt-cli prompts/review.prompt.md
```

For the rendering contract and manual verification checklist, see [pnex Liquid Cursor compatibility](liquid-cursor.md) and its [normative reference](reference/liquid-cursor-tui-compatibility.md).
