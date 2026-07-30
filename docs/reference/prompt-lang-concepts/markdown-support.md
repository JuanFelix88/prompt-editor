# Markdown support and prompt context

## Scope and evidence

This repository implements a Markdown-derived authoring mode, **Markdown Prompt**, rather than a prompt template runtime. Its two layers have different responsibilities:

1. A pinned `tree-sitter-markdown` block grammar and language queries provide Markdown structure, highlighting, outline data, bracket pairing, and embedded-language regions.
2. The dependency-free server in `lsp/prompt-lsp.mjs` scans text and the workspace for prompt-specific file and heading references.

The descriptions below are based on `languages/markdown-prompt/*`, the vendored grammar at revision `c357072…`, and the current server implementation. Claims in the README are treated as intended user-facing behavior; where configuration or code differs, that difference is called out.

## Markdown model

### Block constructs

The registered grammar is the block half of `tree-sitter-markdown`. It parses CommonMark-oriented blocks including:

- paragraphs and sections;
- ATX headings (`#` through `######`) and Setext headings;
- unordered and ordered lists, nested lists, block quotes, and thematic breaks;
- indented and fenced code blocks;
- HTML blocks;
- link reference definitions;
- GFM pipe tables and task-list markers;
- YAML `---` metadata and TOML `+++` metadata at the start of a document.

The vendored source enables GFM, task lists, strikethrough, pipe tables, YAML/TOML metadata, and LaTeX by default. That describes the grammar build's capabilities, not equal editor treatment for every construct. Tags and wiki links are disabled by default. More importantly, strikethrough, LaTeX, emphasis, code spans, ordinary inline links, images, autolinks, and raw inline HTML belong to the repository's separate `markdown_inline` grammar. The active Markdown Prompt injection query does **not** inject `markdown_inline` into `inline` nodes, and only the block grammar is declared for this language. Consequently, the repository does not provide evidence that all vendored inline capabilities are active in Markdown Prompt documents.

The project highlight query explicitly covers paragraphs, indented code, tables, ATX and Setext headings, list and quote markers, table punctuation, fence delimiters/info strings, and link-reference destinations. It is intentionally narrower than the full Markdown syntax. For example, task markers are parsed but have no dedicated query capture, and ordinary emphasis is not addressed by this query.

### Heading structure and outline

The grammar groups ATX headings and their descendants into nested `section` nodes. The outline query exposes those ATX sections, using the heading marker as context and heading content as the name. Setext headings are highlighted but are not included by the current outline query.

Prompt heading navigation uses a separate line scanner rather than the syntax tree. It recognizes only ATX headings with zero to three leading spaces, one to six `#` characters, and whitespace or end-of-line after the marker. Empty headings are ignored. Optional trailing closing `#` characters are removed. Therefore Setext headings can be Markdown headings without being prompt-reference targets.

## Prompt-context notation

### Workspace file and folder references

A token such as `@src/lib.rs` or `@docs/` is a workspace-root-relative reference. The implementation accepts one or more ASCII letters, digits, `.`, `_`, `~`, `+`, `/`, or `-` after `@`. The `@` must be at the beginning of a line or preceded by a non-word character, which avoids treating the domain portion of a typical email address as a reference.

A valid reference can point to either a file or directory. The server can complete it, report whether it exists on hover, and navigate to an existing target. Missing targets receive a distinct semantic style. Resolution is against the workspace root, **not** against the directory containing the prompt document. Leading `@` and leading slashes are normalized away. Lexical paths that escape the workspace are rejected, although the implementation does not canonicalize symlinks before checking the boundary.

These references are navigational context annotations only. The implementation does not read referenced file contents into a prompt, recursively expand references, enforce file types, or submit context to a model.

### Heading references

`[Heading]` addresses an ATX heading in the current document. `[Parent > Child]` addresses a hierarchy assembled from heading levels. Spaces around each `>` component are normalized, but component text is otherwise exact and case-sensitive. A bare child heading is not an alias for its full path: if the scanner records `Review > Style`, `[Style]` does not resolve it.

The scanner deliberately excludes:

- empty brackets;
- task checkboxes `[x]` and `[X]` (after trimming);
- image syntax when `[` is immediately preceded by `!`;
- inline links when `]` is immediately followed by `(`.

Other Markdown bracket forms are not distinguished. In particular, shortcut/reference-style links, link labels, and bracketed prose can be interpreted as prompt heading references. Matching duplicates resolve to the first heading; hover notes the ambiguity.

### Directives and variables

There is no implemented directive language, variable declaration, interpolation syntax such as `{{name}}` or `${name}`, conditional/include directive, frontmatter-to-variable binding, or environment-variable expansion. YAML and TOML metadata are embedded for language-aware editing only. In the current repository, “prompt context” specifically means workspace `@` references and same-document heading references.

## Fenced code and embedding

A fenced block with an info-string language, for example ` ```rust `, exposes its content as that named embedded language. HTML blocks are exposed as HTML. Initial `---` and `+++` metadata blocks are exposed as YAML and TOML respectively. An absent, misspelled, or unavailable fence language has no project-defined fallback beyond remaining code-fence content. Indented code is highlighted as text but has no language selection mechanism.

The query layer and prompt server are independent, which creates important boundaries:

- The heading scanner is fence-aware. It ignores ATX-looking lines between matching backtick or tilde fences. A closer must use the same character and at least the opener's length. An unclosed fence suppresses all later heading targets.
- File references, heading-reference brackets, completions, hover, navigation, and prompt semantic-reference scanning are line/regex based and are **not** excluded inside fenced code, indented code, inline code, HTML, or metadata.
- Hyphen-list semantic styling is also syntax-agnostic and can apply to `- ` lines inside a fence.
- Fence detection for heading indexing only handles fences indented zero to three spaces. It does not model container prefixes such as block quotes as fully as the Markdown parser does.

Thus embedding changes parsing/highlighting for fence content, but it does not create an IntelliSense isolation boundary for prompt notation.

## IntelliSense boundaries

Prompt-aware completion is available only when the server's filename predicate considers the document a prompt document. File completion starts after `@`; continued `/`, `.`, `-`, and `_` typing can re-query because they are configured as word/completion characters. Heading completion starts after an unmatched `[`. The server supplies paths from the whole workspace and heading paths from the current document.

Workspace completion:

- walks breadth-first and ranks direct prefix matches ahead of basename and substring matches;
- offers both files and directories, appending `/` to directories;
- skips a fixed set of common generated/VCS directories, including `.git`, `node_modules`, `target`, `build`, `dist`, and `vendor`;
- caches the index for two seconds, stops at 50,000 entries, and returns at most 500 matches.

Heading completion ranks full-path prefixes, then heading-text prefixes, then path substrings. Lower-level headings are ranked after higher-level headings when there is no prefix. If no closing `]` is present at the cursor, completion inserts one.

The server does not implement diagnostics, code actions, reference search, rename, formatting, prompt execution, or cross-document heading references. “Missing” coloring is produced by semantic token selection, not a diagnostic. File hover reports existence and an absolute path; it does not preview file contents despite the README's phrase “hover to preview.”

## File and language detection

The language configuration and server predicate both recognize the broad, case-insensitive suffixes `md` and `prompt`. Prompt IntelliSense and prompt-reference semantics therefore apply to every `.md` and `.prompt` file.

Reference and hyphen-list semantic tokens are suffix-gated, while ATX heading semantic tokens are generated regardless of the filename. Whether those tokens are observed depends on which documents are connected to this language/server.

Non-file URIs cannot satisfy the prompt filename predicate because URI conversion must produce a filesystem path. Filename matching is case-insensitive, while reference existence follows platform filesystem behavior.

## Edge cases and limitations

- Paths cannot contain spaces, Unicode letters, backslashes, `:`, `#`, or query-like punctuation; scanning stops at the first unsupported character.
- `@../outside` is lexically recognized and may be colored missing, but resolution refuses workspace traversal.
- Bracket references are single-line and cannot escape `]`; Markdown escaping is not interpreted by the prompt scanner.
- Reference scanning is unaware of comments and most Markdown node boundaries, so literal examples may become active references.
- Heading hierarchy follows the latest heading at each level. Skipped levels are compacted when path text is built, so unusual heading-level jumps may yield paths that do not mirror a strict section tree.
- Heading matching includes inline Markdown punctuation literally because the line scanner does not normalize emphasis, links, entities, or code spans in heading text.
- Semantic ranges use JavaScript string indices. The protocol normally measures UTF-16 code units, so this is consistent for lines containing non-BMP characters, but the accepted file-path alphabet remains ASCII.
- The grammar upstream explicitly warns that Tree-sitter Markdown has known parse inaccuracies and is designed primarily for syntactic/editor information, not correctness-critical Markdown validation.
- No project-specific automated tests exercise the server's Markdown/prompt interactions; the vendored grammar has its own corpus tests, including known failing cases.

## Replication guidance

To reproduce this support in another editor or language host without copying platform-specific integration details:

1. **Pin the same grammar revision and build options.** Register the block grammar and explicitly decide whether to add the separate inline grammar. Do not infer active inline support merely from its presence in the source tree.
2. **Port the query intent as separate concerns.** Preserve structural Markdown captures, ATX-only outline behavior, language-selected fence embedding, HTML embedding, and YAML/TOML metadata embedding. Validate every queried node against the pinned grammar's node types.
3. **Define prompt-document detection once.** For compatibility with this repository, enable prompt services for the case-insensitive `.md` and `.prompt` suffixes.
4. **Keep context resolution workspace-relative.** Normalize separators, reject traversal, decide how symlinks should be handled, exclude generated directories, and bound indexing/completion work. Preserve directory references if compatibility is desired.
5. **Replicate reference syntax precisely before extending it.** File tokens use the ASCII path alphabet above. Heading paths are current-document, ATX-only, exact/case-sensitive, and separated by normalized `>` components.
6. **Choose an explicit syntax-boundary policy.** Exact compatibility requires prompt references to remain active in code fences and other literal regions while heading definitions are fence-aware. A more predictable implementation should instead derive all prompt features from parsed ranges and consistently suppress them in fenced/indented code, inline code, HTML, and metadata; that would be an intentional behavior change.
7. **Test collisions with Markdown.** At minimum cover images, inline/reference/shortcut links, task boxes, link definitions, bracketed prose, email addresses, escaped punctuation, duplicate headings, skipped heading levels, unclosed/mixed fences, metadata, Unicode surrounding text, excluded directories, traversal, and index truncation.
8. **Keep navigation separate from prompt execution.** If future work actually assembles model context or expands variables/directives, specify ordering, trust boundaries, size limits, and recursion independently; none of those semantics can be inferred from this implementation.
