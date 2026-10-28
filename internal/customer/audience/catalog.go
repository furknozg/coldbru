// Package audience turns a Customer GraphQL filter into parameterized SQL over
// the dwh star schema and runs the audience queries (D-007, D-062).
//
// The filter is never executed as text: the catalog in this file is the only
// source of table and column names that reach SQL, and every value travels as
// a $n argument. Reachability (consent, bounce, account status) is added by
// this package on every audience query and cannot be filtered on.
package audience

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/furknozg/coldbru/internal/customer/graph/model"
)

// Kind is the value type of a catalog field.
type Kind int

// Field kinds. Date and DateTime take TimeValue operands, the others a plain
// Go value (string, int, model.Decimal, bool).
const (
	KindString Kind = iota + 1
	KindInt
	KindDecimal
	KindDate
	KindDateTime
	KindBool
)

// Field maps one GraphQL filter field to a column.
type Field struct {
	// Name is the GraphQL field name (camelCase, D-044); it equals the json tag
	// of the model struct field.
	Name string
	// Column is the dwh column name, never user-controlled.
	Column string
	Kind   Kind
	// Closed lists the allowed values of a String field whose column has a CHECK
	// constraint (D-062). They must be kept in sync with 02_customer_schema.sql;
	// nil means any text is accepted.
	Closed []string
	// City marks the one field validated against the in-memory city dictionary
	// and canonicalized before it reaches SQL (D-040, D-062).
	City bool

	index int // index of the *XxxFilter field in the model struct
}

// Relation is a one-to-many child of dim_customer (orders, cartItems, events).
type Relation struct {
	// Name is the GraphQL field name on CustomerFilter.
	Name   string
	Entity *Entity

	index int // index of the *XxxRelationFilter field in CustomerFilter
}

// Entity is a table the filter can address.
type Entity struct {
	// Table is the schema-qualified table name.
	Table string
	// Alias is the fixed SQL alias; the root is "c", children "o", "ci", "e".
	Alias string
	// Fields are the filterable columns in schema order.
	Fields []Field
	// Relations are the child entities (root only).
	Relations []Relation

	modelType reflect.Type
}

// Qualified returns alias.column for a column of this entity.
func (e *Entity) Qualified(column string) string { return e.Alias + "." + column }

// Closed value lists, copied from the CHECK constraints of
// infra/postgres/initdb/sql/02_customer_schema.sql (D-062). TestClosedListsMatchSchema
// (integration) compares them with the live constraints.
var (
	sexValues         = []string{"Male", "Female"}
	languageValues    = []string{"tr", "en"}
	countryValues     = []string{"TR", "GB"}
	platformValues    = []string{"ios", "android", "web"}
	loyaltyValues     = []string{"Bronze", "Silver", "Gold", "Platinum"}
	orderStatusValues = []string{"shipped", "delivered", "cancelled", "returned", "refunded"}
	cartStatusValues  = []string{"open", "abandoned", "converted", "removed"}
	eventTypeValues   = []string{
		"app_open", "page_view", "search", "product_view", "wishlist_add",
		"add_to_cart", "remove_from_cart", "checkout_started", "purchase",
	}
)

// customerEntity, orderEntity, cartItemEntity and eventEntity are the whole
// reachable SQL surface (D-042). Personal, consent and account columns are not
// listed, so no filter can name them.
var (
	orderEntity = mustEntity(&Entity{
		Table: "dwh.fact_order", Alias: "o", modelType: reflect.TypeFor[model.OrderFilter](),
		Fields: []Field{
			{Name: "orderDate", Column: "order_date", Kind: KindDateTime},
			{Name: "orderStatus", Column: "order_status", Kind: KindString, Closed: orderStatusValues},
			{Name: "itemsCount", Column: "items_count", Kind: KindInt},
			{Name: "subtotalGbp", Column: "subtotal_gbp", Kind: KindDecimal},
			{Name: "discountCode", Column: "discount_code", Kind: KindString},
			{Name: "discountGbp", Column: "discount_gbp", Kind: KindDecimal},
			{Name: "shippingGbp", Column: "shipping_gbp", Kind: KindDecimal},
			{Name: "totalGbp", Column: "total_gbp", Kind: KindDecimal},
			{Name: "paymentMethod", Column: "payment_method", Kind: KindString},
			{Name: "platform", Column: "platform", Kind: KindString},
			{Name: "shippingCity", Column: "shipping_city", Kind: KindString},
			{Name: "countryCode", Column: "country_code", Kind: KindString},
		},
	})

	cartItemEntity = mustEntity(&Entity{
		Table: "dwh.fact_cart_item", Alias: "ci", modelType: reflect.TypeFor[model.CartItemFilter](),
		Fields: []Field{
			{Name: "addedAt", Column: "added_at", Kind: KindDateTime},
			{Name: "cartStatus", Column: "cart_status", Kind: KindString, Closed: cartStatusValues},
			{Name: "quantity", Column: "quantity", Kind: KindInt},
			{Name: "unitPriceGbp", Column: "unit_price_gbp", Kind: KindDecimal},
			{Name: "convertedAt", Column: "converted_at", Kind: KindDateTime},
		},
	})

	eventEntity = mustEntity(&Entity{
		Table: "dwh.fact_event", Alias: "e", modelType: reflect.TypeFor[model.EventFilter](),
		Fields: []Field{
			{Name: "eventType", Column: "event_type", Kind: KindString, Closed: eventTypeValues},
			{Name: "eventTime", Column: "event_time", Kind: KindDateTime},
			{Name: "platform", Column: "platform", Kind: KindString},
			{Name: "category", Column: "category", Kind: KindString},
			{Name: "valueGbp", Column: "value_gbp", Kind: KindDecimal},
		},
	})

	customerEntity = mustEntity(&Entity{
		Table: "dwh.dim_customer", Alias: "c", modelType: reflect.TypeFor[model.CustomerFilter](),
		Fields: []Field{
			{Name: "countryCode", Column: "country_code", Kind: KindString, Closed: countryValues},
			{Name: "city", Column: "city", Kind: KindString, City: true},
			{Name: "language", Column: "language", Kind: KindString, Closed: languageValues},
			{Name: "timezone", Column: "timezone", Kind: KindString},
			{Name: "signupDate", Column: "signup_date", Kind: KindDate},
			{Name: "lastActiveDate", Column: "last_active_date", Kind: KindDate},
			{Name: "preferredPlatform", Column: "preferred_platform", Kind: KindString, Closed: platformValues},
			{Name: "sessions90d", Column: "sessions_90d", Kind: KindInt},
			{Name: "totalOrders", Column: "total_orders", Kind: KindInt},
			{Name: "totalSpendGbp", Column: "total_spend_gbp", Kind: KindDecimal},
			{Name: "avgOrderValueGbp", Column: "avg_order_value_gbp", Kind: KindDecimal},
			{Name: "firstOrderDate", Column: "first_order_date", Kind: KindDate},
			{Name: "lastOrderDate", Column: "last_order_date", Kind: KindDate},
			{Name: "favouriteCategory", Column: "favourite_category", Kind: KindString},
			{Name: "lastCartAddDate", Column: "last_cart_add_date", Kind: KindDate},
			{Name: "hasOpenCart", Column: "has_open_cart", Kind: KindBool},
			{Name: "openCartValueGbp", Column: "open_cart_value_gbp", Kind: KindDecimal},
			{Name: "loyaltyTier", Column: "loyalty_tier", Kind: KindString, Closed: loyaltyValues},
			{Name: "loyaltyPoints", Column: "loyalty_points", Kind: KindInt},
			{Name: "campaignsReceived90d", Column: "campaigns_received_90d", Kind: KindInt},
			{Name: "emailsOpened90d", Column: "emails_opened_90d", Kind: KindInt},
			{Name: "lastCampaignSentAt", Column: "last_campaign_sent_at", Kind: KindDateTime},
			{Name: "ageYears", Column: "age_years", Kind: KindInt},
			{Name: "sex", Column: "sex", Kind: KindString, Closed: sexValues},
			{Name: "maritalStatus", Column: "marital_status", Kind: KindString},
			{Name: "dependants", Column: "dependants", Kind: KindInt},
			{Name: "yearlySalaryGbp", Column: "yearly_salary_gbp", Kind: KindInt},
			{Name: "yearlyPensionGbp", Column: "yearly_pension_gbp", Kind: KindInt},
			{Name: "retired", Column: "retired", Kind: KindBool},
			{Name: "commuteDistanceKm", Column: "commute_distance_km", Kind: KindDecimal},
			{Name: "vehicleMake", Column: "vehicle_make", Kind: KindString},
			{Name: "vehicleModel", Column: "vehicle_model", Kind: KindString},
			{Name: "vehicleYear", Column: "vehicle_year", Kind: KindInt},
			{Name: "vehicleType", Column: "vehicle_type", Kind: KindString},
		},
		Relations: []Relation{
			{Name: "orders", Entity: orderEntity},
			{Name: "cartItems", Entity: cartItemEntity},
			{Name: "events", Entity: eventEntity},
		},
	})
)

// Key columns used outside the filterable catalog (never reachable from a filter).
const (
	// childKey is the foreign key of every child table to dim_customer.
	childKey = "customer_id"
	// rootKey is the primary key of dim_customer (keyset cursor, correlation).
	rootKey = "c.customer_id"
)

// mustEntity binds the catalog to the model struct by json tag. A field in the
// catalog that is missing from the struct panics at package init: that is a
// programming error, and TestCatalogMatchesModel lists every drift.
func mustEntity(e *Entity) *Entity {
	index := jsonIndex(e.modelType)
	for i := range e.Fields {
		idx, ok := index[e.Fields[i].Name]
		if !ok {
			panic(fmt.Sprintf("audience catalog: %s has no filter field %q", e.modelType.Name(), e.Fields[i].Name))
		}
		e.Fields[i].index = idx
	}
	for i := range e.Relations {
		idx, ok := index[e.Relations[i].Name]
		if !ok {
			panic(fmt.Sprintf("audience catalog: %s has no relation field %q", e.modelType.Name(), e.Relations[i].Name))
		}
		e.Relations[i].index = idx
	}
	return e
}

// jsonIndex maps the json tag name of each struct field to its index.
func jsonIndex(t reflect.Type) map[string]int {
	out := make(map[string]int, t.NumField())
	for i := range t.NumField() {
		tag := t.Field(i).Tag.Get("json")
		if name, _, _ := strings.Cut(tag, ","); name != "" {
			out[name] = i
		}
	}
	return out
}

// ClosedLists returns the CHECK-constrained value lists of the catalog, keyed by
// entity name ("customer", "orders", "cartItems", "events") and then by field
// name. Only fields with a closed list appear. The slices are copies, so callers
// cannot change the catalog. Proposal's metadata is compared with it by a test
// (D-066).
func ClosedLists() map[string]map[string][]string {
	out := make(map[string]map[string][]string)
	add := func(name string, e *Entity) {
		for _, f := range e.Fields {
			if f.Closed == nil {
				continue
			}
			if out[name] == nil {
				out[name] = make(map[string][]string)
			}
			out[name][f.Name] = slices.Clone(f.Closed)
		}
	}
	add("customer", customerEntity)
	for _, r := range customerEntity.Relations {
		add(r.Name, r.Entity)
	}
	return out
}
