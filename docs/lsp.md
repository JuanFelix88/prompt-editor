# LSP embutido

O mesmo binário funciona como servidor LSP por stdio:

```sh
prompt-cli lsp --workspace /caminho/do/repo
```

Não existe segundo binário. A TUI não serializa chamadas internas; ela usa o núcleo Go diretamente. O modo LSP existe para integração com VS Code, Zed e outros clientes compatíveis.

## Capacidades

- full-document synchronization (`TextDocumentSyncKind.Full`);
- completion com triggers `@`, `/`, `[` e `resolveProvider: false`;
- hover;
- definition;
- full semantic tokens;
- sem diagnostics, formatting, rename, code actions ou incremental semantic tokens.

A legenda semântica fixa, em ordem de wire, é:

```text
promptFile
promptFileMissing
promptHeadingReference
promptHeadingReferenceMissing
promptHeadingMarker
promptHeading1
promptHeading2
promptHeading3
promptHeading4
promptHeading5
promptHeading6
promptItem
promptStarItem
```

## Lifecycle e transporte

Mensagens usam `Content-Length` com corpos JSON-RPC UTF-8. O leitor suporta frames partidos e concatenados por meio de leitura exata e rejeita headers acima de 64 KiB, linhas de header acima de 8 KiB e corpos acima de 16 MiB antes de alocar. Logs e erros vão somente para stderr; stdout contém exclusivamente frames.

- argumento `--workspace` prevalece sobre `rootUri`/workspace folders;
- sem argumento, `rootUri` ou o primeiro workspace folder substitui o diretório inicial;
- snapshots de `didOpen`/`didChange` prevalecem sobre o arquivo;
- `didChange` usa o primeiro change como novo texto completo;
- `didClose` remove o snapshot;
- request desconhecido retorna `-32601`;
- falha/panic de handler retorna `-32603`;
- `shutdown` responde `null`; somente `exit` encerra.

Posições e ranges do protocolo usam unidades UTF-16. O serviço aceita apenas URIs `file:` locais.

## Exemplo de configuração conceitual

Configure o cliente para executar:

```json
{
  "command": "/absolute/path/prompt-cli",
  "args": ["lsp", "--workspace", "/absolute/path/workspace"]
}
```

Associe o servidor a `*.md` e `*.prompt`; ambos recebem todos os recursos de Prompt Lang.
