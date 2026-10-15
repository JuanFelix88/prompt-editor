# Uso

## Abrir arquivos

O modo padrão segue convenções comuns de CLIs de editores:

```sh
prompt-cli prompt.md
prompt-cli edit --workspace /repo --line 12 --column 4 prompts/review.prompt.md
prompt-cli --goto prompts/review.prompt.md:12:4
prompt-cli --wait prompt.md
```

- `--workspace`: base de referências `@` somente no modo sem arquivo; o padrão é o diretório atual. Com arquivo associado, a base é sempre a pasta desse arquivo, mesmo antes do primeiro save.
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
| `Ctrl+Shift+S` | com arquivo, grava atomicamente sem fechar; sem arquivo, mantém o buffer aberto e exibe um aviso |
| `Ctrl+K` | alterna Keep open (OFF inicialmente, apenas nesta sessão); ON impede fechar ao salvar, com estado e atalho no rodapé |
| `Ctrl+X` / `Ctrl+C` | abre confirmação; `y` descarta e fecha com código 130, `n`/`Esc`/`Enter` continua |
| `Ctrl+Space` | abre/atualiza completion |
| `Ctrl+G` | abre “Ir para linha”; informe `linha` ou `linha:coluna` e pressione Enter |
| `Tab` ou `Enter` | aceita completion; sem popup, insere tab/newline |
| `Esc` | fecha completion ou “Ir para linha” |
| `↑` / `↓` | move caret ou seleção do popup |
| `←` / `→` | move por grapheme |
| `Ctrl+←` / `Ctrl+→` ou `Alt+←` / `Alt+→` | move por palavra |
| `Alt+↑` / `Alt+↓` | move a linha atual para cima/baixo |
| `Home` / `End` | início/fim da linha |
| `Ctrl+Home` / `Ctrl+End` | início/fim do documento |
| `Backspace` / `Delete` | apaga grapheme/junção de linha |
| `Ctrl+Backspace` / `Alt+Backspace` / `Ctrl+W` | apaga a palavra anterior |
| `Ctrl+Delete` | apaga o restante da palavra atual/próxima |
| `Ctrl+Enter` / `Shift+Enter` | insere uma linha com a mesma indentação abaixo/acima, sem alterar a linha atual |
| `Page Up` / `Page Down` | move uma viewport |

Completion também abre ao digitar `@`, `[`, ou `/` em um contexto válido. O popup nunca substitui o cursor real: o cursor permanece no caret do documento.

Com Keep open ON, `Ctrl+S` grava arquivos sem fechar, inclusive quando o terminal não distingue `Ctrl+Shift+S`. Sem arquivo, o conteúdo permanece na memória, sem escrita em disco/stdout: desative com `Ctrl+K` e confirme com `Ctrl+S`.

## Saída e falhas

No modo sem arquivo, somente `Ctrl+S` produz stdout; cancelamento confirmado não produz texto. Falha de save aparece na barra de status e mantém o editor aberto. Saídas normais, cancelamento, panic e interrupção restauram raw mode, atributos, alternate screen e visibilidade do cursor. Caracteres C0 recebidos por paste são removidos para impedir injeção de sequências de terminal.
