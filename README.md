# prompt-cli

Editor TUI rápido para **Markdown Prompt Lang**, escrito em Go e entregue como um único binário. O mesmo executável também expõe um servidor LSP por `stdio`.

## Recursos

- números de linha, scroll vertical/horizontal e edição Unicode por grapheme;
- `Ctrl+S` salva/confirma e `Ctrl+X` cancela sem persistir;
- popup de IntelliSense para `@arquivos` e `[Headings]`;
- highlighting Markdown e semântico para referências, headings e prompt items;
- completion, hover, definition e semantic tokens via LSP embutido;
- índice BFS do workspace em cache por 2 segundos, limitado e determinístico;
- cursor VT real posicionado no caret em todo frame, compatível com pnex Liquid Cursor;
- gravação por arquivo temporário + rename, sem processo auxiliar, rede ou cache persistente.

## Início rápido

```sh
go build -trimpath -ldflags="-s -w" -o prompt-cli ./cmd/prompt-cli
./prompt-cli prompt.md
```

```text
prompt-cli [edit] [--workspace DIR] [--line N --column N] [FILE]
prompt-cli --goto FILE:LINE[:COLUMN]
prompt-cli lsp [--workspace DIR]
```

O editor aceita apenas arquivos `.md` e `.prompt`. Consulte [docs/usage.md](docs/usage.md) e [docs/architecture.md](docs/architecture.md).

## Desenvolvimento

```sh
go test ./...
go vet ./...
go build ./cmd/prompt-cli
```

Requer Go 1.23 ou mais recente. O teste visual definitivo do cursor líquido deve ser executado dentro do pnex com `PNEX=1`.
