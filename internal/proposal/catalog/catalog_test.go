package catalog

import (
	"reflect"
	"slices"
	"testing"

	"github.com/furknozg/coldbru/internal/customer/audience"
	"github.com/furknozg/coldbru/internal/proposal/dsl"
)

// TestClosedListsMatchCustomer compares the closed lists of the generated catalog with Customer's
// CHECK-constrained lists, entity by entity and field by field (D-066). A list present on one side
// only fails too.
func TestClosedListsMatchCustomer(t *testing.T) {
	t.Parallel()
	got := map[string]map[string][]string{}
	walk := func(e *Entity) {
		for _, f := range e.Fields {
			if f.Closed == nil {
				continue
			}
			if got[e.Name] == nil {
				got[e.Name] = map[string][]string{}
			}
			got[e.Name][f.Name] = f.Closed
		}
	}
	walk(Root)
	for _, r := range Root.Relations {
		walk(r.Entity)
	}
	want := audience.ClosedLists()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("closed lists differ from Customer\n catalog:  %v\n customer: %v", got, want)
	}
}

// TestRootShape pins the structure the T-020 compiler relies on.
func TestRootShape(t *testing.T) {
	t.Parallel()
	if Root.Name != "customer" || Root.Description == "" {
		t.Errorf("Root = %q / %q", Root.Name, Root.Description)
	}
	relNames := []string{"orders", "cartItems", "events"}
	var got []string
	for _, r := range Root.Relations {
		got = append(got, r.Name)
		if r.Entity.Name != r.Name || r.Description == "" || r.Entity.Description == "" {
			t.Errorf("relation %s: entity %q, descriptions %q / %q", r.Name, r.Entity.Name, r.Description, r.Entity.Description)
		}
		if !slices.Equal(r.Quantifiers, []dsl.Quantifier{dsl.Some, dsl.None, dsl.Every}) {
			t.Errorf("relation %s quantifiers = %v", r.Name, r.Quantifiers)
		}
		if len(r.Entity.Relations) != 0 {
			t.Errorf("relation %s nests relations: the catalog is one level", r.Name)
		}
	}
	if !slices.Equal(got, relNames) {
		t.Errorf("relations = %v, want %v", got, relNames)
	}
	for _, e := range append([]*Entity{Root}, relEntities()...) {
		for _, f := range e.Fields {
			if f.Description == "" || f.Kind == 0 || len(f.Ops) == 0 {
				t.Errorf("%s.%s incomplete: %+v", e.Name, f.Name, f)
			}
		}
	}
}

func relEntities() []*Entity {
	var out []*Entity
	for _, r := range Root.Relations {
		out = append(out, r.Entity)
	}
	return out
}

// TestDerivedOps checks operators come from the schema filter inputs (D-066), per kind.
func TestDerivedOps(t *testing.T) {
	t.Parallel()
	ordering := []dsl.Op{dsl.OpEq, dsl.OpNe, dsl.OpGt, dsl.OpGte, dsl.OpLt, dsl.OpLte, OpIn}
	want := map[Kind][]dsl.Op{
		KindString:   {dsl.OpEq, dsl.OpNe, OpIn},
		KindBool:     {dsl.OpEq, dsl.OpNe},
		KindInt:      ordering,
		KindDecimal:  ordering,
		KindDate:     ordering,
		KindDateTime: ordering,
	}
	for _, e := range append([]*Entity{Root}, relEntities()...) {
		for _, f := range e.Fields {
			if !slices.Equal(f.Ops, want[f.Kind]) {
				t.Errorf("%s.%s (%s) ops = %v, want %v", e.Name, f.Name, f.Kind, f.Ops, want[f.Kind])
			}
		}
	}
}

func TestKinds(t *testing.T) {
	t.Parallel()
	want := map[string]Kind{
		"city": KindString, "ageYears": KindInt, "totalSpendGbp": KindDecimal, "signupDate": KindDate,
		"lastCampaignSentAt": KindDateTime, "hasOpenCart": KindBool,
	}
	for name, k := range want {
		if f := Root.Field(name); f == nil || f.Kind != k {
			t.Errorf("%s = %v, want kind %s", name, f, k)
		}
	}
	if Kind(0).String() != "Unknown" {
		t.Errorf("zero Kind = %q", Kind(0))
	}
}

func TestLookups(t *testing.T) {
	t.Parallel()
	if Root.Field("city") == nil || Root.Field("orders") != nil || Root.Field("nope") != nil {
		t.Error("Field lookup wrong")
	}
	if Root.Relation("orders") == nil || Root.Relation("city") != nil {
		t.Error("Relation lookup wrong")
	}
	f := Root.Field("loyaltyTier")
	if !f.Allows(OpIn) || f.Allows(dsl.OpGt) {
		t.Errorf("Allows on %v", f.Ops)
	}
	r := Root.Relation("orders")
	if !r.Allows(dsl.Every) || r.Allows(dsl.Quantifier("most")) {
		t.Errorf("Allows on %v", r.Quantifiers)
	}
}

// TestNoPersonalFields keeps personal, consent and account fields out of the catalog (D-042).
func TestNoPersonalFields(t *testing.T) {
	t.Parallel()
	banned := []string{
		"firstName", "lastName", "email", "phoneE164", "addressStreet", "addressCity", "addressPostcode",
		"dateOfBirth", "employerCompany", "emailOptIn", "smsOptIn", "emailBounced", "accountStatus",
		"deletedAt", "consentUpdatedAt",
	}
	for _, name := range banned {
		if Root.Field(name) != nil {
			t.Errorf("catalog contains %s", name)
		}
	}
}
