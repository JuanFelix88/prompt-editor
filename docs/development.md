# Desenvolvimento e release

## Checks

```sh
gofmt -w cmd internal
go test ./...
go vet ./...
go build ./cmd/prompt-cli
```

Build enxuto e reproduzível:

```sh
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X github.com/juanandrade/prompt-cli/internal/app.Version=0.1.0" -o dist/prompt-cli ./cmd/prompt-cli
```

No Windows, use `dist/prompt-cli.exe`. Dependências são pure Go; não há assets externos nem downloads em runtime.

## Decisões de performance

- snapshot e análise semântica da TUI recebem debounce de 75 ms;
- validação visual reutiliza status por 2 s; LSP hover/definition/semantic continuam frescos;
- completion constrói o índice de forma lazy e o reutiliza por 2 s;
- varredura BFS limitada a 50.000 metadados;
- resposta limitada a 500 candidatos;
- renderer mantém somente viewport e constrói um write por frame;
- documento aberto não é relido do disco;
- validação de navegação continua fresca por design.

O buffer de linhas/runes privilegia simplicidade. Antes de introduzir rope/piece table, meça arquivos Prompt Lang reais; uma estrutura mais complexa só é justificável se benchmarks demonstrarem latência relevante.

## Limitações conhecidas

- Não há undo/redo, mouse, múltiplos buffers ou soft wrap.
- Highlighting estrutural cobre os blocos Markdown relevantes, mas linguagens embutidas indisponíveis degradam para estilo genérico de código.
- O índice é TTL, sem watcher do filesystem.
- Save em filesystems/plataformas sem replace-atômico por `rename` herda a semântica do `os.Rename` local.
- O smoke test de Liquid Cursor depende de pnex.

Essas limitações não alteram a gramática e o IntelliSense Prompt Lang documentados; representam funcionalidades editoriais deliberadamente fora do binário inicial simples.
