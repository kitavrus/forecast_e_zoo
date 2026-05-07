package transformer

import "fmt"

// Registry — runtime-набор всех известных Builder-ов.
type Registry struct {
	byName map[string]Builder
	order  []string
}

// NewRegistry собирает реестр со всеми известными builder-ами по DI.
func NewRegistry(repo MartUpserter) *Registry {
	r := &Registry{byName: make(map[string]Builder)}
	r.add(NewMasterCurrentBuilder(repo))     // справочники сначала (FK-цели)
	r.add(NewDemandHistoryBuilder(repo))     // потом demand
	r.add(NewKpiDailyBuilder(repo))          // потом kpi
	r.add(NewCalculationInputBuilder(repo))  // потом calculation_input (зависит от masters)
	r.add(NewSupplierScorecardBuilder(repo)) // on-demand (не идёт в полный run)
	return r
}

func (r *Registry) add(b Builder) {
	r.byName[b.Name()] = b
	r.order = append(r.order, b.Name())
}

// BuildersForFullRun возвращает builder-ы, исключая OnDemandOnly.
//
// Порядок — детерминированный (как в NewRegistry), что важно
// для DAG-зависимостей (mart_master_current сначала).
func (r *Registry) BuildersForFullRun() []Builder {
	out := make([]Builder, 0, len(r.order))
	for _, name := range r.order {
		b := r.byName[name]
		if b.OnDemandOnly() {
			continue
		}
		out = append(out, b)
	}
	return out
}

// BuilderByName ищет builder по имени mart.
func (r *Registry) BuilderByName(name string) (Builder, error) {
	b, ok := r.byName[name]
	if !ok {
		return nil, fmt.Errorf("transformer: unknown mart %q", name)
	}
	return b, nil
}

// Names возвращает все имена в порядке регистрации (для отладки/логирования).
func (r *Registry) Names() []string {
	cp := make([]string, len(r.order))
	copy(cp, r.order)
	return cp
}
