package validation

import "fmt"

// Rule — конфигурация правила (читается из YAML).
//
// Поля:
//   - Name: уникальный идентификатор для reject_log.rule_id;
//   - Kind: имя зарегистрированного builtin (fk_exists, unique_business_key, ...);
//   - Entity: имя entity, к которому правило применяется;
//   - Severity: critical | soft;
//   - параметры — column, ref_entity, ref_column, keys, sum_column, filter_*.
type Rule struct {
	Name        string   `yaml:"name"`
	Kind        string   `yaml:"kind"`
	Entity      string   `yaml:"entity"`
	Severity    Severity `yaml:"severity"`
	Column      string   `yaml:"column,omitempty"`
	RefEntity   string   `yaml:"ref_entity,omitempty"`
	RefColumn   string   `yaml:"ref_column,omitempty"`
	Keys        []string `yaml:"keys,omitempty"`
	SumColumn   string   `yaml:"sum_column,omitempty"`
	RefSum      string   `yaml:"ref_sum_column,omitempty"`
	Filter      string   `yaml:"filter,omitempty"`
}

// Violation — найденное нарушение для строки entity.
//
// BusinessKey — текстовое представление строки, идентифицирующее её
// в reject_log (обычно — id или composite key).
type Violation struct {
	RuleName    string
	Kind        string
	Entity      string
	Field       string
	BusinessKey string
	Severity    Severity
	Message     string
}

// CheckFunc — обработчик правила: возвращает все найденные нарушения.
type CheckFunc func(rule Rule, ds *Dataset) []Violation

// Registry — мап kind → factory CheckFunc.
type Registry struct {
	checks map[string]CheckFunc
}

// NewRegistry создаёт пустой реестр.
func NewRegistry() *Registry {
	return &Registry{checks: make(map[string]CheckFunc)}
}

// Register регистрирует CheckFunc по имени kind.
// При повторной регистрации того же имени — возвращает ошибку.
func (r *Registry) Register(kind string, fn CheckFunc) error {
	if kind == "" {
		return fmt.Errorf("registry: empty kind")
	}
	if fn == nil {
		return fmt.Errorf("registry: nil CheckFunc for %s", kind)
	}
	if _, exists := r.checks[kind]; exists {
		return fmt.Errorf("registry: kind %q already registered", kind)
	}
	r.checks[kind] = fn
	return nil
}

// Get возвращает CheckFunc по kind либо false.
func (r *Registry) Get(kind string) (CheckFunc, bool) {
	fn, ok := r.checks[kind]
	return fn, ok
}

// Kinds возвращает список зарегистрированных kind.
func (r *Registry) Kinds() []string {
	out := make([]string, 0, len(r.checks))
	for k := range r.checks {
		out = append(out, k)
	}
	return out
}

// DefaultRegistry — реестр со всеми builtin checks ETL-движка.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	mustRegister(r, "fk_exists", FkExistsRule)
	mustRegister(r, "unique_business_key", UniqueBusinessKeyRule)
	mustRegister(r, "aggregate_sum_matches", AggregateSumMatchesRule)
	mustRegister(r, "referential_integrity", ReferentialIntegrityRule)
	mustRegister(r, "null_required_field", NullRequiredFieldRule)
	return r
}

func mustRegister(r *Registry, kind string, fn CheckFunc) {
	if err := r.Register(kind, fn); err != nil {
		panic(err)
	}
}
