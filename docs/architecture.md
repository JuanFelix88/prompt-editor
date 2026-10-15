# Arquitetura

## Um executável, um núcleo

```text
cmd/prompt-cli
  ├─ editor TUI (modo padrão)
  ├─ adaptador LSP stdio (`lsp`)
  └─ internal/lang
       ├─ snapshots de documentos abertos
       ├─ scanners Prompt Lang
       ├─ índice de metadados do workspace
       └─ completion / hover / definition / semantic tokens
```

A TUI chama `internal/lang` diretamente; não inicia LSP, subprocesso ou runtime secundário. O subcomando `lsp` adapta o mesmo núcleo a JSON-RPC para clientes externos. Assim, editor e integrações usam as mesmas regras e o build produz um único binário.

## Estado e I/O

- O arquivo é lido uma vez ao abrir.
- Texto aberto fica em memória e sempre prevalece sobre o disco.
- Edições atualizam o snapshot completo; análise semântica da TUI usa debounce de 75 ms.
- O índice guarda somente caminho, basename, tipo e profundidade.
- A varredura BFS é lazy, reutilizada por 2 segundos, para em 50.000 entradas e ignora diretórios gerados/VCS especificados.
- Completion retorna no máximo 500 itens.
- Hover, definition e semantic tokens LSP validam referências com `stat` atual, sem confiar no índice possivelmente stale.
- Save usa arquivo temporário no mesmo diretório, `Sync` e `Rename`.
- `F12`/`Ctrl+O` empilha instâncias internas do editor, mantendo um único loop de terminal e leitor de stdin. Save/close e cancelamento da instância filha restauram a anterior; nenhum subprocesso é iniciado. Timers de análise pendentes são descartados ao trocar a instância ativa.

## Buffer e coordenadas

O buffer inicial usa linhas de runes, uma escolha simples para prompts e Markdown. Movimentos e deleções respeitam grapheme clusters. Existem três sistemas de coordenadas explícitos:

1. runes/graphemes para edição;
2. células de terminal para layout, tabs, CJK, combining marks e emoji;
3. unidades UTF-16 para a fronteira LSP.

O renderer usa `uniseg` e `go-runewidth` tanto para desenhar quanto para posicionar o caret. Linhas não quebram: arquivos largos usam scroll horizontal.

## Renderização

Cada frame é montado integralmente em memória e escrito uma vez:

1. ocultação breve do cursor dentro do batch;
2. pintura da viewport, status e popup;
3. `CUP` absoluto na célula lógica do caret;
4. `DECTCEM show` como operação final;
5. um único write.

A TUI usa alternate screen, suportada pelo pnex, e não habilita mouse. A última célula da última linha não é escrita para evitar wrap/scroll acidental.

## Highlighting

A camada estrutural é um scanner block-oriented puro Go para ATX/Setext, thematic breaks, código indentado/fenced, frontmatter YAML/TOML, HTML, link definitions, listas, blockquotes e tabelas. Estados de fence/frontmatter são cacheados por versão; marcadores Markdown não vazam para conteúdo fenced. A camada semântica implementa os constructs Prompt Lang, prevalece e divide spans amplos ao redor de referências.

Não há Tree-sitter/CGO nem parsers de linguagens embutidas registrados. Conforme a regra de fallback da referência, fences com linguagem indisponível degradam para estilo de código/texto sem quebrar Markdown. Isso mantém o binário pequeno, mas não oferece cores específicas de Rust/Go/etc. dentro do fence.

## Diretórios

- `cmd/prompt-cli`: entrypoint.
- `internal/app`: CLI e convenções de abertura.
- `internal/editor`: buffer, input raw, viewport e ANSI renderer.
- `internal/lang`: serviço de linguagem compartilhado.
- `internal/lsp`: framing e lifecycle LSP.
- `docs/reference`: contratos originais que governam a implementação.
