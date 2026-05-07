// Package configs содержит unit-проверки для статических YAML-конфигов
// сервиса (validation_rules.yaml и др.). Парсинг и smoke-проверки тут,
// чтобы поломанный YAML не доехал до прода.
package configs_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/Kitavrus/e_zoo/internal/features/data_export/validation"
)

func yamlPath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(thisFile), "validation_rules.yaml")
}

func TestYAML_Parses(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(yamlPath(t))
	require.NoError(t, err)
	var sc validation.FileSchema
	require.NoError(t, yaml.Unmarshal(raw, &sc))
	require.Equal(t, 1, sc.Version)
	require.GreaterOrEqual(t, len(sc.Rules), 7)
}

func TestYAML_AllRulesHaveSeverity(t *testing.T) {
	t.Parallel()
	eng, err := validation.Load(yamlPath(t))
	require.NoError(t, err)
	for _, r := range eng.Rules() {
		require.NotEmpty(t, r.ID, "rule must have id")
		require.NotEmpty(t, r.Entity, "rule %s must have entity", r.ID)
		require.NotEmpty(t, r.Check, "rule %s must have check", r.ID)
		require.Contains(t, []validation.Severity{
			validation.SeverityCritical, validation.SeveritySoft,
		}, r.Severity, "rule %s severity invalid", r.ID)
	}
}
