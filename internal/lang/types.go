package lang

import "time"

const (
	MaxIndexEntries = 50_000
	MaxCompletions  = 500
	IndexTTL        = 2 * time.Second
)

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"` // UTF-16 code units, as required by LSP.
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

type Completion struct {
	Label      string   `json:"label"`
	Kind       int      `json:"kind,omitempty"`
	Detail     string   `json:"detail,omitempty"`
	SortText   string   `json:"sortText,omitempty"`
	FilterText string   `json:"filterText,omitempty"`
	TextEdit   TextEdit `json:"textEdit"`
}

type CompletionList struct {
	IsIncomplete bool         `json:"isIncomplete"`
	Items        []Completion `json:"items"`
}

type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    Range         `json:"range"`
}

type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

type TokenKind int

const (
	TokenFile TokenKind = iota
	TokenFileMissing
	TokenHeadingRef
	TokenHeadingRefMissing
	TokenHeadingMarker
	TokenHeading1
	TokenHeading2
	TokenHeading3
	TokenHeading4
	TokenHeading5
	TokenHeading6
	TokenPromptItem
)

var SemanticLegend = []string{
	"promptFile", "promptFileMissing", "promptHeadingReference", "promptHeadingReferenceMissing",
	"promptHeadingMarker", "promptHeading1", "promptHeading2", "promptHeading3",
	"promptHeading4", "promptHeading5", "promptHeading6", "promptItem",
}

type SemanticToken struct {
	Line, Start, Length int
	Kind                TokenKind
}

type Heading struct {
	Level                              int
	Line, LineUTF16, MarkerUTF16       int
	ContentStartUTF16, ContentEndUTF16 int
	Text, Path                         string
}

type Reference struct {
	StartRune, EndRune   int
	StartUTF16, EndUTF16 int
	Value                string
	File                 bool
}
