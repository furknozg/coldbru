// Package catalog is Proposal's view of what a segment filter may mention: the entities, fields,
// operators and relations of the Customer GraphQL contract, with prompt descriptions and the
// closed value lists (D-044, D-066).
//
// The data is generated: catalog_gen.go comes from internal/customer/graph/schema.graphqls and
// metadata.yaml (`make generate-catalog`). This file holds the types and lookups, check.go the
// semantic pass over a parsed DSL expression.
package catalog

import "github.com/furknozg/coldbru/internal/proposal/dsl"

// OpIn is the membership operator, spelled as in the DSL. The comparison operators are the dsl.Op
// constants; the parser has no Op for `in` because it builds a separate dsl.In node.
const OpIn dsl.Op = "in"

// Kind is the value type of a field, taken from the schema's filter input (StringFilter, ...).
type Kind int

// Field kinds.
const (
	KindString Kind = iota + 1
	KindInt
	KindDecimal
	KindDate
	KindDateTime
	KindBool
)

// String returns the name used in error messages and prompts.
func (k Kind) String() string {
	switch k {
	case KindString:
		return "String"
	case KindInt:
		return "Int"
	case KindDecimal:
		return "Decimal"
	case KindDate:
		return "Date"
	case KindDateTime:
		return "DateTime"
	case KindBool:
		return "Bool"
	}
	return "Unknown"
}

// Field is one filterable field.
type Field struct {
	Name string
	Kind Kind
	// Ops are the operators the schema's filter input offers, in schema order. They include
	// OpIn when the filter has an `in` field. The semantic pass may narrow them further
	// (ago() allows only ordering operators).
	Ops []dsl.Op
	// Closed lists the allowed values of a String field with a CHECK constraint (metadata.yaml).
	// nil means any text; Customer validates city separately (D-062).
	Closed      []string
	Description string
}

// Allows reports whether op is one of the field's operators.
func (f *Field) Allows(op dsl.Op) bool {
	for _, o := range f.Ops {
		if o == op {
			return true
		}
	}
	return false
}

// Relation is a one-to-many child of an entity (orders, cartItems, events).
type Relation struct {
	Name string
	// Quantifiers are the quantifier fields of the schema's relation filter, in schema order.
	Quantifiers []dsl.Quantifier
	// Entity is the child; its Name equals the relation Name.
	Entity      *Entity
	Description string
}

// Allows reports whether q is one of the relation's quantifiers.
func (r *Relation) Allows(q dsl.Quantifier) bool {
	for _, x := range r.Quantifiers {
		if x == q {
			return true
		}
	}
	return false
}

// Entity is a filterable record type: the root customer or a relation's child.
type Entity struct {
	// Name is "customer" for the root, the relation name for a child.
	Name        string
	Description string
	Fields      []Field
	Relations   []Relation
}

// Field returns the field called name, or nil.
func (e *Entity) Field(name string) *Field {
	for i := range e.Fields {
		if e.Fields[i].Name == name {
			return &e.Fields[i]
		}
	}
	return nil
}

// Relation returns the relation called name, or nil.
func (e *Entity) Relation(name string) *Relation {
	for i := range e.Relations {
		if e.Relations[i].Name == name {
			return &e.Relations[i]
		}
	}
	return nil
}
