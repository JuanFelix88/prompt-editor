# Building TUIs Compatible with pnex's Liquid Cursor

## Purpose

This document is an implementation contract for terminal user interface (TUI) applications that run inside **pnex** and should work correctly with its liquid cursor effect.

The liquid cursor is implemented by pnex itself. A TUI does **not** need a pnex SDK, a custom protocol, or its own animation code. It only needs to maintain the terminal's real VT cursor correctly.

> **Core rule:** keep the real terminal cursor visible and move it to the TUI's current logical focus after every render.

pnex launches programs through a PTY with:

```text
TERM=xterm-256color
PNEX=1
```

`PNEX=1` may be used for optional pnex-specific behavior, but a well-behaved TUI can satisfy this contract in every terminal.

## How the Effect Works

pnex uses xterm.js as its terminal emulator. Its liquid cursor reads the active terminal buffer's cursor position and draws an animated, full-cell canvas overlay at that position.

The effect:

- follows standard VT/xterm cursor movement;
- animates between terminal cells when the cursor position changes;
- works in the normal and alternate screen buffers;
- uses the pnex theme's cursor color;
- blinks after input becomes idle;
- snaps to the correct cell after scrolling, resizing, or changing buffers;
- is hidden when the application sends the standard cursor-hide sequence;
- is shown when the application sends the standard cursor-show sequence;
- appears as an outline while the terminal window is unfocused.

The effect observes the cursor in xterm.js's **active buffer**, not application widgets or CUP commands in isolation. In the normal buffer, pnex also accounts for the cursor, scrollback base, and viewport offsets. A TUI should maintain its own logical cursor normally; it should not move that cursor merely because the user views older scrollback. pnex cannot infer which field, button, menu item, or list row is selected, so the TUI communicates that information by placing the real terminal cursor at the appropriate cell.

## Compatibility Requirements

### 1. Keep DECTCEM enabled

The most important requirement is that the terminal cursor remains visible while the interactive screen is displayed.

| Operation | Escape sequence | Meaning |
| --- | --- | --- |
| Show cursor | `CSI ? 25 h` (`\x1b[?25h`) | Enables DECTCEM and the liquid cursor |
| Hide cursor | `CSI ? 25 l` (`\x1b[?25l`) | Disables DECTCEM and hides the liquid cursor |

Many TUI frameworks hide the cursor by default because they draw their own cursor or selection marker. That default is incompatible with the liquid effect. Override it and show the terminal cursor.

"Keep the cursor visible" means that it must be visible at every committed render boundary. A renderer may hide it briefly inside one batched redraw to prevent flicker, provided that the same batch ends by placing it at logical focus and emitting `CSI ? 25 h` before flushing. Use longer cursor hiding only when the interface intentionally has no focus target, such as a short loading transition.

### 2. Place the cursor at the logical focus

After drawing a frame, move the terminal cursor to the cell that represents the current interaction target:

- text input: the insertion cell;
- menu: the selected row, preferably in a dedicated cursor gutter;
- list or table: the selected row or focused cell;
- button group: the focused button's leading cell;
- editor: the document caret;
- modal: the modal's focused control.

For example, this sequence places the cursor at row 8, column 14 and makes it visible:

```text
ESC [ 8 ; 14 H ESC [ ? 25 h
```

As bytes or a string literal:

```text
\x1b[8;14H\x1b[?25h
```

VT row and column parameters are **1-based**. Application layout code is often 0-based, so convert with `row + 1` and `column + 1`.

### 3. Make cursor placement the final operation in each frame

A renderer commonly moves the cursor many times while painting widgets. pnex may observe those positions. Build the complete frame in memory and write it in one batch when possible, with the logical cursor placement at the end:

```text
[enter/update screen]
[draw all widgets]
CSI <focus-row> ; <focus-column> H
CSI ? 25 h
```

This prevents the liquid cursor from chasing temporary paint positions or ending at the last cell used to draw a status bar.

If the framework flushes several times per frame, ensure the final flush restores the logical cursor position.

### 4. Use the real cursor as the source of truth

Do not leave the real cursor in an unrelated location while drawing a fake cursor using:

- inverse-video text;
- a colored block character;
- a custom glyph;
- a framework-only caret;
- a selected-row background.

Those decorations may still be used as additional focus styling, but the real terminal cursor must occupy the same semantic target. Otherwise the liquid effect will animate somewhere else.

### 5. Reposition after every state or layout change

Restore the cursor after:

- keyboard navigation;
- text insertion or deletion;
- mouse selection;
- opening or closing a modal;
- changing tabs or panes;
- asynchronous data updates;
- terminal resize;
- full or partial redraw;
- entering or leaving the alternate screen.

Do not assume the previous cursor position survived a clear, scroll, buffer switch, or framework render pass.

### 6. Track display-cell widths correctly

Cursor coordinates refer to terminal **cells**, not bytes, Unicode code points, grapheme count, or pixels.

Account for:

- wide CJK characters, usually two cells;
- combining marks, usually zero additional cells;
- emoji and grapheme clusters;
- tabs;
- wrapping at the right edge;
- clipped text.

Use the same `wcwidth`/display-width implementation as the renderer. A mismatch makes both the native terminal cursor and the liquid cursor appear one or more cells away from the visual caret.

### 7. Handle resize and cleanup

On `SIGWINCH` or the framework's resize event:

1. read the new terminal dimensions;
2. recompute layout;
3. redraw;
4. clamp the logical focus to a valid cell;
5. place and show the cursor again.

On normal exit, error, panic, signal, or cancellation, restore terminal state. At minimum:

```text
CSI ? 1049 l # leave alternate screen, if it was entered
CSI ? 25 h   # show cursor after any saved state was restored
SGR 0        # reset attributes
```

Install cleanup handlers where the language/runtime permits it. A TUI that exits while DECTCEM is disabled can leave the shell without a visible cursor.

## Recommended Rendering Pattern

The following pseudocode is framework-independent:

```text
start:
    enable raw input mode
    enter alternate screen if desired
    show terminal cursor

render(state):
    frame = new output buffer
    frame += draw_interface(state)

    target = logical_focus_cell(state)
    target.row = clamp(target.row, 0, terminal_rows - 1)
    target.col = clamp(target.col, 0, terminal_cols - 1)

    frame += CUP(target.row + 1, target.col + 1)
    frame += DECTCEM_SHOW
    write_and_flush_once(frame)

on_input(event):
    update_state(event)
    render(state)

on_resize:
    recompute_layout()
    render(state)

shutdown:
    show terminal cursor
    leave alternate screen if active
    reset terminal attributes
    disable raw input mode
```

A minimal sequence for entering a full-screen interface is:

```text
\x1b[?1049h          enter alternate screen
\x1b[2J\x1b[H       clear and home
...rendered frame...
\x1b[8;14H\x1b[?25h place and show the logical cursor
```

A minimal exit sequence is:

```text
\x1b[?1049l\x1b[?25h\x1b[0m
```

## Framework Integration Guidance

When using a TUI framework, inspect its initialization, frame-finalization, and shutdown behavior.

Look for APIs or commands named similarly to:

- `show_cursor` / `ShowCursor`;
- `hide_cursor` / `HideCursor`;
- `set_cursor_position` / `set_cursor`;
- `cursor_position`;
- `enable_raw_mode`;
- `enter_alternate_screen`.

Apply this order for every frame:

1. let the framework draw normally;
2. tell the framework the desired cursor position, if it supports that;
3. show the cursor;
4. flush.

If the framework always emits `\x1b[?25l` after rendering and provides no override, it is not directly compatible. Use a renderer hook, output middleware, or a small final ANSI write to emit `CUP` followed by `\x1b[?25h`. That fallback must run after the framework's last write at the render boundary; later framework output could hide or move the cursor again. Prefer an official cursor API over filtering arbitrary framework output.

For interfaces that normally do not use a caret, reserve a one-cell focus gutter. Example:

```text
  Settings
  ┌──────────────────────────────┐
  │ [ General                  ] │
  │ [ Appearance               ] │
  │ [ Key bindings             ] │
  └──────────────────────────────┘
    ^ place the real cursor in a stable cell beside the selected row
```

A stable gutter prevents the full-cell liquid cursor from covering important label text.

## Escape Sequences Relevant to pnex

pnex follows xterm.js's parsed terminal state. Standard cursor commands therefore work as expected.

| Sequence | Name | Recommended use |
| --- | --- | --- |
| `CSI r ; c H` | CUP | Preferred absolute placement after a frame |
| `CSI r ; c f` | HVP | Equivalent absolute placement |
| `CSI n A/B/C/D` | CUU/CUD/CUF/CUB | Relative movement; use cautiously during rendering |
| `CSI ? 25 h` | DECTCEM show | Required for liquid cursor visibility |
| `CSI ? 25 l` | DECTCEM hide | Hides the liquid cursor |
| `CSI s` / `CSI u` | Save/restore cursor | Safe only when the restored position is intentional |
| `ESC 7` / `ESC 8` | DEC save/restore cursor | Same caution as above |
| `CSI ? 1049 h/l` | Alternate screen | Supported; reposition and show after either transition |
| `ESC c` | Full reset | pnex restores visibility, but this also resets terminal state; do not use merely to show the cursor |
| `CSI ! p` | Soft reset | pnex restores visibility; avoid using it merely as a cursor command |

Even though pnex treats both reset sequences as restoring visibility, explicitly emit `CSI ? 25 h` after a reset for portability across terminals.

Cursor-shape sequences such as DECSCUSR may change a normal terminal cursor, but pnex's liquid cursor is intentionally a full-cell animated shape. Do not depend on beam, underline, or blink-style requests to configure the liquid overlay.

## Avoid These Patterns

### Hiding the cursor for the application's entire lifetime

```text
start -> CSI ? 25 l -> render fake selections -> exit
```

This completely disables the liquid cursor.

### Ending a frame at a paint location

```text
render focused input
render footer at the bottom
flush while cursor remains at footer end
```

The effect follows the footer instead of the focused input. Append an absolute cursor placement before flushing.

### Moving through many visible intermediate positions

```text
write chunk -> flush -> move -> write chunk -> flush -> move -> flush
```

This can create unintended cursor travel. Batch the frame and place the cursor last.

### Assuming one character equals one cell

Using string length as the cursor column fails with wide or combining characters. Use terminal display width.

### Using the bottom-right cell carelessly

Writing to the final column of the final row can trigger wrapping or scrolling, changing the resulting cursor position. Clip the draw operation or disable auto-wrap only if the renderer manages that mode correctly.

## Compatibility Test Plan

A TUI is compatible when all of the following pass inside pnex with **Cursor Animation → Liquid** enabled.

### Basic behavior

- [ ] The cursor is visible after the application starts.
- [ ] Arrow-key navigation moves the liquid cursor to the selected item.
- [ ] Tab and Shift+Tab move it to the newly focused control.
- [ ] Typing keeps it at the text insertion point.
- [ ] Backspace, Delete, Home, End, and horizontal scrolling preserve correct placement.
- [ ] No duplicate fake cursor appears in another cell.

### Rendering behavior

- [ ] Full redraws do not leave the cursor at the bottom or at a status bar.
- [ ] Partial redraws restore the logical cursor before flush.
- [ ] Fast key repeats produce intentional movement rather than jumps through paint positions.
- [ ] Wide characters and emoji do not offset the cursor.
- [ ] The target cell remains valid at the right and bottom edges.

### Lifecycle behavior

- [ ] Resizing redraws and repositions the cursor.
- [ ] Entering and leaving a modal preserves visibility.
- [ ] Alternate-screen entry and exit preserve visibility.
- [ ] Suspending/resuming the application restores the cursor.
- [ ] Normal exit returns a visible shell cursor.
- [ ] Error and interrupt exits also attempt terminal cleanup.

### pnex-specific smoke test

Temporarily print or log the inherited environment and verify that `PNEX=1` is available. Use `PNEX=1`, not `TERM=xterm-256color`, to detect pnex-specific behavior: `TERM` only describes the terminal capability baseline and does not prove that the liquid overlay exists. Do not require `PNEX` unless the application intentionally has a pnex-only mode.

## Agent Implementation Checklist

When assigning another coding agent to build a compatible TUI, include these acceptance criteria:

1. **Do not implement the liquid animation in the TUI.** pnex owns it.
2. Keep DECTCEM enabled (`\x1b[?25h`) whenever an interactive focus target exists.
3. Maintain one logical focus cell in application state.
4. End every rendered frame with absolute CUP placement at that cell and show the cursor.
5. Batch frame output into one flush whenever practical.
6. Use display-cell width calculations for Unicode text.
7. Recompute and restore cursor placement after resize, redraw, and screen-buffer changes.
8. Restore cursor visibility and terminal modes on every supported exit path.
9. Test the compiled application inside pnex, not only in a conventional terminal.
10. Treat any framework-level cursor hiding as a compatibility bug unless it is brief and intentional.

## Quick Reference

```text
Compatibility signal:  PNEX=1
Terminal profile:      TERM=xterm-256color
Show liquid cursor:    \x1b[?25h
Hide liquid cursor:    \x1b[?25l
Place at row/column:   \x1b[<row>;<column>H   (1-based)
Best frame ending:     CUP + DECTCEM show + flush
Primary invariant:     real terminal cursor == current logical focus
```
