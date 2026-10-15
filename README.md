# prompt-cli

A small, fast terminal editor for Markdown prompts in AI coding harnesses.

`prompt-cli` is a single Go binary: it provides a focused terminal UI, workspace-aware Prompt Lang assistance, and an optional LSP server over stdio. Use it to edit a prompt file, compose a harness message in `$EDITOR`, or return an untitled prompt through stdout.

## Why prompt-cli

- **Simple by design:** one static-friendly binary, no runtime downloads, network access, subprocesses, or persistent cache.
- **Made for terminals:** Unicode/grapheme-aware editing, vertical and horizontal scrolling, atomic saves, and a real VT caret.
- **Prompt-aware:** Markdown highlighting plus completion for `@` workspace references and `[Heading]` references.
- **Harness-friendly:** blocks until editing ends, accepts `--wait` for editor-command compatibility, and supports untitled stdout output.
- **Editor-ready:** the same binary exposes a stdio LSP for compatible clients.

## Install

### GitHub Release (Linux amd64 and Windows amd64)

Each version tag publishes a GitHub Release with a compressed binary and a matching SHA-256 file. The current release is `v0.6.1`.

**Linux**

```sh
VERSION=v0.6.1
asset="prompt-cli_${VERSION}_linux_amd64.tar.gz"
curl -fLO "https://github.com/juanandrade/prompt-cli/releases/download/${VERSION}/${asset}"
tar -xzf "$asset"
install -m 0755 prompt-cli ~/.local/bin/prompt-cli
prompt-cli version
```

**Windows PowerShell**

```powershell
$version = "v0.6.1"
$asset = "prompt-cli_$($version)_windows_amd64.zip"
Invoke-WebRequest "https://github.com/juanandrade/prompt-cli/releases/download/$version/$asset" -OutFile $asset
Expand-Archive $asset -DestinationPath "$HOME\bin\prompt-cli" -Force
& "$HOME\bin\prompt-cli\prompt-cli.exe" version
```

Add the selected directory to `PATH`. Check the downloaded archive against its adjacent `.sha256` asset before installing it.

### Build from source

Requires Go 1.23 or newer.

```sh
git clone https://github.com/juanandrade/prompt-cli.git
cd prompt-cli
go build -trimpath -ldflags="-s -w" -o prompt-cli ./cmd/prompt-cli
```

Use `-o prompt-cli.exe` on Windows.

## Quick start

Open an existing prompt:

```sh
prompt-cli prompt.md
```

Open or create a file at a position:

```sh
prompt-cli edit --workspace /path/to/repo --line 12 --column 4 prompts/review.prompt.md
```

Use an editor-style file location:

```sh
prompt-cli --goto prompts/review.prompt.md:12:4
```

Start with an untitled buffer:

```sh
prompt-cli
```

In untitled mode, `Ctrl+S` exits and writes only the Markdown to stdout. The UI writes to stderr, so stdout remains suitable for a pipeline:

```sh
prompt="$(prompt-cli)"
```

## AI harness integration

Put `prompt-cli` on `PATH` before launching a harness. It requires an interactive terminal and waits until the editor closes; `Ctrl+S` saves and exits with status 0, while confirmed cancellation exits with status 130. File-backed editing accepts only `.md` and `.prompt` filenames.

| Harness | Supported integration |
| --- | --- |
| [Claude Code](docs/harnesses.md#claude-code) | External composer editor through `VISUAL` / `EDITOR`; use `Ctrl+G`. |
| [Codex CLI](docs/harnesses.md#codex-cli) | External composer editor through `VISUAL` / `EDITOR`; use `Ctrl+G`. |
| [OpenCode](docs/harnesses.md#opencode) | External editor through `EDITOR`; use `/editor` or `Ctrl+X E`. |
| [Cursor](docs/harnesses.md#cursor) | Run it from Cursor's integrated terminal; no generic external-composer setting is documented. |
| [pnex](docs/harnesses.md#pnex) | Run it in the pnex terminal; the VT caret supports Liquid Cursor tracking. |

For full commands, limitations, and official references, see **[Harness integration](docs/harnesses.md)**. The external-editor flow is separate from the LSP server.

## Command line

```text
prompt-cli [edit] [--workspace DIR] [--line N --column N] [FILE]
prompt-cli --goto FILE:LINE[:COLUMN]
prompt-cli lsp [--workspace DIR]
prompt-cli version
prompt-cli help
```

| Option or command | Description |
| --- | --- |
| `edit` | Optional explicit name for the default editor mode. |
| `--workspace DIR` | Base for `@` references only in untitled mode; defaults to the current directory. Files always use their own directory. |
| `--line N`, `--column N` | Initial one-based caret location; both default to 1. |
| `--goto` | Parse `FILE:LINE[:COLUMN]`, including a Windows drive path. |
| `--wait` | Accepted for editor-command compatibility; the process always waits. |
| `lsp` | Run the embedded LSP server over stdio. |
| `version` / `--version` | Print the build version. |
| `help` / `--help` / `-h` | Print usage. |

Supply at most one file. A nonexistent file stays in memory until its first save.

## Key bindings

| Key | Action |
| --- | --- |
| `Ctrl+S` | Save/confirm and exit by default; stay open when Keep open is ON. In untitled mode, write Markdown to stdout only on exit. |
| `Ctrl+Shift+S` | Save a file without closing the editor. Untitled buffers remain open and unchanged. |
| `Ctrl+K` | Toggle Keep open (OFF by default, current session only). State and shortcut remain visible in the status bar. |
| `Ctrl+X` / `Ctrl+C` | Ask before discarding changes; confirmed cancellation exits with code 130. |
| `Ctrl+Space` | Open or refresh completion. |
| `Ctrl+G` | Open Go to Line; enter `line` or `line:column`, then press Enter. |
| `Tab` / `Enter` | Accept completion, or insert a tab/newline when no popup is open. |
| `Esc` | Close completion, Go to Line, or cancel a discard prompt. |
| Arrow keys | Move the caret or completion selection. |
| `Ctrl+←` / `Ctrl+→` or `Alt+←` / `Alt+→` | Move by word. |
| `Alt+↑` / `Alt+↓` | Move the current line up/down. |
| `Home` / `End` | Move to the start/end of the line. |
| `Ctrl+Home` / `Ctrl+End` | Move to the start/end of the document. |
| `Backspace` / `Delete` | Delete a grapheme or join lines. |
| `Ctrl+Backspace` / `Alt+Backspace` / `Ctrl+W` | Delete the previous word. |
| `Ctrl+Delete` | Delete the rest of the current/next word. |
| `Ctrl+Enter` / `Shift+Enter` | Insert an indented line below/above without changing the current line. |
| `Page Up` / `Page Down` | Move by one viewport. |

Completion also opens after `@`, `[`, or `/` in a valid context.

With Keep open ON, `Ctrl+S` saves files without exiting, even if the terminal sends the same key code for `Ctrl+Shift+S`. Untitled buffers stay in memory without writing to disk or stdout; turn Keep open OFF with `Ctrl+K`, then press `Ctrl+S` to confirm and exit.

## Prompt Lang support

Prompt-specific semantics are enabled, case-insensitively, for:

- `prompt.md` and `prompts.md`;
- `*.prompt.md` and `*.prompts.md`;
- `*.prompt`.

Other Markdown files retain normal editing, Markdown highlighting, and ATX-heading completion, but not Prompt Lang references or prompt-item semantics.

```md
Read @src/main.go

# Review
## Style
Use [Review > Style]

- Check error handling
```

File references resolve from the directory of the document being edited, including `../` paths outside the workspace. Untitled buffers use `--workspace` or the current directory. Heading references target ATX headings in the current document. Read [Prompt Lang](docs/language.md) for activation rules, syntax, matching behavior, and limits.

## LSP

Run the embedded server over stdio:

```sh
prompt-cli lsp --workspace /path/to/repo
```

A compatible client launches it with this conceptual configuration:

```json
{
  "command": "/absolute/path/prompt-cli",
  "args": ["lsp", "--workspace", "/absolute/path/workspace"]
}
```

The server supports full-document synchronization, completion, hover, definition, and full semantic tokens. It does not provide diagnostics, formatting, rename, code actions, or incremental semantic tokens. See [LSP integration](docs/lsp.md) for protocol details, file associations, and semantic-token types.

## Limitations

- No undo/redo, mouse support, multiple buffers, or soft wrap.
- No filesystem watcher; the workspace index uses a short TTL.
- Workspace scanning is limited to 50,000 metadata entries and completion to 500 candidates.
- Fenced code blocks without an available language highlighter use generic code styling.
- Atomic replacement follows the local filesystem behavior of `os.Rename`.
- Liquid Cursor visual smoke testing requires pnex, although the editor works without it.

## Documentation

- [Harness integration](docs/harnesses.md)
- [Usage and key bindings](docs/usage.md)
- [Prompt Lang](docs/language.md)
- [LSP integration](docs/lsp.md)
- [Architecture](docs/architecture.md)
- [Development and release](docs/development.md)
- [pnex Liquid Cursor compatibility](docs/liquid-cursor.md)
- [Reference contracts](docs/reference/)

## Development

```sh
gofmt -w cmd internal
go test ./...
go vet ./...
go build ./cmd/prompt-cli
```

The CI workflow validates tests, vet, and builds on Linux and Windows. Pushing a `v*` tag builds Linux/Windows amd64 archives, generates SHA-256 files, and publishes a GitHub Release.
