package dsl

import "slices"

// Span is the source range of a node. Embedded in every node, so Start and
// End are promoted fields.
type Span struct {
	Start Pos
	End   Pos
}

// Extent returns the span itself; it is promoted to every node.
func (s Span) Extent() Span { return s }

func (s *Span) setSpan(sp Span) { *s = sp }

// Expr is a boolean condition node. Implementations are pointers: *And, *Or,
// *Not, *Compare, *In and *Relation.
type Expr interface {
	Extent() Span
	setSpan(Span)
	expr()
}

// And is a conjunction. Chains are flattened: a && b && c is one And with
// three terms, also when written with parentheses.
type And struct {
	Span
	Terms []Expr
}

// Or is a disjunction, flattened like And.
type Or struct {
	Span
	Terms []Expr
}

// Not is the logical negation of X.
type Not struct {
	Span
	X Expr
}

// Op is a comparison operator, spelled as in the DSL.
type Op string

// Comparison operators of DSL 0.1 (docs/coldbru-gqdsl.md §3.2).
const (
	OpEq  Op = "==="
	OpNe  Op = "!=="
	OpGt  Op = ">"
	OpGte Op = ">="
	OpLt  Op = "<"
	OpLte Op = "<="
)

// Path is a dotted reference such as city or o.totalGbp. Inside a lambda body
// the first segment is the lambda variable, as written.
type Path struct {
	Span
	Segments []string
}

// Compare is `Field Op Value`. The field is always on the left and the value
// is a literal (field-to-field is reserved, D-045).
type Compare struct {
	Span
	Field Path
	Op    Op
	Value Literal
}

// In is `Field in [Values...]` with 1 to MaxInValues literals.
type In struct {
	Span
	Field  Path
	Values []Literal
}

// Quantifier is a relation quantifier.
type Quantifier string

// Relation quantifiers (docs/coldbru-gqdsl.md §5).
const (
	Some  Quantifier = "some"
	None  Quantifier = "none"
	Every Quantifier = "every"
)

// Relation is `Path.Quantifier(Var => Body)`. Whether Path names a relation
// is decided by the semantic pass.
type Relation struct {
	Span
	Path       Path
	Quantifier Quantifier
	Var        string
	VarSpan    Span
	Body       Expr
}

func (*And) expr()      {}
func (*Or) expr()       {}
func (*Not) expr()      {}
func (*Compare) expr()  {}
func (*In) expr()       {}
func (*Relation) expr() {}

// Literal is a right-hand-side value. Implementations are plain values.
type Literal interface {
	Extent() Span
	literal()
}

// String is a string literal; Value is the decoded text.
type String struct {
	Span
	Value string
}

// Number is a number literal kept as its source text (never a float, D-065).
type Number struct {
	Span
	Text string
}

// Bool is true or false.
type Bool struct {
	Span
	Value bool
}

// Ago is ago("30d"); Text is the validated argument, e.g. 30d (D-039).
type Ago struct {
	Span
	Text string
}

// Date is date("2026-01-01"); Text is the validated argument.
type Date struct {
	Span
	Text string
}

func (String) literal() {}
func (Number) literal() {}
func (Bool) literal()   {}
func (Ago) literal()    {}
func (Date) literal()   {}

// Equal reports whether a and b are the same tree, ignoring spans.
func Equal(a, b Expr) bool {
	switch x := a.(type) {
	case *And:
		y, ok := b.(*And)
		if !ok || x == nil || y == nil {
			return ok && x == y
		}
		return equalExprs(x.Terms, y.Terms)
	case *Or:
		y, ok := b.(*Or)
		if !ok || x == nil || y == nil {
			return ok && x == y
		}
		return equalExprs(x.Terms, y.Terms)
	case *Not:
		y, ok := b.(*Not)
		if !ok || x == nil || y == nil {
			return ok && x == y
		}
		return Equal(x.X, y.X)
	case *Compare:
		y, ok := b.(*Compare)
		if !ok || x == nil || y == nil {
			return ok && x == y
		}
		return equalPath(x.Field, y.Field) && x.Op == y.Op && equalLiteral(x.Value, y.Value)
	case *In:
		y, ok := b.(*In)
		if !ok || x == nil || y == nil {
			return ok && x == y
		}
		return equalPath(x.Field, y.Field) && slices.EqualFunc(x.Values, y.Values, equalLiteral)
	case *Relation:
		y, ok := b.(*Relation)
		if !ok || x == nil || y == nil {
			return ok && x == y
		}
		return equalPath(x.Path, y.Path) && x.Quantifier == y.Quantifier && x.Var == y.Var && Equal(x.Body, y.Body)
	case nil:
		return b == nil
	}
	return false
}

func equalExprs(a, b []Expr) bool { return slices.EqualFunc(a, b, Equal) }

func equalPath(a, b Path) bool { return slices.Equal(a.Segments, b.Segments) }

func equalLiteral(a, b Literal) bool {
	switch x := a.(type) {
	case String:
		y, ok := b.(String)
		return ok && x.Value == y.Value
	case Number:
		y, ok := b.(Number)
		return ok && x.Text == y.Text
	case Bool:
		y, ok := b.(Bool)
		return ok && x.Value == y.Value
	case Ago:
		y, ok := b.(Ago)
		return ok && x.Text == y.Text
	case Date:
		y, ok := b.(Date)
		return ok && x.Text == y.Text
	case nil:
		return b == nil
	}
	return false
}
