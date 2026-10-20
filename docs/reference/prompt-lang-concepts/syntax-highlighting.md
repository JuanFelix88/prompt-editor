# Syntax Highlighting for Prompt-Flavored Markdown

## Purpose and model

The language is ordinary Markdown with a small prompt-oriented semantic layer. Its highlighting is best understood as two cooperating passes:

1. **Structural highlighting** parses Markdown blocks and assigns stable, theme-oriented scopes to headings, paragraphs, lists, tables, links, code fences, and other markup.
2. **Contextual highlighting** scans prompt documents for constructs whose meaning depends on the workspace or the current document: `@file` references, `[Heading]` references, ATX heading levels, `-`/`*` prompt items, and task-marker state.

The second pass is not a replacement for the Markdown parser. It overlays narrower semantic ranges on broad structural ranges. A faithful implementation must give those narrower ranges precedence and must avoid emitting overlapping semantic ranges.

The semantic layer applies to every file ending in `.md` or `.prompt`, case-insensitively. Contextual file references, heading references, and prompt-item coloring therefore work in generic Markdown too. Custom ATX heading decoration is computed independently of that gate.

## Structural Markdown layer

The bundled parser is the block half of `tree-sitter-markdown` at revision `c3570720f7f7bbad22fe96603f106276618e0cf5`. It follows CommonMark approximately and has the default GitHub-flavored extensions enabled, including task lists, strikethrough, pipe tables, YAML front matter, and TOML front matter. Like most editor-oriented Markdown parsers, it prioritizes useful incremental structure over perfect conformance.

The structural query assigns the following conceptual scopes:

| Syntax or node | Conceptual scope |
| --- | --- |
| Paragraph, indented code block, pipe table | ordinary text |
| Setext heading and thematic break | markup title |
| ATX `#` marker, levels 1–6 | comment-like marker |
| ATX heading text, levels 1 and 4 | markup title |
| ATX heading text, levels 2 and 5 | string-like text in the structural pass |
| ATX heading text, levels 3 and 6 | literal markup text |
| Setext heading paragraph | markup title |
| `+`, `-`, `*`, ordered-list dot, and ordered-list parenthesis markers | list-marker punctuation |
| Block quote marker and block continuation | markup punctuation |
| Table pipes and delimiter-row hyphens | markup punctuation |
| Code-fence delimiter and info string | embedded-markup punctuation |
| Link reference definition | link text |
| Link destination | link URI |

The ATX palette intentionally repeats every three levels: 1/4, 2/5, and 3/6. In the contextual pass, levels 2/5 use a function-like role rather than the structural pass's string-like role. Hosts that apply semantic highlighting over syntax highlighting will therefore show the contextual palette. This difference is intentional in the current implementation and should be resolved in favor of the contextual role when reproducing the complete behavior.

Setext headings receive only structural title coloring. They do not participate in the heading hierarchy used by prompt references and do not receive level-specific contextual tokens.

### Markdown inline behavior

The upstream Markdown project is a split parser: block parsing identifies `inline` ranges, and a second `markdown_inline` parser is normally needed for detailed emphasis, strong emphasis, code spans, images, and inline links. The language-specific injection query in this repository does **not** request that second inline parse. Consequently, the guaranteed behavior here is block-oriented: paragraphs and heading contents receive broad scopes, while detailed inline styling is available only if the target host independently supplies the split inline pass.

A replica should not assume that the block parser alone identifies all inline Markdown. If full Markdown fidelity is desired, run the inline grammar over each block parser `inline` range, excluding child ranges as required by the split-parser design. That enhancement should coexist with the prompt semantic overlay rather than changing prompt reference recognition.

## Embedded constructs

The structural injection rules delegate these ranges to embedded language parsers:

- A fenced code block with an info-string language injects its `code_fence_content` using the parsed language name.
- An HTML block injects its whole content as HTML.
- `---` metadata injects as YAML.
- `+++` metadata injects as TOML.

Unknown or unavailable fence language names should degrade to code/plain text without breaking the surrounding Markdown parse. Fences without a language do not request an embedded parser.

The prompt semantic scanner is line-based and does not use the Markdown syntax tree to suppress ranges inside injected content. Therefore `@path` and `[Heading]`-shaped text inside code fences, HTML, metadata, inline code, or comments can still receive prompt semantic coloring. This is a notable limitation, not an instruction to the embedded parser. A byte-for-byte behavioral replica should preserve it; a more syntax-aware port may choose to suppress such tokens, but that is a deliberate compatibility change.

## Contextual prompt constructs

### Workspace file and folder references

A file reference has this effective lexical form:

```text
(^|non-word) @ [A-Za-z0-9._~+/-]+
```

Only the `@` and path are colored; the non-word boundary is not. Practical examples include:

```md
Read @src/main.rs
Compare (@docs/guide.md)
Use @assets/
```

Recognition details:

- The reference may begin at the start of a line or after a character that JavaScript-style `\w` does not recognize.
- It cannot immediately follow an ASCII letter, digit, or underscore.
- Allowed path characters are ASCII letters and digits plus `.`, `_`, `~`, `+`, `/`, and `-`.
- Spaces, backslashes, colons, query strings, fragments, and most other punctuation terminate the token.
- A bare `@` is not highlighted.
- Both files and directories count as existing targets.

The captured path is normalized by removing the `@`, converting backslashes if supplied by another entry path, and removing leading slashes. It is then resolved relative to the directory of the document being edited. In particular, `@/docs/a.md` is treated as `@docs/a.md` relative to that document, not as an operating-system absolute path. Resolution rejects NULs and absolute or drive-qualified forms remaining after normalization. Dot segments are allowed: `..` can reach outside the document directory and workspace. Whether case differences resolve successfully follows the filesystem.

Existing references use a link/URI-like blue role (the supplied fallback color is `#4ea1ff`). Missing references use an error role with a red fallback (`#ff5f5f`) and an underline. Existence is live filesystem state rather than a grammar property, so a portable implementation needs a document-relative resolver and a refresh strategy when files are created, removed, or renamed.

### Heading references

A heading reference is a single bracketed range on one line:

```md
[Topic]
[Parent > Child]
```

The whole range, including brackets, receives one semantic scope. Existing references use a method/function-like link role. Missing references use an underlined error role.

The scanner initially accepts `[` followed by one or more characters other than `]` or newline, followed by `]`. It trims the interior before resolving it, then excludes:

- empty or whitespace-only interiors;
- `[x]` and `[X]`, reserved for task-list checkboxes;
- a bracket immediately preceded by `!`, to avoid Markdown images;
- a bracket immediately followed by `(`, to avoid ordinary inline links.

These exclusions are purely lexical. Escapes and Markdown parse context are not consulted. As a result:

- `[label] (url)` is treated as a heading reference because the `(` is not immediate;
- shortcut and reference-style Markdown links can be mistaken for heading references;
- brackets inside code, HTML, metadata, or comments can be references;
- nested or malformed bracket sequences can produce surprising captures;
- only the first closing bracket ends the candidate.

If an `@file` token overlaps a bracket candidate—for example, `[@src/a.md]`—the file reference wins and the heading reference is omitted. This precedence prevents overlapping semantic tokens.

### Heading hierarchy and resolution

Only non-empty ATX headings are indexed. A valid indexed heading:

- begins with zero to three spaces;
- has one to six `#` characters;
- has whitespace or end-of-line after the marker;
- contains non-whitespace heading text.

Optional closing `#` characters are removed only when preceded by whitespace. Remaining text is trimmed for identity. Markdown markup inside the heading is not rendered or normalized: a heading named `## **Style**` is indexed as `**Style**`.

The hierarchy is constructed in document order with a six-level stack. On each heading, deeper stack entries are discarded and the current level is replaced. Empty intermediate levels are filtered out, so a level-3 heading directly below a level-1 heading still has the path `Level 1 > Level 3`. A document beginning with a level-2 heading gives that heading a one-component path.

Reference text is normalized by splitting on every `>`, trimming each component, removing empty components, and joining with the canonical separator ` > `. Resolution is an exact, case-sensitive comparison against the indexed path. There is no slugification, fuzzy matching, or formatting normalization.

Consequences include:

- `[Child]` resolves only when the target's full hierarchy is exactly `Child`; a nested child normally requires `[Parent > Child]`.
- Repeated headings with the same path are all matches; existence coloring only needs one. Navigation-like consumers conventionally choose the first.
- A literal `>` in a heading is indistinguishable from a hierarchy separator.
- A single heading named `Parent > Child` can collide with an actual parent/child path.
- Setext headings are never targets.

The heading scanner suppresses headings inside fenced blocks using a lightweight fence state. A line indented by at most three spaces and beginning with at least three backticks or tildes opens a fence; a marker of the same character and at least the opening length closes it. This scanner is simpler than the Markdown grammar and may disagree with it on malformed fences or closing lines with trailing content.

### ATX heading semantics

The contextual pass separates the marker from heading text:

- all `#` markers receive a comment-like role;
- levels 1/4 receive a title role;
- levels 2/5 receive a function/built-in-function role;
- levels 3/6 receive a literal-text role.

The heading text range excludes trailing spaces and a valid closing `#` sequence. Empty ATX headings do not receive contextual heading tokens, although the structural parser may still color their marker.

A reference occurring inside heading text keeps its reference-specific color. The surrounding heading token is split into segments before and after the reference so that no semantic ranges overlap.

### Prompt items

Any prompt-document line matching `^(\s*)[-*]\s+` is treated as a prompt item. Coloring begins at the list marker and continues to the end of the physical line. Hyphen items emit `promptItem`; star items emit `promptStarItem`. The terminal renderer maps these categories to ANSI standard red (`SGR 31`) and blue (`SGR 34`) respectively, so their exact colors come from the active terminal theme rather than fixed RGB values.

This behavior is intentionally narrower than Markdown list syntax:

- only `-` and `*` items receive whole-line semantic coloring;
- `+` and ordered lists receive only structural marker punctuation;
- nested prompt items work because arbitrary leading whitespace is accepted;
- a marker without following whitespace is not an item;
- continuation lines are not colored;
- task items are colored as part of the line, while `[x]` and `[X]` are excluded from heading-reference recognition;
- task markers are recognized positionally, only as the first bracketed value after a list marker;
- the task-marker span is cut out of the prompt-item span rather than overlapping it;

### Task markers

A task marker opens a list item as `- [x] `, `* [*] `, or `+ [😀] `, where the item marker is `-`, `*`, or `+`, one or more spaces or a tab, and a single-character bracketed marker. Recognition is positional: the brackets must follow the list marker at the start of the line, so `see [x]` in running text is ordinary content and never a task marker.

Only the inner character carries the token; the brackets keep the color of the line they belong to. `x` in any case emits `promptTaskDone`, and any other non-space marker emits `promptTaskMarker`. The terminal renderer maps these to ANSI standard gray (`SGR 90`) and yellow (`SGR 33`), so their exact colors come from the active terminal theme rather than fixed RGB values.

An empty `[ ]` is an unchecked task and receives no token; neither does a bracketed value with more than one character or with no trailing whitespace. Ordered-list items are not recognized.

The task-marker span is excluded from the enclosing prompt-item span, so a marked item emits two prompt-item ranges around one marker range instead of overlapping ranges. A task marker is never resolved as a heading reference. That holds for a custom marker such as `[*]` as well: without the exclusion it would be painted as a missing reference, since nothing else is likely to be named `*`.

The structural pass colors the same span with the same roles, so a document rendered without contextual tokens keeps the task marker readable, and a task marker inside a fence or metadata block stays generic code or metadata styling.

File or heading references inside a prompt item retain their own scopes. The item range is split around each reference.

## Range composition and precedence

A robust replica should compose ranges in this order for each line:

1. Discover file-reference ranges.
2. Discover heading-reference ranges, dropping any that overlap file references.
3. Sort reference ranges by start and end position.
4. Emit broad heading-text and prompt-item ranges only for gaps not occupied by references.
5. Emit the narrow reference ranges.
6. Sort all semantic ranges by line and start before sending them to the rendering layer.

Structural captures may overlap these semantic ranges because they belong to a separate highlighting layer. Within the contextual layer, however, ranges must be non-overlapping. This matters most for references in headings and list items.

Positions are measured per physical line using JavaScript string indices, which correspond to UTF-16 code units. A host with byte-, code-point-, or grapheme-based positions must convert offsets consistently. The recognized special syntax itself is ASCII, but non-ASCII text before a token can expose incorrect conversions.

## Bracket and delimiter behavior

The language pairs `()`, `[]`, `{}`, double quotes, single quotes, and backticks. Parentheses, square brackets, quotes, and backticks close inline; braces permit newline-oriented pairing. Quote, backtick, and fenced-code delimiter pairs are excluded from rainbow coloring, while ordinary bracket pairs may participate.

This pairing is an editing aid rather than prompt syntax. In particular, automatic `[]` pairing can help author heading references, but bracket pairing does not determine whether a bracketed range resolves as a heading.

## Limitations and compatibility choices

A reproducing implementation should make the following constraints explicit:

- Prompt references are semantic overlays found by regular expressions, not grammar nodes.
- Workspace references are root-relative; document-relative paths are not supported.
- File targets are checked synchronously in the reference model and include directories.
- Heading targets are local to the current document and ATX-only.
- Heading identity is exact and case-sensitive, including literal Markdown formatting.
- Semantic scans do not generally respect Markdown or embedded-language context.
- The semantic fence detector and the structural Markdown parser can disagree on malformed input.
- Inline Markdown parsing is not guaranteed by the supplied language query.
- Generic Markdown and prompt-document semantics use the same suffix test.
- Fixed colors exist only as fallbacks for existing and missing file references; most other appearance is theme-derived.

These are important compatibility boundaries. Improving them—such as ignoring references in code, supporting escaped brackets, resolving paths relative to the document, indexing Setext headings, or case-folding heading names—may be desirable, but changes the language's current meaning.

## Replication checklist

To reproduce the feature in another editor or rendering host:

1. Parse with a compatible CommonMark/GFM block grammar and apply the structural capture table above.
2. Add YAML, TOML, HTML, and language-tagged fence injections.
3. Optionally add the separate Markdown inline parser if detailed inline styling is required.
4. Restrict workspace-sensitive prompt semantics to the designated prompt filenames.
5. Implement the exact file-reference boundary and path alphabet, safe root-relative resolution, and existence-dependent scopes.
6. Build the ATX-only heading stack, normalize bracket paths, and resolve exact current-document matches.
7. Implement lexical exclusions for tasks, images, and immediate inline links.
8. Apply the repeating heading palette and whole-line `-`/`*` item scopes.
9. Split broad semantic ranges around references and task markers, and enforce file-reference precedence on overlap.
10. Convert line offsets to the host's coordinate system, especially for non-ASCII prefixes.
11. Refresh contextual colors after text edits and relevant filesystem changes.
12. Test references inside headings, list items, fences, inline code, malformed brackets, duplicate headings, skipped heading levels, missing targets, and paths attempting to escape the workspace.
13. Test task markers: `-`, `*`, and `+` items; `x`, `X`, and custom markers; unchecked `[ ]`; over-wide `[xy]`; markers without trailing whitespace; bracketed text away from the list marker; and coexistence with a reference on the same line.

The key design principle is separation: Markdown supplies document structure and embedded-language boundaries, while the prompt layer adds workspace-aware and document-aware meaning. Keeping those passes independent, then composing their ranges deterministically, preserves both ordinary Markdown readability and prompt-specific feedback.
