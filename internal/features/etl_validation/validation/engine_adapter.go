package validation

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// FileSchema — корневая структура YAML конфига etl_validation_rules.yaml.
type FileSchema struct {
	Version int    `yaml:"version"`
	Rules   []Rule `yaml:"rules"`
}

// Engine — runtime для прогона правил над Dataset.
type Engine struct {
	registry *Registry
	rules    []Rule
}

// New создаёт Engine с явно переданным registry. Используется в тестах.
func New(reg *Registry, rules []Rule) *Engine {
	return &Engine{registry: reg, rules: rules}
}

// Load читает yamlPath, валидирует структуру, регистрирует CheckFunc-и
// из DefaultRegistry.
func Load(yamlPath string) (*Engine, error) {
	raw, err := os.ReadFile(yamlPath) //nolint:gosec // путь приходит из appconfig
	if err != nil {
		return nil, fmt.Errorf("validation.Load: read %s: %w", yamlPath, err)
	}
	return parseYAML(raw)
}

// LoadBytes — версия Load для тестов / готового []byte.
func LoadBytes(raw []byte) (*Engine, error) {
	return parseYAML(raw)
}

func parseYAML(raw []byte) (*Engine, error) {
	var sc FileSchema
	if err := yaml.Unmarshal(raw, &sc); err != nil {
		return nil, fmt.Errorf("validation.parseYAML: %w", err)
	}
	if sc.Version != 1 {
		return nil, fmt.Errorf("validation.parseYAML: unsupported version %d", sc.Version)
	}
	reg := DefaultRegistry()
	for _, r := range sc.Rules {
		if r.Name == "" || r.Kind == "" || r.Entity == "" || r.Severity == "" {
			return nil, fmt.Errorf("validation.parseYAML: rule %+v missing required fields", r)
		}
		if !r.Severity.IsValid() {
			return nil, fmt.Errorf("validation.parseYAML: rule %s invalid severity %q", r.Name, r.Severity)
		}
		if _, ok := reg.Get(r.Kind); !ok {
			return nil, fmt.Errorf("validation.parseYAML: rule %s unknown kind %q", r.Name, r.Kind)
		}
	}
	return &Engine{registry: reg, rules: sc.Rules}, nil
}

// Report — итог прогона engine над Dataset.
type Report struct {
	LinesTotal  int
	LinesFailed int
	Violations  []Violation
}

// CriticalCount — число critical-нарушений.
func (r Report) CriticalCount() int {
	n := 0
	for _, v := range r.Violations {
		if v.Severity == SeverityCritical {
			n++
		}
	}
	return n
}

// SoftCount — число soft-нарушений.
func (r Report) SoftCount() int {
	n := 0
	for _, v := range r.Violations {
		if v.Severity == SeveritySoft {
			n++
		}
	}
	return n
}

// Run прогоняет все rules engine над ds, возвращает Report.
//
// Контракт: engine не выбрасывает ошибок (за исключением misconfigured-rules,
// которые превращаются в Violation с Severity=critical).
func (e *Engine) Run(ds *Dataset) Report {
	if ds == nil {
		ds = NewDataset()
	}
	out := make([]Violation, 0)
	for _, rule := range e.rules {
		fn, ok := e.registry.Get(rule.Kind)
		if !ok {
			out = append(out, Violation{
				RuleName: rule.Name, Kind: rule.Kind, Entity: rule.Entity,
				Severity: SeverityCritical,
				Message:  fmt.Sprintf("неизвестный kind %q", rule.Kind),
			})
			continue
		}
		out = append(out, fn(rule, ds)...)
	}
	return Report{
		LinesTotal:  ds.CountAll(),
		LinesFailed: len(out),
		Violations:  out,
	}
}

// Rules возвращает копию правил (для логирования / диагностики).
func (e *Engine) Rules() []Rule {
	cp := make([]Rule, len(e.rules))
	copy(cp, e.rules)
	return cp
}
