// Command gen writes internal/proposal/catalog/catalog_gen.go from the Customer GraphQL schema and
// the Proposal metadata file (D-044, D-066). Run it from the repository root with `make
// generate-catalog`; a freshness test fails when the committed output is stale.
//
// Fields, filter operators and relations are derived from `input CustomerFilter`; metadata.yaml
// only adds prompt descriptions and the closed value lists. Any disagreement between the two is a
// generator error, so schema drift fails the build instead of reaching the prompt.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
	"go.yaml.in/yaml/v3"
)

func main() {
	schemaPath := flag.String("schema", "internal/customer/graph/schema.graphqls", "Customer GraphQL schema")
	metaPath := flag.String("meta", "internal/proposal/catalog/metadata.yaml", "catalog metadata (descriptions, closed lists)")
	outPath := flag.String("out", "internal/proposal/catalog/catalog_gen.go", "generated Go file")
	flag.Parse()

	if err := run(*schemaPath, *metaPath, *outPath); err != nil {
		fmt.Fprintln(os.Stderr, "catalog gen:", err)
		os.Exit(1)
	}
}

func run(schemaPath, metaPath, outPath string) error {
	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return err
	}
	meta, err := os.ReadFile(metaPath)
	if err != nil {
		return err
	}
	out, err := generate(schema, meta)
	if err != nil {
		return err
	}
	return os.WriteFile(outPath, out, 0o644)
}

// rootFilter is the input type the catalog starts from; rootName is its entity key in the metadata.
const (
	rootFilter = "CustomerFilter"
	rootName   = "customer"
)

// metadata is the YAML file. Entity keys are rootName and the relation names of the root.
type metadata struct {
	Entities map[string]metaEntity `yaml:"entities"`
}

type metaEntity struct {
	Description string                  `yaml:"description"`
	Fields      map[string]metaField    `yaml:"fields"`
	Relations   map[string]metaRelation `yaml:"relations"`
}

type metaField struct {
	Description string   `yaml:"description"`
	Closed      []string `yaml:"closed"`
}

type metaRelation struct {
	Description string `yaml:"description"`
}

// filterKinds maps a scalar filter input to the Kind constant of the catalog.
var filterKinds = map[string]string{
	"StringFilter":   "KindString",
	"IntFilter":      "KindInt",
	"DecimalFilter":  "KindDecimal",
	"DateFilter":     "KindDate",
	"DateTimeFilter": "KindDateTime",
	"BooleanFilter":  "KindBool",
}

// opNames maps a filter input field to the DSL operator constant.
var opNames = map[string]string{
	"eq": "dsl.OpEq", "ne": "dsl.OpNe", "gt": "dsl.OpGt", "gte": "dsl.OpGte",
	"lt": "dsl.OpLt", "lte": "dsl.OpLte", "in": "OpIn",
}

// quantifierNames maps a relation filter input field to the DSL quantifier constant.
var quantifierNames = map[string]string{"some": "dsl.Some", "none": "dsl.None", "every": "dsl.Every"}

// composites are the logical fields of every filter input; the DSL has its own && || !.
var composites = []string{"and", "or", "not"}

type genEntity struct {
	name, description string
	fields            []genField
	relations         []genRelation
}

type genField struct {
	name, kind, description string
	ops                     []string
	closed                  []string
}

type genRelation struct {
	name, description string
	quantifiers       []string
	entity            *genEntity
}

// generate returns the gofmt'ed catalog source for a schema and a metadata file. All drift
// problems are reported together.
func generate(schema, meta []byte) ([]byte, error) {
	var md metadata
	dec := yaml.NewDecoder(bytes.NewReader(meta))
	dec.KnownFields(true)
	if err := dec.Decode(&md); err != nil {
		return nil, fmt.Errorf("metadata: %w", err)
	}
	doc, perr := parser.ParseSchema(&ast.Source{Name: "schema.graphqls", Input: string(schema)})
	if perr != nil {
		return nil, fmt.Errorf("schema: %w", perr)
	}
	b := &builder{md: md, defs: map[string]*ast.Definition{}, used: map[string]bool{}}
	for _, d := range doc.Definitions {
		if d.Kind == ast.InputObject {
			b.defs[d.Name] = d
		}
	}
	root := b.defs[rootFilter]
	if root == nil {
		return nil, fmt.Errorf("schema: no input %s", rootFilter)
	}
	ent := b.entity(rootName, root, []string{rootFilter})
	for _, key := range sortedKeys(md.Entities) {
		if !b.used[key] {
			b.problem("metadata entity %q has no counterpart in the schema", key)
		}
	}
	if len(b.problems) > 0 {
		return nil, errors.New("catalog drift:\n  " + strings.Join(b.problems, "\n  "))
	}
	return render(ent)
}

type builder struct {
	md       metadata
	defs     map[string]*ast.Definition
	used     map[string]bool
	problems []string
}

func (b *builder) problem(format string, args ...any) {
	b.problems = append(b.problems, fmt.Sprintf(format, args...))
}

// entity derives one entity from a filter input. key is the metadata entity key, stack the filter
// inputs being expanded (a cycle would never terminate).
func (b *builder) entity(key string, def *ast.Definition, stack []string) *genEntity {
	me, ok := b.md.Entities[key]
	b.used[key] = true
	if !ok {
		b.problem("schema entity %q (input %s) has no metadata entry", key, def.Name)
	} else if clean(me.Description) == "" {
		b.problem("entity %q has no description", key)
	}
	ent := &genEntity{name: key, description: clean(me.Description)}

	fieldsSeen := map[string]bool{}
	relsSeen := map[string]bool{}
	for _, f := range def.Fields {
		if slices.Contains(composites, f.Name) {
			continue
		}
		if f.Type.Elem != nil {
			b.problem("%s.%s: list-typed filter fields are not supported", key, f.Name)
			continue
		}
		typeName := f.Type.NamedType
		if kind, ok := filterKinds[typeName]; ok {
			fieldsSeen[f.Name] = true
			ent.fields = append(ent.fields, b.field(key, f.Name, kind, typeName, me))
			continue
		}
		if strings.HasSuffix(typeName, "RelationFilter") {
			relsSeen[f.Name] = true
			if r := b.relation(key, f.Name, typeName, me, stack); r != nil {
				ent.relations = append(ent.relations, *r)
			}
			continue
		}
		b.problem("%s.%s: unknown filter type %q", key, f.Name, typeName)
	}
	for _, name := range sortedKeys(me.Fields) {
		if !fieldsSeen[name] {
			b.problem("%s: metadata field %q has no counterpart in the schema", key, name)
		}
	}
	for _, name := range sortedKeys(me.Relations) {
		if !relsSeen[name] {
			b.problem("%s: metadata relation %q has no counterpart in the schema", key, name)
		}
	}
	return ent
}

func (b *builder) field(entity, name, kind, filterType string, me metaEntity) genField {
	f := genField{name: name, kind: kind}
	for _, of := range b.defs[filterType].Fields {
		op, ok := opNames[of.Name]
		if !ok {
			b.problem("%s.%s: operator %q of %s is not supported", entity, name, of.Name, filterType)
			continue
		}
		f.ops = append(f.ops, op)
	}
	mf, ok := me.Fields[name]
	f.description = clean(mf.Description)
	if !ok || f.description == "" {
		b.problem("%s.%s has no description", entity, name)
	}
	if mf.Closed != nil {
		switch {
		case kind != "KindString":
			b.problem("%s.%s: a closed list needs a String field, not %s", entity, name, strings.TrimPrefix(kind, "Kind"))
		case len(mf.Closed) == 0:
			b.problem("%s.%s: closed list is empty", entity, name)
		default:
			seen := map[string]bool{}
			for _, v := range mf.Closed {
				if seen[v] {
					b.problem("%s.%s: closed list repeats %q", entity, name, v)
				}
				seen[v] = true
			}
			f.closed = slices.Clone(mf.Closed)
		}
	}
	return f
}

func (b *builder) relation(entity, name, filterType string, me metaEntity, stack []string) *genRelation {
	rdef := b.defs[filterType]
	if rdef == nil {
		b.problem("%s.%s: relation filter %q is not defined", entity, name, filterType)
		return nil
	}
	r := &genRelation{name: name}
	mr, ok := me.Relations[name]
	r.description = clean(mr.Description)
	if !ok || r.description == "" {
		b.problem("%s.%s (relation) has no description", entity, name)
	}
	child := ""
	for _, qf := range rdef.Fields {
		q, ok := quantifierNames[qf.Name]
		if !ok {
			b.problem("%s.%s: quantifier %q of %s is not supported", entity, name, qf.Name, filterType)
			continue
		}
		r.quantifiers = append(r.quantifiers, q)
		if qf.Type.Elem != nil || (child != "" && child != qf.Type.NamedType) {
			b.problem("%s.%s: quantifiers of %s must all take one filter input", entity, name, filterType)
			continue
		}
		child = qf.Type.NamedType
	}
	cdef := b.defs[child]
	switch {
	case child == "" || cdef == nil:
		b.problem("%s.%s: relation filter %s names no known child filter (%q)", entity, name, filterType, child)
		return nil
	case slices.Contains(stack, child):
		b.problem("%s.%s: filter %s is its own ancestor", entity, name, child)
		return nil
	}
	r.entity = b.entity(name, cdef, append(slices.Clone(stack), child))
	return r
}

// clean flattens a YAML text into one trimmed line.
func clean(s string) string { return strings.Join(strings.Fields(s), " ") }

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

const header = `// Code generated by catalog/gen. DO NOT EDIT.

package catalog

import "github.com/furknozg/coldbru/internal/proposal/dsl"

`

func render(root *genEntity) ([]byte, error) {
	var w bytes.Buffer
	w.WriteString(header)
	w.WriteString("// Root is the customer entity: the whole filter catalog, derived from the Customer\n")
	w.WriteString("// GraphQL schema and metadata.yaml (D-044, D-066).\n")
	w.WriteString("var Root = ")
	writeEntity(&w, root)
	w.WriteString("\n")
	out, err := format.Source(w.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format generated code: %w\n%s", err, w.String())
	}
	return out, nil
}

func writeEntity(w *bytes.Buffer, e *genEntity) {
	fmt.Fprintf(w, "&Entity{\nName: %q,\nDescription: %q,\nFields: []Field{\n", e.name, e.description)
	for _, f := range e.fields {
		fmt.Fprintf(w, "{Name: %q, Kind: %s, Ops: []dsl.Op{%s}, ", f.name, f.kind, strings.Join(f.ops, ", "))
		if len(f.closed) > 0 {
			fmt.Fprintf(w, "Closed: []string{%s}, ", quoteAll(f.closed))
		}
		fmt.Fprintf(w, "Description: %q},\n", f.description)
	}
	w.WriteString("},\n")
	if len(e.relations) > 0 {
		w.WriteString("Relations: []Relation{\n")
		for _, r := range e.relations {
			fmt.Fprintf(w, "{\nName: %q,\nDescription: %q,\nQuantifiers: []dsl.Quantifier{%s},\nEntity: ", r.name, r.description, strings.Join(r.quantifiers, ", "))
			writeEntity(w, r.entity)
			w.WriteString(",\n},\n")
		}
		w.WriteString("},\n")
	}
	w.WriteString("}")
}

func quoteAll(ss []string) string {
	q := make([]string, len(ss))
	for i, s := range ss {
		q[i] = fmt.Sprintf("%q", s)
	}
	return strings.Join(q, ", ")
}
