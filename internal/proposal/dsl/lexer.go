package dsl

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type tokKind int

const (
	tkEOF tokKind = iota
	tkError
	tkIdent
	tkNumber
	tkString

	// Reserved words (docs/coldbru-gqdsl.md §4): never field or lambda names.
	tkSome
	tkNone
	tkEvery
	tkIn
	tkTrue
	tkFalse
	tkAgo
	tkDate

	// Operators of the 0.1 grammar.
	tkEq       // ===
	tkNe       // !==
	tkGt       // >
	tkGte      // >=
	tkLt       // <
	tkLte      // <=
	tkAnd      // &&
	tkOr       // ||
	tkNot      // !
	tkLParen   // (
	tkRParen   // )
	tkLBracket // [
	tkRBracket // ]
	tkComma    // ,
	tkDot      // .
	tkArrow    // =>

	// Lexed only so the parser can reject them with a position.
	tkTilde   // ~=  reserved (D-045)
	tkLooseEq // ==  rejected (D-045)
	tkLooseNe // !=  rejected (D-045)
	tkAssign  // =   assignment, outside the DSL
)

var keywords = map[string]tokKind{
	"some":  tkSome,
	"none":  tkNone,
	"every": tkEvery,
	"in":    tkIn,
	"true":  tkTrue,
	"false": tkFalse,
	"ago":   tkAgo,
	"date":  tkDate,
}

// operators lists the operator spellings, longest first so that === wins over
// == and => over =.
var operators = []struct {
	text string
	kind tokKind
}{
	{"===", tkEq}, {"!==", tkNe},
	{"==", tkLooseEq}, {"!=", tkLooseNe}, {"~=", tkTilde},
	{">=", tkGte}, {"<=", tkLte}, {"=>", tkArrow}, {"&&", tkAnd}, {"||", tkOr},
	{">", tkGt}, {"<", tkLt}, {"!", tkNot}, {"=", tkAssign},
	{"(", tkLParen}, {")", tkRParen}, {"[", tkLBracket}, {"]", tkRBracket},
	{",", tkComma}, {".", tkDot},
}

// token is a lexeme. text is the source text; for strings val is the decoded
// content; for tkError err carries the lexical error.
type token struct {
	kind       tokKind
	text       string
	val        string
	start, end Pos
	err        *Error
}

type lexer struct {
	src  string
	off  int
	line int
	col  int
}

// lex scans the whole source. The result always ends with a tkEOF token, or
// with a tkError token at the first lexical error.
func lex(src string) []token {
	l := &lexer{src: src, line: 1, col: 1}
	var toks []token
	for {
		t := l.scan()
		toks = append(toks, t)
		if t.kind == tkEOF || t.kind == tkError {
			return toks
		}
	}
}

func (l *lexer) pos() Pos { return Pos{Offset: l.off, Line: l.line, Col: l.col} }

// peekRune decodes the rune at the current offset without consuming it.
// ok is false at the end of the input; an invalid byte returns RuneError, 1.
func (l *lexer) peekRune() (r rune, size int) {
	if l.off >= len(l.src) {
		return 0, 0
	}
	return utf8.DecodeRuneInString(l.src[l.off:])
}

// advance consumes one rune.
func (l *lexer) advance() {
	r, size := l.peekRune()
	if size == 0 {
		return
	}
	l.off += size
	if r == '\n' {
		l.line++
		l.col = 1
		return
	}
	l.col++
}

func (l *lexer) errorf(code Code, start Pos, format string, args ...any) token {
	err := newError(l.src, code, fmt.Sprintf(format, args...), start, l.pos())
	return token{kind: tkError, start: err.Start, end: err.End, err: err}
}

func isIdentStart(r rune) bool { return r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' }
func isDigit(r rune) bool      { return r >= '0' && r <= '9' }
func isIdentPart(r rune) bool  { return isIdentStart(r) || isDigit(r) }

func (l *lexer) scan() token {
	for {
		r, size := l.peekRune()
		if size == 0 || !(r == ' ' || r == '\t' || r == '\n' || r == '\r') {
			break
		}
		l.advance()
	}
	start := l.pos()
	r, size := l.peekRune()
	switch {
	case size == 0:
		return token{kind: tkEOF, start: start, end: start}
	case r == utf8.RuneError && size == 1:
		l.advance()
		return l.errorf(CodeSyntaxError, start, "invalid UTF-8 byte in the expression")
	case isIdentStart(r):
		return l.scanIdent(start)
	case isDigit(r):
		return l.scanNumber(start)
	case r == '-':
		if l.off+1 < len(l.src) && isDigit(rune(l.src[l.off+1])) {
			return l.scanNumber(start)
		}
		l.advance()
		return l.errorf(CodeSyntaxError, start, "unexpected character '-'; a negative number is written -5, with the sign directly before the digits")
	case r == '"' || r == '\'':
		return l.scanString(start, r)
	}
	for _, op := range operators {
		if strings.HasPrefix(l.src[l.off:], op.text) {
			for range op.text {
				l.advance()
			}
			return token{kind: op.kind, text: op.text, start: start, end: l.pos()}
		}
	}
	l.advance()
	return l.errorf(CodeSyntaxError, start, "%s", unexpectedCharMessage(r))
}

func unexpectedCharMessage(r rune) string {
	switch r {
	case '/', '#':
		return fmt.Sprintf("unexpected character %q; comments are not supported", r)
	case '+':
		return "unexpected character '+'; numbers have no leading +, write 5 instead of +5"
	case '`':
		return "unexpected character '`'; template strings are not supported, quote strings with ' or \""
	case '&':
		return "unexpected character '&'; use && for logical AND"
	case '|':
		return "unexpected character '|'; use || for logical OR"
	}
	return fmt.Sprintf("unexpected character %q; identifiers are ASCII letters, digits and _", r)
}

func (l *lexer) scanIdent(start Pos) token {
	from := l.off
	for {
		r, size := l.peekRune()
		if size == 0 || !isIdentPart(r) {
			break
		}
		l.advance()
	}
	text := l.src[from:l.off]
	kind, ok := keywords[text]
	if !ok {
		kind = tkIdent
	}
	return token{kind: kind, text: text, start: start, end: l.pos()}
}

// scanNumber scans -?[0-9]+(\.[0-9]+)?. Exponents, underscores and a trailing
// dot are rejected here, with a message that names the rule (D-065).
func (l *lexer) scanNumber(start Pos) token {
	from := l.off
	if r, _ := l.peekRune(); r == '-' {
		l.advance()
	}
	l.digits()
	if l.off+1 < len(l.src) && l.src[l.off] == '.' && isDigit(rune(l.src[l.off+1])) {
		l.advance()
		l.digits()
	}
	if r, size := l.peekRune(); size > 0 && (r == '.' || isIdentStart(r)) {
		l.advance()
		return l.errorf(CodeSyntaxError, start, "malformed number: numbers are -?digits with an optional .digits part; exponents (1e3), underscores (1_000) and a trailing dot are not supported")
	}
	return token{kind: tkNumber, text: l.src[from:l.off], start: start, end: l.pos()}
}

func (l *lexer) digits() {
	for {
		r, size := l.peekRune()
		if size == 0 || !isDigit(r) {
			return
		}
		l.advance()
	}
}

// scanString scans a quoted string. Escapes are \\, \" and \'; a string
// cannot span lines (D-065).
func (l *lexer) scanString(start Pos, quote rune) token {
	l.advance() // opening quote
	var val strings.Builder
	for {
		r, size := l.peekRune()
		switch {
		case size == 0 || r == '\n' || r == '\r':
			return l.errorf(CodeSyntaxError, start, "unterminated string: close it with %c on the same line (strings cannot span lines)", quote)
		case r == utf8.RuneError && size == 1:
			bad := l.pos()
			l.advance()
			return l.errorf(CodeSyntaxError, bad, "invalid UTF-8 byte in a string")
		case r == quote:
			l.advance()
			return token{kind: tkString, text: l.src[start.Offset:l.off], val: val.String(), start: start, end: l.pos()}
		case r == '\\':
			escStart := l.pos()
			l.advance()
			n, nsize := l.peekRune()
			if nsize == 0 || n == '\n' || n == '\r' {
				return l.errorf(CodeSyntaxError, start, "unterminated string: close it with %c on the same line (strings cannot span lines)", quote)
			}
			if n != '\\' && n != '"' && n != '\'' {
				l.advance()
				return l.errorf(CodeSyntaxError, escStart, "invalid escape \\%c: the only escapes are \\\\, \\\" and \\'", n)
			}
			l.advance()
			val.WriteRune(n)
		default:
			l.advance()
			val.WriteRune(r)
		}
	}
}
