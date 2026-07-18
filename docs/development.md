# Development and release

## Local checks

```sh
gofmt -w cmd internal
go test ./...
go vet ./...
go build ./cmd/prompt-cli
```

A small, reproducible release build:

```sh
CGO_ENABLED=0 go build \
  -trimpath \
  -ldflags="-s -w -X github.com/juanandrade/prompt-cli/internal/app.Version=v0.6.1" \
  -o dist/prompt-cli \
  ./cmd/prompt-cli
```

On Windows, use `dist/prompt-cli.exe`. Dependencies are pure Go; the application has no external assets or runtime downloads.

## GitHub Actions and releases

GitHub Actions contains two workflows:

- [CI](../.github/workflows/ci.yml) runs `go test ./...`, `go vet ./...`, and `go build ./cmd/prompt-cli` on Linux and Windows for pull requests and pushes to `main`.
- [Release](../.github/workflows/release.yml) runs the same checks and publishes a GitHub Release when a tag matching `v*` is pushed.

The release workflow builds the tag version into the binary, then uploads these amd64 assets:

```text
prompt-cli_vX.Y.Z_linux_amd64.tar.gz
prompt-cli_vX.Y.Z_linux_amd64.tar.gz.sha256
prompt-cli_vX.Y.Z_windows_amd64.zip
prompt-cli_vX.Y.Z_windows_amd64.zip.sha256
```

Create and publish a future release only after the intended commit is on `main`:

```sh
git tag -a vX.Y.Z -m "vX.Y.Z"
git push origin vX.Y.Z
```

The workflow uses the repository `GITHUB_TOKEN` with `contents: write` only in its release job. No personal access token is required for normal GitHub Releases.

## Performance decisions

- TUI snapshots and semantic analysis use a 75 ms debounce;
- visual validation reuses status for 2 s; LSP hover/definition/semantic tokens remain fresh;
- completion builds the index lazily and reuses it for 2 s;
- breadth-first workspace scanning is limited to 50,000 metadata entries;
- responses are limited to 500 candidates;
- the renderer keeps only the viewport and produces one write per frame;
- an open document is not read from disk again;
- navigation validation stays fresh by design.

The line/rune buffer favors simplicity. Before adding a rope or piece table, benchmark real Prompt Lang files; a more complex structure is justified only if benchmarks demonstrate material latency.

## Known limitations

- There is no undo/redo, mouse support, multiple buffers, or soft wrap.
- Structural highlighting covers the relevant Markdown blocks; unavailable embedded-language highlighting falls back to generic code styling.
- The index is TTL-based and has no filesystem watcher.
- Saving on filesystems or platforms without atomic `rename` replacement inherits the local `os.Rename` behavior.
- The Liquid Cursor smoke test depends on pnex.

These limitations do not alter the documented Prompt Lang grammar or IntelliSense; they are intentional omissions from the initial small binary.
