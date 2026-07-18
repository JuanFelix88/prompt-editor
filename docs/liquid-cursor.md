# Compatibilidade pnex Liquid Cursor

Contrato original: [`reference/liquid-cursor-tui-compatibility.md`](reference/liquid-cursor-tui-compatibility.md).

O app não implementa a animação líquida. pnex observa o cursor VT real. A invariável do renderer é:

```text
cursor real do terminal == caret lógico do documento
```

Todo frame termina, no mesmo batch, com:

```text
CSI <linha>;<coluna> H
CSI ? 25 h
```

O cálculo inclui gutter, scroll horizontal, tabs, graphemes, combining marks, emoji e largura CJK. Durante a confirmação de cancelamento, o cursor real muda para o fim da pergunta; ao fechá-la, volta ao caret. O cursor é reposicionado após input, completion, scroll, resize e análise assíncrona. A posição é limitada à grade válida.

Em terminais Unix, `Ctrl+Z` restaura tela/raw mode antes de `SIGTSTP`; depois de `SIGCONT`, o app reentra em raw/alternate screen e redesenha o caret.

Na saída:

```text
CSI ? 2004 l   # bracketed paste off
CSI ? 1049 l   # alternate screen off
CSI ? 25 h     # cursor visible
SGR 0
```

O raw mode é restaurado em seguida. A interface funciona sem `PNEX`; `PNEX=1` é apenas o sinal para smoke tests específicos.

## Smoke test obrigatório no pnex

Com **Cursor Animation → Liquid**:

- [ ] cursor visível desde o primeiro frame;
- [ ] digitação, setas, Home/End, Backspace/Delete mantêm o overlay no caret;
- [ ] popup não move o cursor real para a opção selecionada;
- [ ] CJK, combining marks, emoji e tabs não deslocam o overlay;
- [ ] scroll horizontal/vertical mantém o caret correto;
- [ ] resize, inclusive grades muito pequenas, recalcula gutter, popup e viewport sem coordenadas fictícias;
- [ ] Ctrl+S, Ctrl+X, Ctrl+C e interrupção devolvem cursor visível ao shell;
- [ ] redraw rápido não faz o cursor perseguir status/pontos de pintura;
- [ ] nenhuma escrita na célula inferior direita causa scroll.

Este teste precisa ser manual porque pnex/xterm.js e o canvas da animação não estão disponíveis em `go test`.
