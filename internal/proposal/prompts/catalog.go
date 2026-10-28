package prompts

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/furknozg/coldbru/internal/proposal/catalog"
	"github.com/furknozg/coldbru/internal/proposal/dsl"
)

// RenderCatalog writes the filter catalog as prompt text (D-060, D-066): entities, field names,
// kinds, operators, descriptions and closed values. The output depends only on root, with no maps
// or clocks, so the same catalog renders to the same bytes.
func RenderCatalog(root *catalog.Entity) string {
	var b strings.Builder
	b.WriteString("CATALOG\n")
	b.WriteString("Only the names below exist. Operators are listed per field; in is membership in a literal list.\n\n")
	fmt.Fprintf(&b, "Root entity: %s. %s\n", root.Name, root.Description)
	b.WriteString("Customer fields, written by name alone:\n")
	writeFields(&b, root.Fields)
	for _, r := range root.Relations {
		fmt.Fprintf(&b, "\nRelation %s: %s\n", r.Name, r.Description)
		fmt.Fprintf(&b, "Quantifiers: %s.\n", joinQuantifiers(r.Quantifiers))
		fmt.Fprintf(&b, "Fields of one record of %s, always written with the lambda variable (for example x.%s):\n", r.Name, firstField(r.Entity))
		writeFields(&b, r.Entity.Fields)
	}
	return strings.TrimRight(b.String(), "\n")
}

func writeFields(b *strings.Builder, fields []catalog.Field) {
	for _, f := range fields {
		ops := make([]string, len(f.Ops))
		for i, o := range f.Ops {
			ops[i] = string(o)
		}
		fmt.Fprintf(b, "- %s (%s; %s): %s", f.Name, f.Kind, strings.Join(ops, ", "), f.Description)
		if len(f.Closed) > 0 {
			quoted := make([]string, len(f.Closed))
			for i, c := range f.Closed {
				quoted[i] = strconv.Quote(c)
			}
			fmt.Fprintf(b, " Allowed values: %s.", strings.Join(quoted, ", "))
		}
		b.WriteString("\n")
	}
}

func joinQuantifiers(qs []dsl.Quantifier) string {
	out := make([]string, len(qs))
	for i, q := range qs {
		out[i] = string(q)
	}
	return strings.Join(out, ", ")
}

func firstField(e *catalog.Entity) string {
	if len(e.Fields) == 0 {
		return "field"
	}
	return e.Fields[0].Name
}
