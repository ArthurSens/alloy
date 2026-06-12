package postgres_exporter

import (
	"testing"
	"time"

	upstream_config "github.com/prometheus-community/postgres_exporter/config"
	config_util "github.com/prometheus/common/config"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func Test_ParsePostgresURL(t *testing.T) {
	t.Run("with valid url", func(t *testing.T) {
		dsn := "postgresql://linus:42secret@localhost:5432/postgres?sslmode=disable"
		expected := map[string]string{
			"dbname":   "postgres",
			"host":     "localhost",
			"password": "42secret",
			"port":     "5432",
			"sslmode":  "disable",
			"user":     "linus",
		}

		actual, err := parsePostgresURL(dsn)
		require.NoError(t, err)
		require.Equal(t, actual, expected)
	})

	t.Run("with minimal url", func(t *testing.T) {
		minimal := "postgres://"
		expected := map[string]string{}
		actual, err := parsePostgresURL(minimal)
		require.NoError(t, err)
		require.Equal(t, actual, expected)
	})
}

func Test_getDataSourceNames(t *testing.T) {
	tt := []struct {
		name   string
		config string
		env    string
		expect []string
	}{
		{
			name:   "env",
			config: "{}",
			env:    "foo",
			expect: []string{"foo"},
		},
		{
			name:   "multi-env",
			config: "{}",
			env:    "foo,bar",
			expect: []string{"foo", "bar"},
		},
		{
			name: "config",
			config: `{
        "data_source_names": [
          "foo"
        ]
      }`,
			env:    "",
			expect: []string{"foo"},
		},
		{
			name: "config and env",
			config: `{
        "data_source_names": [
          "foo"
        ]
      }`,
			env:    "bar",
			expect: []string{"foo"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("POSTGRES_EXPORTER_DATA_SOURCE_NAME", tc.env)

			var cfg Config
			err := yaml.Unmarshal([]byte(tc.config), &cfg)
			require.NoError(t, err)

			res, err := cfg.getDataSourceNames()
			require.NoError(t, err)
			require.Equal(t, tc.expect, res)
		})
	}
}

func Test_upstreamConfig(t *testing.T) {
	cfg := Config{
		DataSourceNames:        []config_util.Secret{"postgresql://localhost:5432/postgres?sslmode=disable"},
		DisableSettingsMetrics: true,
		AutodiscoverDatabases:  true,
		ExcludeDatabases:       []string{"rdsadmin"},
		IncludeDatabases:       []string{"postgres"},
		DisableDefaultMetrics:  true,
		QueryPath:              "/tmp/queries.yaml",
		EnabledCollectors:      []string{upstream_config.CollectorStatStatements, upstream_config.CollectorStatCheckpointer},
		StatStatementFlags: &StatStatementFlags{
			IncludeQuery:     true,
			QueryLength:      200,
			Limit:            50,
			ExcludeDatabases: []string{"template0"},
			ExcludeUsers:     []string{"postgres"},
		},
	}

	dsns, err := cfg.getDataSourceNames()
	require.NoError(t, err)

	actual := cfg.upstreamConfig(dsns)
	require.Equal(t, []string{"postgresql://localhost:5432/postgres?sslmode=disable"}, actual.DataSourceNames)
	require.Equal(t, "pg", actual.MetricPrefix)
	require.Equal(t, 10*time.Second, actual.CollectionTimeout)
	require.True(t, actual.AutoDiscoverDatabases)
	require.True(t, actual.DisableDefaultMetrics)
	require.Equal(t, "/tmp/queries.yaml", actual.UserQueriesPath)
	require.Equal(t, []string{"rdsadmin"}, actual.ExcludeDatabases)
	require.Equal(t, []string{"postgres"}, actual.IncludeDatabases)
	require.True(t, actual.Collectors[upstream_config.CollectorStatStatements])
	require.True(t, actual.Collectors[upstream_config.CollectorStatCheckpointer])
	require.True(t, actual.Collectors[upstream_config.CollectorDatabase])
	require.False(t, actual.Collectors[upstream_config.CollectorSettings])
	require.Equal(t, upstream_config.PGStatStatementsConfig{
		IncludeQuery:     true,
		QueryLength:      200,
		Limit:            50,
		ExcludeDatabases: []string{"template0"},
		ExcludeUsers:     []string{"postgres"},
	}, actual.PGStatStatements)

	require.NoError(t, actual.Validate())
}
