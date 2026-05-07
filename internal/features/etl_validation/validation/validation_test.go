package validation_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/validation"
)

func newDS() *validation.Dataset { return validation.NewDataset() }

// --- fk_exists ---

func TestFkExistsRule(t *testing.T) {
	t.Parallel()
	ds := newDS()
	ds.SetEntity("products", []validation.Row{
		{"id": "p1"}, {"id": "p2"},
	})
	ds.SetEntity("stock_on_hand", []validation.Row{
		{"id": "s1", "product_id": "p1"},
		{"id": "s2", "product_id": "p_missing"},
		{"id": "s3", "product_id": ""}, // null/empty → пропускаем
	})
	rule := validation.Rule{
		Name: "stock_fk_product", Kind: "fk_exists", Entity: "stock_on_hand",
		Severity: validation.SeverityCritical,
		Column:   "product_id", RefEntity: "products", RefColumn: "id",
	}
	violations := validation.FkExistsRule(rule, ds)
	require.Len(t, violations, 1)
	assert.Equal(t, "s2", violations[0].BusinessKey)
	assert.Equal(t, validation.SeverityCritical, violations[0].Severity)
}

func TestFkExistsRule_Misconfigured(t *testing.T) {
	t.Parallel()
	rule := validation.Rule{Name: "bad", Kind: "fk_exists", Entity: "x", Severity: validation.SeverityCritical}
	v := validation.FkExistsRule(rule, newDS())
	require.Len(t, v, 1)
	assert.Equal(t, validation.SeverityCritical, v[0].Severity)
}

// --- unique_business_key ---

func TestUniqueBusinessKeyRule(t *testing.T) {
	t.Parallel()
	ds := newDS()
	ds.SetEntity("products", []validation.Row{
		{"id": "p1", "sku": "A"},
		{"id": "p2", "sku": "B"},
		{"id": "p3", "sku": "A"}, // duplicate by sku
		{"id": "p4", "sku": ""},  // skip — empty key
	})
	rule := validation.Rule{
		Name: "products_unique_sku", Kind: "unique_business_key", Entity: "products",
		Severity: validation.SeverityCritical, Keys: []string{"sku"},
	}
	v := validation.UniqueBusinessKeyRule(rule, ds)
	require.Len(t, v, 1)
	assert.Equal(t, "A", v[0].BusinessKey)
}

func TestUniqueBusinessKeyRule_Misconfigured(t *testing.T) {
	t.Parallel()
	rule := validation.Rule{Name: "bad", Kind: "unique_business_key", Entity: "x", Severity: validation.SeverityCritical}
	v := validation.UniqueBusinessKeyRule(rule, newDS())
	require.Len(t, v, 1)
}

// --- aggregate_sum_matches ---

func TestAggregateSumMatchesRule_Match(t *testing.T) {
	t.Parallel()
	ds := newDS()
	ds.SetEntity("receipts", []validation.Row{{"qty": 10.0}, {"qty": 5.0}})
	ds.SetEntity("dispatch", []validation.Row{{"qty": 7.5}, {"qty": 7.5}})
	rule := validation.Rule{
		Name: "rev_match", Kind: "aggregate_sum_matches", Entity: "receipts",
		Severity: validation.SeverityCritical,
		SumColumn: "qty", RefEntity: "dispatch", RefSum: "qty",
	}
	require.Empty(t, validation.AggregateSumMatchesRule(rule, ds))
}

func TestAggregateSumMatchesRule_Mismatch(t *testing.T) {
	t.Parallel()
	ds := newDS()
	ds.SetEntity("receipts", []validation.Row{{"qty": 10.0}})
	ds.SetEntity("dispatch", []validation.Row{{"qty": 9.0}})
	rule := validation.Rule{
		Name: "rev_match", Kind: "aggregate_sum_matches", Entity: "receipts",
		Severity: validation.SeveritySoft,
		SumColumn: "qty", RefEntity: "dispatch", RefSum: "qty",
	}
	v := validation.AggregateSumMatchesRule(rule, ds)
	require.Len(t, v, 1)
	assert.Equal(t, validation.SeveritySoft, v[0].Severity)
}

func TestAggregateSumMatchesRule_Misconfigured(t *testing.T) {
	t.Parallel()
	rule := validation.Rule{Name: "bad", Kind: "aggregate_sum_matches", Entity: "x", Severity: validation.SeverityCritical}
	v := validation.AggregateSumMatchesRule(rule, newDS())
	require.Len(t, v, 1)
}

// --- referential_integrity ---

func TestReferentialIntegrityRule(t *testing.T) {
	t.Parallel()
	ds := newDS()
	ds.SetEntity("suppliers", []validation.Row{{"id": "sup1"}, {"id": "sup2"}})
	ds.SetEntity("order_rule", []validation.Row{{"id": "or1", "supplier_id": "sup1"}})
	rule := validation.Rule{
		Name: "supplier_used", Kind: "referential_integrity", Entity: "order_rule",
		Severity: validation.SeveritySoft,
		Column:   "supplier_id", RefEntity: "suppliers", RefColumn: "id",
	}
	v := validation.ReferentialIntegrityRule(rule, ds)
	require.Len(t, v, 1)
	assert.Equal(t, "sup2", v[0].BusinessKey)
}

func TestReferentialIntegrityRule_Misconfigured(t *testing.T) {
	t.Parallel()
	rule := validation.Rule{Name: "bad", Kind: "referential_integrity", Entity: "x", Severity: validation.SeverityCritical}
	v := validation.ReferentialIntegrityRule(rule, newDS())
	require.Len(t, v, 1)
}

// --- null_required_field ---

func TestNullRequiredFieldRule(t *testing.T) {
	t.Parallel()
	ds := newDS()
	ds.SetEntity("products", []validation.Row{
		{"id": "p1", "name": "Apple"},
		{"id": "p2", "name": ""},  // violation
		{"id": "p3"},               // missing → violation
	})
	rule := validation.Rule{
		Name: "products_name_required", Kind: "null_required_field", Entity: "products",
		Severity: validation.SeverityCritical, Column: "name",
	}
	v := validation.NullRequiredFieldRule(rule, ds)
	assert.Len(t, v, 2)
}

func TestNullRequiredFieldRule_Misconfigured(t *testing.T) {
	t.Parallel()
	rule := validation.Rule{Name: "bad", Kind: "null_required_field", Entity: "x", Severity: validation.SeverityCritical}
	v := validation.NullRequiredFieldRule(rule, newDS())
	require.Len(t, v, 1)
}

// --- registry / engine ---

func TestDefaultRegistry_HasAllBuiltins(t *testing.T) {
	t.Parallel()
	r := validation.DefaultRegistry()
	for _, kind := range []string{
		"fk_exists", "unique_business_key", "aggregate_sum_matches",
		"referential_integrity", "null_required_field",
	} {
		_, ok := r.Get(kind)
		assert.True(t, ok, "%s must be registered", kind)
	}
}

func TestRegistry_DuplicateRegistration(t *testing.T) {
	t.Parallel()
	r := validation.NewRegistry()
	require.NoError(t, r.Register("custom", validation.FkExistsRule))
	require.Error(t, r.Register("custom", validation.FkExistsRule))
	require.Error(t, r.Register("", validation.FkExistsRule))
	require.Error(t, r.Register("nilfn", nil))
}

func TestEngine_Run_HappyPath(t *testing.T) {
	t.Parallel()
	yaml := []byte(`
version: 1
rules:
  - name: products_unique_id
    kind: unique_business_key
    entity: products
    severity: critical
    keys: [id]
  - name: stock_fk_product
    kind: fk_exists
    entity: stock_on_hand
    severity: critical
    column: product_id
    ref_entity: products
    ref_column: id
`)
	eng, err := validation.LoadBytes(yaml)
	require.NoError(t, err)
	ds := newDS()
	ds.SetEntity("products", []validation.Row{{"id": "p1"}, {"id": "p2"}, {"id": "p1"}})
	ds.SetEntity("stock_on_hand", []validation.Row{
		{"id": "s1", "product_id": "p1"},
		{"id": "s2", "product_id": "p_missing"},
	})
	r := eng.Run(ds)
	assert.Equal(t, 5, r.LinesTotal)
	assert.Equal(t, 2, r.LinesFailed)
	assert.Equal(t, 2, r.CriticalCount())
	assert.Equal(t, 0, r.SoftCount())
}

func TestEngine_Load_InvalidVersion(t *testing.T) {
	t.Parallel()
	_, err := validation.LoadBytes([]byte("version: 99\nrules: []\n"))
	require.Error(t, err)
}

func TestEngine_Load_BadSeverity(t *testing.T) {
	t.Parallel()
	yaml := []byte(`
version: 1
rules:
  - name: r1
    kind: null_required_field
    entity: x
    severity: weird
    column: name
`)
	_, err := validation.LoadBytes(yaml)
	require.Error(t, err)
}

func TestEngine_Load_UnknownKind(t *testing.T) {
	t.Parallel()
	yaml := []byte(`
version: 1
rules:
  - name: r1
    kind: pyromancy
    entity: x
    severity: critical
`)
	_, err := validation.LoadBytes(yaml)
	require.Error(t, err)
}

func TestEngine_Load_MissingFields(t *testing.T) {
	t.Parallel()
	yaml := []byte(`
version: 1
rules:
  - kind: null_required_field
    severity: critical
`)
	_, err := validation.LoadBytes(yaml)
	require.Error(t, err)
}

func TestEngine_Load_BadYaml(t *testing.T) {
	t.Parallel()
	_, err := validation.LoadBytes([]byte("not: [valid"))
	require.Error(t, err)
}

func TestEngine_Run_NilDataset(t *testing.T) {
	t.Parallel()
	eng := validation.New(validation.DefaultRegistry(), nil)
	r := eng.Run(nil)
	assert.Equal(t, 0, r.LinesTotal)
}

func TestEngine_Rules_Copy(t *testing.T) {
	t.Parallel()
	original := []validation.Rule{{Name: "r1", Kind: "null_required_field", Entity: "x", Severity: validation.SeverityCritical, Column: "y"}}
	eng := validation.New(validation.DefaultRegistry(), original)
	cp := eng.Rules()
	cp[0].Name = "mutated"
	assert.Equal(t, "r1", eng.Rules()[0].Name)
}

func TestSeverity_IsValid(t *testing.T) {
	t.Parallel()
	assert.True(t, validation.SeverityCritical.IsValid())
	assert.True(t, validation.SeveritySoft.IsValid())
	assert.False(t, validation.Severity("warn").IsValid())
}

func TestDataset_AddAndAccessors(t *testing.T) {
	t.Parallel()
	ds := newDS()
	ds.Add("products", validation.Row{"id": "p1"})
	ds.Add("products", validation.Row{"id": "p2"})
	assert.Len(t, ds.Rows("products"), 2)
	assert.Empty(t, ds.Rows("ghost"))
	assert.Contains(t, ds.Entities(), "products")
	assert.Equal(t, 2, ds.CountAll())
}
