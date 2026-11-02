package query

import (
	"unicode/utf16"
	"unicode/utf8"
)

// Error codes. A client keys its message and its notice on these.
const (
	CodeEmpty        = "empty"
	CodeTooLong      = "too_long"
	CodeTooDeep      = "too_deep"
	CodeTooMany      = "too_many_conditions"
	CodeSyntax       = "syntax"
	CodeUnknownField = "unknown_field"
	CodeBadValue     = "bad_value"
	CodeBadRange     = "bad_range"
	CodeUnknownName  = "unknown_name"
	CodeAmbiguous    = "ambiguous"
	CodeOwnerOnly    = "owner_only"
)

// Candidate is one entity an ambiguous name could mean.
type Candidate struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Error is a query that can't run, pointing at the part that's wrong (P0-7). Start and
// End are offsets in UTF-16 code units — the unit the SPA slices a string by — so the
// error echo underlines the same characters the server meant.
type Error struct {
	Code        string      `json:"code"`
	Message     string      `json:"message"`
	Start       int         `json:"start"`
	End         int         `json:"end"`
	Field       string      `json:"field,omitempty"`
	Suggestions []string    `json:"suggestions,omitempty"`
	Candidates  []Candidate `json:"candidates,omitempty"`
}

func (e *Error) Error() string { return e.Message }

// errAt builds an *Error for the byte span [pos, end) of the source.
func (q *Query) errAt(code, msg string, pos, end int) *Error {
	return newError(q.src, code, msg, pos, end)
}

func newError(src, code, msg string, pos, end int) *Error {
	return &Error{Code: code, Message: msg, Start: utf16Offset(src, pos), End: utf16Offset(src, end)}
}

// utf16Offset converts a byte offset into src to UTF-16 code units.
func utf16Offset(src string, byteOff int) int {
	byteOff = min(max(byteOff, 0), len(src))
	n := 0
	for _, r := range src[:byteOff] {
		if r == utf8.RuneError {
			n++
			continue
		}
		n += utf16.RuneLen(r)
	}
	return n
}
