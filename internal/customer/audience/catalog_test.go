package audience

import (
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/furknozg/coldbru/internal/customer/graph/model"
)

var identifier = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// wantFilterType is the model filter type each Kind must be bound to.
var wantFilterType = map[Kind]reflect.Type{
	KindString:   reflect.TypeFor[*model.StringFilter](),
	KindInt:      reflect.TypeFor[*model.IntFilter](),
	KindDecimal:  reflect.TypeFor[*model.DecimalFilter](),
	KindBool:     reflect.TypeFor[*model.BooleanFilter](),
	KindDate:     reflect.TypeFor[*model.DateFilter](),
	KindDateTime: reflect.TypeFor[*model.DateTimeFilter](),
}

// TestCatalogMatchesModel proves the catalog and the generated filter structs
// list the same fields: nothing in the schema is missing from the catalog, and
// nothing in the catalog is absent from the schema (D-042).
func TestCatalogMatchesModel(t *testing.T) {
	t.Parallel()
	entities := map[string]*Entity{
		"CustomerFilter": customerEntity, "OrderFilter": orderEntity,
		"CartItemFilter": cartItemEntity, "EventFilter": eventEntity,
	}
	for name, ent := range entities {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if ent.modelType.Name() != name {
				t.Fatalf("entity bound to %s, want %s", ent.modelType.Name(), name)
			}
			tags := jsonIndex(ent.modelType)
			want := map[string]bool{}
			for tag := range tags {
				if tag != "and" && tag != "or" && tag != "not" {
					want[tag] = true
				}
			}
			for _, f := range ent.Fields {
				if !want[f.Name] {
					t.Errorf("catalog field %q is not a field of %s", f.Name, name)
				}
				delete(want, f.Name)
				got := ent.modelType.Field(f.index).Type
				if got != wantFilterType[f.Kind] {
					t.Errorf("field %q: model type %v, kind %d expects %v", f.Name, got, f.Kind, wantFilterType[f.Kind])
				}
				if !identifier.MatchString(f.Column) {
					t.Errorf("field %q: column %q is not a plain identifier", f.Name, f.Column)
				}
				if (f.Closed != nil || f.City) && f.Kind != KindString {
					t.Errorf("field %q: closed list or city on a non-string field", f.Name)
				}
			}
			for _, r := range ent.Relations {
				if !want[r.Name] {
					t.Errorf("catalog relation %q is not a field of %s", r.Name, name)
				}
				delete(want, r.Name)
			}
			for left := range want {
				t.Errorf("%s.%s is in the schema but not in the catalog", name, left)
			}
		})
	}
}

func TestCatalogHasNoPersonalOrConsentColumns(t *testing.T) {
	t.Parallel()
	banned := []string{
		"first_name", "last_name", "email", "phone_e164", "date_of_birth", "employer_company",
		"address_street", "address_city", "address_postcode", "email_opt_in", "sms_opt_in",
		"email_bounced", "account_status", "deleted_at", "consent_updated_at", "customer_id",
	}
	for _, ent := range []*Entity{customerEntity, orderEntity, cartItemEntity, eventEntity} {
		for _, f := range ent.Fields {
			if slices.Contains(banned, f.Column) {
				t.Errorf("%s.%s must not be filterable", ent.Table, f.Column)
			}
		}
	}
}

func TestCatalogColumnsAreUnique(t *testing.T) {
	t.Parallel()
	for _, ent := range []*Entity{customerEntity, orderEntity, cartItemEntity, eventEntity} {
		seen := map[string]bool{}
		for _, f := range ent.Fields {
			if seen[f.Column] || seen[strings.ToLower(f.Name)] {
				t.Errorf("%s: duplicate column or field %q", ent.Table, f.Name)
			}
			seen[f.Column] = true
		}
	}
}

// TestClosedFieldsAreTheCheckedOnes pins which fields are validated (D-062).
func TestClosedFieldsAreTheCheckedOnes(t *testing.T) {
	t.Parallel()
	got := map[string]bool{}
	for _, ent := range []*Entity{customerEntity, orderEntity, cartItemEntity, eventEntity} {
		for _, f := range ent.Fields {
			if f.Closed != nil {
				got[ent.Alias+"."+f.Name] = true
			}
		}
	}
	want := []string{"c.sex", "c.language", "c.countryCode", "c.preferredPlatform", "c.loyaltyTier", "o.orderStatus", "ci.cartStatus", "e.eventType"}
	if len(got) != len(want) {
		t.Errorf("closed fields = %v, want exactly %v", got, want)
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("%s is not a closed-list field", w)
		}
	}
}
