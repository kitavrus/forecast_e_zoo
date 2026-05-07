// Package validation содержит severity-движок бизнес-валидации
// (config-driven, ADR-006). Правила задаются YAML-файлом, обработчики —
// в builtin.go. Engine не выбрасывает ошибок, а собирает Violations,
// которые loader (фаза 10) разделяет: critical → fail load, soft → reject_log.
package validation

import (
	"fmt"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

// Severity — severity-уровень правила.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeveritySoft     Severity = "soft"
)

// Rule — конфигурация одного правила (yaml).
type Rule struct {
	ID       string   `yaml:"id"`
	Entity   string   `yaml:"entity"`
	Check    string   `yaml:"check"`
	Field    string   `yaml:"field,omitempty"`
	Fields   []string `yaml:"fields,omitempty"`
	Pattern  string   `yaml:"pattern,omitempty"`
	Min      *float64 `yaml:"min,omitempty"`
	Max      *float64 `yaml:"max,omitempty"`
	Severity Severity `yaml:"severity"`
}

// FileSchema — схема validation_rules.yaml.
type FileSchema struct {
	Version        int      `yaml:"version"`
	Rules          []Rule   `yaml:"rules"`
	EntityOptional []string `yaml:"entity_optional,omitempty"`
}

// CheckFunc — обработчик правила; ok=true → нарушения нет.
type CheckFunc func(rule Rule, payload map[string]any, state *State) (ok bool, msg string)

// State — изменяемое состояние engine в рамках одного load (для duplicate_pk и т.п.).
type State struct {
	mu       sync.Mutex
	seenPKs  map[string]map[string]struct{} // entity → set of pk strings
	loadID   string
}

// NewState создаёт новое состояние для load loadID.
func NewState(loadID string) *State {
	return &State{
		seenPKs: make(map[string]map[string]struct{}),
		loadID:  loadID,
	}
}

// markPK — true, если pk впервые встречен; false если duplicate.
func (s *State) markPK(entity, pk string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	set, ok := s.seenPKs[entity]
	if !ok {
		set = make(map[string]struct{})
		s.seenPKs[entity] = set
	}
	if _, exists := set[pk]; exists {
		return false
	}
	set[pk] = struct{}{}
	return true
}

// Engine — severity-движок.
type Engine struct {
	rules          []Rule
	byEntity       map[string][]Rule
	checks         map[string]CheckFunc
	entityOptional map[string]struct{}
}

// New создаёт Engine с дефолтным набором builtin checks.
func New(rules []Rule, optional []string) *Engine {
	e := &Engine{
		rules:          rules,
		byEntity:       make(map[string][]Rule),
		checks:         defaultChecks(),
		entityOptional: make(map[string]struct{}, len(optional)),
	}
	for _, r := range rules {
		e.byEntity[r.Entity] = append(e.byEntity[r.Entity], r)
	}
	for _, ent := range optional {
		e.entityOptional[ent] = struct{}{}
	}
	return e
}

// Load парсит YAML-файл и возвращает Engine.
func Load(yamlPath string) (*Engine, error) {
	raw, err := os.ReadFile(yamlPath) //nolint:gosec // путь приходит из конфига сервиса
	if err != nil {
		return nil, fmt.Errorf("validation.Load: read %s: %w", yamlPath, err)
	}
	var sc FileSchema
	if err := yaml.Unmarshal(raw, &sc); err != nil {
		return nil, fmt.Errorf("validation.Load: parse %s: %w", yamlPath, err)
	}
	if sc.Version != 1 {
		return nil, fmt.Errorf("validation.Load: unsupported version %d", sc.Version)
	}
	for _, r := range sc.Rules {
		if r.ID == "" || r.Entity == "" || r.Check == "" || r.Severity == "" {
			return nil, fmt.Errorf("validation.Load: rule %+v missing required fields", r)
		}
		if r.Severity != SeverityCritical && r.Severity != SeveritySoft {
			return nil, fmt.Errorf("validation.Load: rule %s has invalid severity %q", r.ID, r.Severity)
		}
	}
	return New(sc.Rules, sc.EntityOptional), nil
}

// Violation — найденное нарушение.
type Violation struct {
	RuleID   string
	Entity   string
	Field    string
	Severity Severity
	Message  string
}

// Check выполняет все правила для entity по payload, возвращает violations.
// Для entity_optional — отсутствие правил не ошибка.
func (e *Engine) Check(entity string, payload map[string]any, state *State) []Violation {
	rules := e.byEntity[entity]
	if len(rules) == 0 {
		// optional → молча. Иначе тоже ничего не возвращаем (no rules = no violations).
		return nil
	}
	out := make([]Violation, 0, len(rules))
	for _, r := range rules {
		fn, ok := e.checks[r.Check]
		if !ok {
			out = append(out, Violation{
				RuleID: r.ID, Entity: entity, Field: r.Field,
				Severity: SeverityCritical,
				Message:  fmt.Sprintf("unknown check %q in rule %s", r.Check, r.ID),
			})
			continue
		}
		valid, msg := fn(r, payload, state)
		if !valid {
			out = append(out, Violation{
				RuleID: r.ID, Entity: entity, Field: r.Field,
				Severity: r.Severity,
				Message:  msg,
			})
		}
	}
	return out
}

// IsEntityOptional — true, если у entity допустимо отсутствие правил.
func (e *Engine) IsEntityOptional(entity string) bool {
	_, ok := e.entityOptional[entity]
	return ok
}

// Rules возвращает копию slice'а правил (для логирования/диагностики).
func (e *Engine) Rules() []Rule {
	cp := make([]Rule, len(e.rules))
	copy(cp, e.rules)
	return cp
}
