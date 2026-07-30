# Markdown Prompt Lang

Os contratos normativos completos estão em [`docs/reference/prompt-lang-concepts`](reference/prompt-lang-concepts/). Este resumo descreve o comportamento exposto pelo binário.

## Ativação

Recursos contextuais são ativados em qualquer arquivo `*.md` ou `*.prompt`, sem distinção pelo nome.

## Referências de workspace

```md
Read @src/main.go
Use @docs/
```

Forma efetiva: `(^|non-word)@[A-Za-z0-9._~+/-]+`. O alvo é relativo à raiz do workspace. Caminhos que escapem da raiz são rejeitados; arquivos e pastas são válidos. Completion é case-insensitive, enquanto existência segue o filesystem.

## Referências de headings

```md
# Review
## Style
Use [Review > Style]
```

Somente headings ATX de 1 a 6 níveis entram na hierarquia; headings dentro de fences são ignorados. Matching final é exato e case-sensitive. `[x]`, imagens e links inline imediatos são excluídos. Duplicatas navegam para a primeira ocorrência.

## Prompt items

Uma linha `- item` recebe token semântico de prompt item. `*`, `+` e listas ordenadas mantêm apenas estilo Markdown estrutural. Referências dentro de um item ou heading têm precedência e dividem o span amplo.

## IntelliSense

- `@prefix`: arquivos e pastas do workspace, pastas com `/` final.
- `[prefix`: caminhos de headings do documento atual; adiciona `]` quando necessário.
- Contexto de heading tem precedência sobre arquivo.
- Resultados são ranqueados por prefixo, basename/leaf e substring, com ordem determinística.

O serviço também oferece hover, go-to-definition e tokens diferentes para referências existentes/ausentes. Ausências **não** são diagnostics LSP.

## Limites mantidos por compatibilidade

Paths com espaços, Unicode, `:`, `#` ou backslash não formam um único token. Heading references são lineares e lexicais, não Markdown-AST-aware. Referências podem continuar ativas em code fences; apenas a descoberta de headings é fence-aware. Setext headings não são alvos.
