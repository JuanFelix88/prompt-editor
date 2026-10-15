# Implementation Concepts

## Purpose and scope

This document describes how the implementation works as a system, without depending on its implementation languages or on one editor's API. It is intended to be sufficient for reimplementing the same behavior in another host or language.

Three companion documents own the feature-level details and should be read for exact user-facing behavior:

- [Syntax highlighting](syntax-highlighting.md) defines the prompt reference forms and recognition rules.
- [IntelliSense](intellisense.md) defines completion, hover, definition, and semantic feedback.
- [Markdown support](markdown-support.md) defines baseline Markdown parsing, highlighting, outlines, brackets, and embedded-language behavior.

This document does not repeat their catalogs. Instead, it explains the architecture that discovers resources, parses context, resolves references, maintains state, and transports the resulting information.

## System model

The extension is a small adapter around a standalone language-service process, plus a declarative Markdown language package.

```text
installed package
  ├─ manifest and language registration
  ├─ compiled host adapter
  │    └─ embedded copy of the language-service program
  ├─ Markdown grammar and declarative queries/styles
  └─ source assets used to build the above

one opened workspace/worktree
  └─ one service process
       ├─ framed request/notification transport over standard streams
       ├─ in-memory snapshots of open documents
       ├─ short-lived per-directory file indexes
       ├─ direct filesystem resolver
       └─ stateless parsers and feature handlers
```

The major architectural split is deliberate:

1. **The host adapter** validates the requested service, materializes the embedded server program into a writable extension working directory, locates a host-provided script runtime, and launches the service with the workspace root.
2. **The Markdown layer** supplies general syntax-tree behavior: ordinary Markdown highlighting, bracket pairing, outline extraction, and embedded-language injection.
3. **The language-service layer** supplies workspace-aware behavior that a static grammar cannot: reference resolution, ranked suggestions, hover, navigation, and semantic status.
4. **The style mapping layer** translates semantic categories such as “existing file reference” or “missing heading reference” into host theme concepts and a few explicit visual fallbacks.

The Markdown parser and language service do not share a parse tree. General Markdown behavior is grammar-driven, while prompt-aware service behavior uses focused line scanners. This keeps the service dependency-free and portable, at the cost of some Markdown edge-case divergence.

## Component responsibilities

### Package manifest and registration

The manifest binds one named language to one language service, pins the Markdown grammar source revision, and describes the compiled adapter. Registration is configuration, not runtime discovery: names and identifiers must agree among the manifest, language definition, and adapter.

The language definition and service both recognize general Markdown (`.md`) and Prompt Lang (`.prompt`) suffixes. Prompt-reference features and list styling therefore apply to both.

### Host adapter

The adapter contains the server source as a build-time resource. On each command request it:

1. rejects unknown service identifiers;
2. creates a fixed service subdirectory under the extension's current working directory;
3. overwrites the service program with the embedded version;
4. turns that location into an absolute path;
5. requests the host's script-runtime executable; and
6. returns a command containing the script path and explicit workspace-root argument.

Materializing an embedded program avoids distributing or resolving a separate server package. Overwriting on launch guarantees that the executable adapter and extracted server stay at the same version. It also means the working directory must be writable and concurrent launches share the same extracted path.

### Language service

The service is a single long-lived process with a configured workspace fallback. File-reference bases are derived independently from each document URI. Its handlers are thin orchestration around reusable operations:

- URI/path conversion;
- prompt-document classification;
- document snapshot retrieval;
- line and token context extraction;
- heading scanning and hierarchy construction;
- directory indexing and document-relative path resolution;
- feature-specific ranking or result formatting; and
- semantic-token conflict removal and encoding.

There is no database, background worker, watcher, network access, or persistent cache.

### Declarative Markdown resources

A pinned Markdown grammar provides the base syntax tree. Declarative queries assign ordinary Markdown styles, produce section outlines, pair delimiters, and mark fenced code, HTML, and metadata regions for embedded parsing. Semantic service results are layered over those grammar results. See [Markdown support](markdown-support.md) for the exact query behavior and precedence expectations.

## Process and protocol lifecycle

### Startup

The workspace root is normally passed on the process command line by the adapter. It is immediately converted to an absolute normalized path. If no explicit root was passed, the process starts with its current directory and may replace it during protocol initialization with the client's root or first workspace folder.

An explicit command-line workspace wins over initialization data for the configured fallback. It does not override the directory of a document when resolving or completing file references.

### Initialization

The service advertises:

- full-document synchronization;
- completion with prompt-related trigger characters;
- hover and definition lookup; and
- full-document semantic tokens with a fixed, ordered token legend.

The order of that legend is a protocol invariant: encoded token numbers and the style mapping must refer to the same category order.

### Steady state

Open and changed documents are stored in a map keyed by their URI. Changes replace the complete text snapshot; incremental edits are not applied. Closing a document removes its snapshot.

A feature request obtains text in this order:

1. use the open-document snapshot, including unsaved edits;
2. otherwise convert the URI to a local path and read it from disk;
3. on conversion or read failure, use empty text.

This makes open snapshots authoritative and keeps closed-document support simple.

### Shutdown

A shutdown request is acknowledged but does not itself terminate the process. A later exit notification terminates it. This follows the usual two-phase language-service lifecycle. Unexpected end-of-input is left to the runtime's normal process behavior.

### Framing boundary

Messages use JSON-RPC objects framed by an ASCII `Content-Length` header, a blank line, and an exact number of UTF-8 body bytes. Input is accumulated because a read may contain a partial message or several messages. Complete frames are parsed and dispatched in order. Output uses the same byte-counted framing; logs go to the error stream so they cannot corrupt protocol output.

A malformed header is discarded through its terminator. Malformed JSON is logged. Unknown requests receive a method-not-found response, while unknown notifications are ignored. Exceptions inside known request handling are logged and converted to internal-error responses.

## Control and data flow

### Completion

```text
cursor request
  → classify document
  → fetch authoritative text
  → extract current line
  → infer heading context first, otherwise file context
  → scan current headings OR obtain the requested directory index
  → score, sort, and cap candidates
  → attach one replacement range to every candidate
  → return candidates plus an “incomplete” flag
```

Heading context has priority when both recognizers could theoretically apply. Replacement affects only the text after the opening delimiter, preserving the delimiter itself. Heading insertion adds a closing bracket only when one is not already at the cursor. File insertion uses document-relative paths, preserves explicit dot-directory prefixes, and appends a slash to folders.

The exact trigger, filtering, ranking, labels, and insertion semantics belong to [IntelliSense](intellisense.md).

### Hover and definition

Both features inspect only the requested line. File-reference recognition is attempted first; heading-reference recognition is the fallback.

For a file reference, the resolver normalizes the path relative to the document directory, then performs a fresh filesystem status lookup. Parent traversal is permitted, including outside the workspace. Hover reports existence, kind, and absolute destination; definition returns the destination at its beginning. Direct status checks intentionally do not depend on the completion index, so navigation and validation reflect filesystem changes immediately.

For a heading reference, the service rescans the current document, normalizes the requested hierarchy, and finds exact path matches. Hover reports the first target and notes duplicates. Definition selects the first match in document order and returns the whole heading line. Missing references produce informative hover content and no definition target.

### Semantic pass

A semantic request performs a whole-document pass:

1. scan headings once and index them by line;
2. for every line, collect prompt reference ranges when prompt behavior is enabled;
3. classify each reference by live file status or heading resolution;
4. emit heading-marker and level-specific heading-text categories;
5. on prompt documents, emit distinct categories for `-` and `*` list items;
6. split broad heading or list spans around reference ranges;
7. emit references into the resulting gaps;
8. sort all tokens by line and start position; and
9. delta-encode them against the fixed semantic legend.

The segmentation step is important: protocol semantic tokens must not overlap. Prompt references take precedence over broader heading and list styling. A heading-looking bracket range that overlaps an already recognized file range is skipped for the same reason.

Positions are line-relative character offsets. A compatible reimplementation must use the protocol's expected character unit, not UTF-8 byte offsets.

## Resource discovery

Completion discovery and reference validation are separate mechanisms.

### Directory indexes

The completion index is built breadth-first from the document directory, or from the directory explicitly selected by a prefix containing `./` or `../` segments. Indexes are cached separately by scan directory. Within each visited directory, directories are ordered before files and names are ordered lexically. Breadth-first traversal naturally favors shallow results and lets directories enter the queue before their descendants.

A fixed exclusion set omits common metadata, editor settings, dependency, generated-output, coverage, and build directories. Exclusion is by exact directory basename. Unreadable directories are skipped without failing the request.

Each entry records:

- normalized relative path;
- insertion path, with a trailing slash for directories;
- basename;
- directory flag; and
- depth.

The scan stops at 50,000 entries. Each directory result is cached for two seconds and then lazily rebuilt by the next completion request. At most 64 indexes are retained; adding a new directory beyond that limit clears the index map first. Results are capped at 500 candidates. The response is marked incomplete when either the index was truncated or more candidates matched than were returned.

### Live validation

Hover, definition, and uncached semantic status resolve each reference directly and perform a current filesystem status check. A failed check is represented as a known candidate path with no status, while an invalid path has no candidate path at all. User-facing behavior treats both as missing. The TUI can cache semantic existence briefly, keyed by the resolved absolute path rather than reference text.

No file contents are indexed. Completion discovery is metadata-only.

## Path and context normalization

### Document-relative paths

The canonical reference form is relative to the directory of the file being edited and uses `/` separators. Untitled editor buffers use the configured workspace, defaulting to the process working directory. Normalization:

1. converts alternate separators to `/`;
2. removes one leading reference marker;
3. removes all leading `/` characters; and
4. preserves case and remaining path text.

Resolution rejects embedded null characters and absolute or drive-qualified paths remaining after normalization. It joins the normalized text to the document directory and cleans dot segments.

Parent-directory traversal is permitted without a workspace containment check. A document in `/repo/docs/` resolves `@guide.md` to `/repo/docs/guide.md` and `@../../guide.md` to `/guide.md`.

Filesystem status follows symbolic links. Reference resolution is navigational behavior, not a filesystem sandbox.

### Document identity and activation

Document URIs are converted only through local-file URI rules. Snapshot-map keys remain the original URI strings; they are not separately canonicalized. Prompt behavior is enabled by a case-insensitive filename check for the `.md` and `.prompt` suffixes.

The filename gate is intentionally independent of path-reference normalization.

### Cursor context

Feature handlers split text on either common newline convention and select the requested line. Completion examines only text before the cursor. File context uses the last `@`, requires a non-word boundary before it, and requires every following character to belong to the path alphabet. Heading context uses the last unmatched `[`, rejects image syntax, and rejects nested opening brackets.

Reference lookup at the cursor allows the cursor at either boundary of the recognized span. The scanners and completion-context parser are related but separate, so their boundary behavior must be kept aligned deliberately.

## Parsing and matching algorithms

### Heading model

The service recognizes hash-prefixed headings with:

- zero to three leading spaces;
- one through six markers;
- whitespace or end-of-line after the marker; and
- non-empty text after trimming.

Optional trailing closing markers are removed. Headings inside matching backtick or tilde fences are ignored. A closing fence must use the same marker character and at least the opening marker length.

Hierarchy is built with a six-level stack. On a heading at level `N`, deeper/equal stale entries are truncated and the heading text is stored at slot `N`. Empty skipped levels are omitted when forming the path. Consequently, a level jump still yields a usable path rather than inventing missing ancestors. The display/resolution path joins retained parts with ` > `.

Only these hash-prefixed headings participate in service-side heading references. Other Markdown heading forms may still be understood by the base grammar but are outside this resolver.

### Heading references

Bracket candidates are found on one line. Empty values, exact checkbox markers (`x` or `X` after trimming), image labels, and labels immediately followed by `(` are excluded to avoid common Markdown constructs. Candidate content is trimmed.

A query path is normalized by splitting on `>`, trimming each part, removing empty parts, and rejoining with the canonical separator. Matching against computed heading paths is exact and case-sensitive. Duplicate paths remain valid; consumers select the first document-order match unless they need to report multiplicity.

The service does not perform slug matching, fuzzy resolution, Unicode normalization, or cross-document heading search.

### File references

A file token starts with `@` at the beginning of a line or after a non-word character. It must contain at least one path character after the marker. The path alphabet consists of ASCII letters and digits plus `.`, `_`, `~`, `+`, `/`, and `-`. The marker is part of hover and semantic ranges but not the path passed to resolution.

### Candidate ranking

File candidates are assigned the first applicable score:

1. full insertion path starts with the prefix;
2. when the prefix has no slash, basename starts with it;
3. relative path contains the prefix immediately after a slash;
4. relative path contains the prefix anywhere; or
5. no match.

Comparison is case-insensitive. Ties prefer directories, shallower entries, then lexical insertion path. Empty-prefix ranking begins with depth.

Heading ranking similarly prefers canonical path prefix, leaf-heading prefix, then canonical path substring. Ties follow document order, then lexical path. Empty-prefix ranking begins with heading level.

These simple tiers are deterministic and explainable; they intentionally avoid fuzzy matching and usage history.

## State, freshness, and error handling

The mutable process state is limited to:

- configured workspace fallback;
- URI-to-full-text snapshot map;
- bounded directory-to-index map and index build timestamps; and
- resolved-absolute-path semantic existence cache for TUI requests.

Heading scans, line scans, and direct status results are request-local. Unsaved heading edits are immediately visible, while newly created paths may take up to the directory index time-to-live to appear in completion. Hover, definition, and uncached semantic status remain fresh; the TUI's cached semantic status can lag by the same short TTL.

Filesystem failures are generally soft:

- unreadable document becomes empty;
- unreadable indexed directory is skipped;
- failed status becomes “missing”; and
- invalid URI yields no disk text or destination.

Startup/materialization failures are hard and prevent process launch. Protocol handler failures are isolated to the current request. The service publishes semantic status rather than a separate diagnostics stream, and it does not retry failed I/O.

## Core invariants

A faithful implementation should preserve these properties:

1. The configured workspace is only a fallback; it never overrides a document's directory for file references.
2. Open-document text always wins over disk text.
3. File references are interpreted relative to the directory of the document being edited.
4. Parent paths may leave the document directory and workspace; index/status caches must not mix distinct resolved locations.
5. Completion index paths use one separator convention and folder insertions end in `/`.
6. Direct validation does not use the potentially stale completion index.
7. Heading paths are derived from document order and level hierarchy.
8. Heading resolution is normalized for separator whitespace but otherwise exact and case-sensitive.
9. Duplicate heading definitions resolve to the first match.
10. Semantic token categories use the same stable order advertised at initialization.
11. Semantic ranges are sorted and non-overlapping; references override broad line/heading styles.
12. Prompt behavior is guarded by the `.md`/`.prompt` filename predicate.
13. Protocol output contains frames only; logging never shares its stream.

## Design trade-offs

### Benefits

- **Small deployment surface:** one embedded, dependency-free service program and one grammar asset.
- **Predictable behavior:** tiered ranking and exact matching are deterministic.
- **Fresh navigation:** direct status checks avoid cache-induced false positives or negatives.
- **Low state complexity:** full snapshots remove incremental-edit bookkeeping.
- **Host portability:** the service boundary is a standard framed protocol and local filesystem operations.
- **Graceful degradation:** most filesystem and malformed-input problems affect one result rather than the process.

### Costs

- **Repeated work:** headings are rescanned for many requests, and semantic classification may perform one filesystem status operation per reference occurrence.
- **Blocking I/O:** indexing and status calls run synchronously with request handling, so large or slow filesystems can delay all requests.
- **Coarse cache invalidation:** the file index is time-based, not event-based.
- **Parser divergence:** focused scanners do not model all Markdown contexts understood by the grammar.
- **Broad scans:** ordinary file completion scans the document directory tree; explicit dot-directory prefixes scope the scan to the requested directory.
- **Memory scaling:** full text for every open document is retained.
- **Workspace initialization:** initialization chooses one configured fallback rather than merging workspace folders; file references nevertheless use each document's own directory.

The implementation favors minimal packaging and understandable logic over perfect Markdown fidelity, huge-repository scalability, and advanced relevance ranking.

## Known limitations and edge cases

- Each directory index is capped, so entries beyond the breadth-first limit cannot be suggested until the scan scope is reduced or exclusions change.
- Excluded directory names are exact and case-sensitive at scan time.
- Symbolic links are not traversed as indexed directories, but direct status resolution follows them.
- Heading references do not resolve setext-style headings and do not search other documents.
- Prompt reference scanners operate line by line and do not generally suppress references inside inline code, comments, or fenced code. Fence awareness is currently specific to heading-definition scanning.
- Bracket heuristics avoid images, inline links, and simple task boxes, but they are not a complete Markdown inline parser.
- File tokens use a deliberately narrow ASCII path alphabet; spaces, percent escapes, many Unicode names, and several legal filesystem punctuation characters are not recognized as one token.
- A leading slash is normalized into a document-relative reference rather than interpreted as a filesystem-root path.
- File definition targets always begin at line zero; directories may be returned as targets if the host can open them.
- No cancellation, request concurrency, progress reporting, diagnostics, configuration updates, watched-file events, or incremental semantic tokens are implemented.
- The framing parser has no explicit maximum body size.
- The fixed extracted-server path assumes the extension working directory is writable and that simultaneous writers produce identical content.
- Language registration and prompt feature activation include ordinary Markdown suffixes. Host language-selection rules remain part of the effective activation behavior.

## Packaging and runtime assumptions

The source repository contains the adapter source, embedded service source, package metadata, language queries/configuration, lockfile, documentation, and license. Compiled adapter and grammar binaries may exist in a development checkout but are generated/ignored artifacts rather than authoritative tracked source.

A build produces a sandbox-compatible compiled adapter. The server source is captured into that adapter at build time, so changing the standalone service source requires rebuilding/reinstalling the adapter despite the server itself needing no transpilation or dependency installation. The pinned Markdown grammar is built separately into a portable grammar binary.

At runtime the host must provide:

- a writable extension working directory;
- a compatible script runtime with filesystem, path, URI, standard-stream, UTF-8, and JSON support;
- local-file URIs;
- permission to traverse the workspace; and
- semantic-token support if semantic styling is desired.

The runtime service has no package-manager or network dependency. All workspace knowledge comes from the root argument, document notifications, and local filesystem.

## Language-agnostic replication blueprint

1. **Define identifiers and activation.** Choose one language identity, one service identity, the filename gate, and the pinned Markdown grammar. Ensure registration names agree.
2. **Package the adapter.** Embed the server resource, materialize it under a writable deterministic path, and launch it with an explicit absolute workspace root.
3. **Implement framed JSON-RPC transport.** Maintain a byte buffer, parse byte-counted UTF-8 frames, separate logs from responses, and distinguish requests from notifications.
4. **Implement lifecycle/state.** Honor initialization root precedence, advertise full synchronization and supported features, store full snapshots by URI, and implement shutdown/exit.
5. **Create path primitives.** Normalize reference separators, convert local-file URIs, derive each document's base directory, allow parent traversal, and expose fresh status lookup.
6. **Create document primitives.** Implement prompt filename classification, snapshot-first text retrieval, newline-independent line selection, and protocol-compatible character offsets.
7. **Implement heading scanning.** Recognize supported headings, suppress fenced definitions, maintain the level stack, and produce canonical hierarchy paths with source ranges.
8. **Implement reference/context scanners.** Keep file-token, bracket-reference, and completion-context rules distinct but tested against shared boundary cases.
9. **Implement resource discovery.** Breadth-first scan with exclusions, deterministic ordering, metadata records, an entry cap, a short lazy cache, and soft handling of unreadable directories.
10. **Implement feature resolvers.** Add tiered completion ranking and replacement ranges; use live status for file hover/definition; use current snapshots for heading hover/definition.
11. **Implement semantic composition.** Classify references, generate broad heading/list spans, subtract reference blockers, sort ranges, and delta-encode against a stable legend.
12. **Add the Markdown layer.** Configure base highlighting, outlines, delimiters, and injections independently from prompt intelligence, as described in [Markdown support](markdown-support.md).
13. **Verify invariants.** Test document-relative paths, parent traversal outside the workspace, separate caches for multiple documents, untitled fallback, unsaved text precedence, duplicate headings, skipped heading levels, fence handling, overlapping style ranges, truncated indexes, partial protocol frames, malformed requests, and launch failures.
14. **Package reproducibly.** Pin grammar and host-contract versions, rebuild embedded resources, and treat generated binaries as outputs rather than editable source.

A replication is behaviorally equivalent when the same workspace snapshot and cursor position produce the same normalized candidates, first-match targets, existence classifications, non-overlapping semantic categories, and protocol lifecycle outcomes—even if its internal language, host integration, parser library, or concurrency model differs.
