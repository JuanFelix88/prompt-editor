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
- Arquivo omitido: abre `[No Name]`; nada é gravado. `Ctrl+S` fecha e escreve somente o Markdown no stdout; a TUI usa stderr para não contaminar pipelines.
- Arquivo inexistente informado: cria um buffer e só materializa no primeiro save.

O editor restringe escrita a `.md` e `.prompt`. A semântica Prompt Lang completa é ativada apenas nos nomes definidos em [language.md](language.md).

## Teclas

| Tecla | Ação |
| --- | --- |
| `Ctrl+S` | com arquivo, grava atomicamente; sem arquivo, envia texto ao stdout; fecha com código 0 |
| `Ctrl+X` / `Ctrl+C` | abre confirmação; `y` descarta e fecha com código 130, `n`/`Esc`/`Enter` continua |
| `Ctrl+Space` | abre/atualiza completion |
| `Tab` ou `Enter` | aceita completion; sem popup, insere tab/newline |
| `Esc` | fecha completion |
| `↑` / `↓` | move caret ou seleção do popup |
| `←` / `→` | move por grapheme |
| `Home` / `End` | início/fim da linha |
| `Backspace` / `Delete` | apaga grapheme/junção de linha |
| `Ctrl+Backspace` / `Alt+Backspace` / `Ctrl+W` | apaga a palavra anterior |
| `Ctrl+Enter` | insere uma linha abaixo, preservando a linha atual e sua indentação |
| `Page Up` / `Page Down` | move uma viewport |

Completion também abre ao digitar `@`, `[`, ou `/` em um contexto válido. O popup nunca substitui o cursor real: o cursor permanece no caret do documento.

## Saída e falhas

No modo sem arquivo, somente `Ctrl+S` produz stdout; cancelamento confirmado não produz texto. Falha de save aparece na barra de status e mantém o editor aberto. Saídas normais, cancelamento, panic e interrupção restauram raw mode, atributos, alternate screen e visibilidade do cursor. Caracteres C0 recebidos por paste são removidos para impedir injeção de sequências de terminal.
