package dsl

import (
	"slices"
	"testing"
)

func lexKinds(src string) []tokKind {
	var kinds []tokKind
	for _, t := range lex(src) {
		kinds = append(kinds, t.kind)
	}
	return kinds
}

func TestLex(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		src  string
		want []tokKind
	}{
		{"empty", "", []tokKind{tkEOF}},
		{"whitespace only", " \t\r\n ", []tokKind{tkEOF}},
		{"all operators", "=== !== > >= < <= && || ! ( ) [ ] , . =>",
			[]tokKind{tkEq, tkNe, tkGt, tkGte, tkLt, tkLte, tkAnd, tkOr, tkNot, tkLParen, tkRParen, tkLBracket, tkRBracket, tkComma, tkDot, tkArrow, tkEOF}},
		{"rejected operators", "~= == != =",
			[]tokKind{tkTilde, tkLooseEq, tkLooseNe, tkAssign, tkEOF}},
		{"longest operator wins", "a===b a==b a=>b a!==b a!=b",
			[]tokKind{tkIdent, tkEq, tkIdent, tkIdent, tkLooseEq, tkIdent, tkIdent, tkArrow, tkIdent, tkIdent, tkNe, tkIdent, tkIdent, tkLooseNe, tkIdent, tkEOF}},
		{"reserved words", "some none every in true false ago date",
			[]tokKind{tkSome, tkNone, tkEvery, tkIn, tkTrue, tkFalse, tkAgo, tkDate, tkEOF}},
		{"reserved prefix is an identifier", "somewhere inx dates _ago",
			[]tokKind{tkIdent, tkIdent, tkIdent, tkIdent, tkEOF}},
		{"identifiers", "city _x a1 A_b2", []tokKind{tkIdent, tkIdent, tkIdent, tkIdent, tkEOF}},
		{"numbers", "0 42 -7 3.14 -0.5 007", []tokKind{tkNumber, tkNumber, tkNumber, tkNumber, tkNumber, tkNumber, tkEOF}},
		{"minus glued to previous token", "a-5", []tokKind{tkIdent, tkNumber, tkEOF}},
		{"strings", `'a' "b" '' "it's" 'say "x"'`, []tokKind{tkString, tkString, tkString, tkString, tkString, tkEOF}},
		{"stops at first error", "a $ b", []tokKind{tkIdent, tkError}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := lexKinds(tt.src); !slices.Equal(got, tt.want) {
				t.Errorf("lex(%q) kinds = %v, want %v", tt.src, got, tt.want)
			}
		})
	}
}

func TestLexTokenText(t *testing.T) {
	t.Parallel()
	toks := lex(`-12.50 'it\'s' "a\\b\"" Ünï`)
	// Ünï fails at the first non-ASCII rune; the earlier tokens are intact.
	if len(toks) != 4 || toks[3].kind != tkError {
		t.Fatalf("tokens = %+v", toks)
	}
	if toks[0].text != "-12.50" {
		t.Errorf("number text = %q, want -12.50 (kept as text)", toks[0].text)
	}
	if toks[1].val != "it's" {
		t.Errorf("single-quoted value = %q, want it's", toks[1].val)
	}
	if toks[2].val != `a\b"` {
		t.Errorf("escaped value = %q, want a\\b\"", toks[2].val)
	}
}

func TestLexPositions(t *testing.T) {
	t.Parallel()
	toks := lex("ab ===\n  \"İ\" x")
	want := []struct {
		kind       tokKind
		start, end string
	}{
		{tkIdent, "1:1/0", "1:3/2"},
		{tkEq, "1:4/3", "1:7/6"},
		{tkString, "2:3/9", "2:6/13"}, // İ is two bytes but one column
		{tkIdent, "2:7/14", "2:8/15"},
		{tkEOF, "2:8/15", "2:8/15"},
	}
	if len(toks) != len(want) {
		t.Fatalf("got %d tokens, want %d", len(toks), len(want))
	}
	for i, w := range want {
		if toks[i].kind != w.kind || fmtPosOff(toks[i].start) != w.start || fmtPosOff(toks[i].end) != w.end {
			t.Errorf("token %d = kind %v %s-%s, want kind %v %s-%s", i,
				toks[i].kind, fmtPosOff(toks[i].start), fmtPosOff(toks[i].end), w.kind, w.start, w.end)
		}
	}
}

func TestLexErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		src  string
		from string // line:col of Start
		to   string // line:col of End
		msg  string // substring of the message
	}{
		{"exponent", "x === 1e3", "1:7", "1:9", "exponents"},
		{"underscore", "x === 1_000", "1:7", "1:9", "underscores"},
		{"trailing dot", "x === 5.", "1:7", "1:9", "trailing dot"},
		{"leading plus", "x === +5", "1:7", "1:8", "leading +"},
		{"lone minus", "x === -", "1:7", "1:8", "directly before the digits"},
		{"minus before name", "x === -y", "1:7", "1:8", "directly before the digits"},
		{"unterminated at end", "x === 'abc", "1:7", "1:11", "unterminated string"},
		{"newline in string", "x === \"a\nb\"", "1:7", "1:9", "unterminated string"},
		{"carriage return in string", "x === 'a\rb'", "1:7", "1:9", "unterminated string"},
		{"invalid escape", `x === 'a\q'`, "1:9", "1:11", `invalid escape \q`},
		{"escaped newline", `x === 'a\n'`, "1:9", "1:11", `invalid escape \n`},
		{"backslash at end", `x === 'a\`, "1:7", "1:10", "unterminated string"},
		{"slash comment", "a === 1 // c", "1:9", "1:10", "comments are not supported"},
		{"hash comment", "a === 1 # c", "1:9", "1:10", "comments are not supported"},
		{"single ampersand", "a & b", "1:3", "1:4", "use &&"},
		{"single pipe", "a | b", "1:3", "1:4", "use ||"},
		{"backtick", "x === `a`", "1:7", "1:8", "template strings"},
		{"dollar", "a $ b", "1:3", "1:4", "unexpected character '$'"},
		{"non-ascii identifier", "city === \"İstanbul\" && cıty === 1", "1:25", "1:26", "unexpected character 'ı'"},
		{"non-ascii at start spans one column", "şehir === 1", "1:1", "1:2", "unexpected character 'ş'"},
		{"invalid utf-8", "a === \xff", "1:7", "1:8", "invalid UTF-8"},
		{"invalid utf-8 in string", "a === 'x\xffy'", "1:9", "1:10", "invalid UTF-8"},
		{"no-break space is not whitespace", "a === 1", "1:2", "1:3", "unexpected character"},
		{"error on second line", "a === 1 &&\n  b === 'x", "2:9", "2:11", "unterminated string"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			toks := lex(tt.src)
			last := toks[len(toks)-1]
			if last.kind != tkError {
				t.Fatalf("lex(%q) did not end in an error: %+v", tt.src, last)
			}
			e := last.err
			if e.Code != CodeSyntaxError {
				t.Errorf("code = %s, want %s", e.Code, CodeSyntaxError)
			}
			if got := fmtPos(e.Start); got != tt.from {
				t.Errorf("start = %s, want %s", got, tt.from)
			}
			if got := fmtPos(e.End); got != tt.to {
				t.Errorf("end = %s, want %s", got, tt.to)
			}
			if !contains(e.Message, tt.msg) {
				t.Errorf("message = %q, want it to contain %q", e.Message, tt.msg)
			}
		})
	}
}
