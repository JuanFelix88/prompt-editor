# Uso

## Abrir arquivos

O modo padrão segue convenções comuns de CLIs de editores:

```sh
prompt-cli prompt.md
prompt-cli edit --workspace /repo --line 12 --column 4 prompts/review.prompt.md
prompt-cli --goto prompts/review.prompt.md:12:4
prompt-cli --wait prompt.md
```

- `--workspace`: raiz usada por referências `@`; o padrão é o diretório atual.
- `--line` e `--column`: posição inicial, baseada em 1.
- `--goto`: interpreta `FILE:LINE[:COLUMN]`, inclusive depois de um drive Windows.
- `--wait`: aceito por compatibilidade; o processo sempre espera o editor fechar.
- Arquivo omitido: `prompt.md`.
- Arquivo inexistente: cria um buffer e só materializa no primeiro save.

O editor restringe escrita a `.md` e `.prompt`. A semântica Prompt Lang completa é ativada apenas nos nomes definidos em [language.md](language.md).

## Teclas

| Tecla | Ação |
| --- | --- |
| `Ctrl+S` | grava atomicamente e confirma, fechando com código 0 |
| `Ctrl+X` / `Ctrl+C` | cancela e fecha sem gravar, código 130 |
| `Ctrl+Space` | abre/atualiza completion |
| `Tab` ou `Enter` | aceita completion; sem popup, insere tab/newline |
| `Esc` | fecha completion |
| `↑` / `↓` | move caret ou seleção do popup |
| `←` / `→` | move por grapheme |
| `Home` / `End` | início/fim da linha |
| `Backspace` / `Delete` | apaga grapheme/junção de linha |
| `Page Up` / `Page Down` | move uma viewport |

Completion também abre ao digitar `@`, `[`, ou `/` em um contexto válido. O popup nunca substitui o cursor real: o cursor permanece no caret do documento.

## Saída e falhas

Falha de save aparece na barra de status e mantém o editor aberto. Saídas normais, cancelamento, panic e interrupção restauram raw mode, atributos, alternate screen e visibilidade do cursor. Caracteres C0 recebidos por paste são removidos para impedir injeção de sequências de terminal.
