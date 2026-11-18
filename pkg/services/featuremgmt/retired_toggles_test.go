package featuremgmt

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	featuretoggleapi "github.com/grafana/grafana/pkg/services/featuremgmt/feature_toggle_api"
	"github.com/grafana/grafana/pkg/setting"
)

const elasticsearchCrossClusterSearch = "elasticsearchCrossClusterSearch"

func TestElasticsearchCrossClusterSearchRetired(t *testing.T) {
	for _, flag := range standardFeatureFlags {
		require.NotEqual(t, elasticsearchCrossClusterSearch, flag.Name)
	}

	t.Run("default config leaves the retired toggle disabled", func(t *testing.T) {
		cfg, err := setting.NewCfgFromBytes([]byte("[feature_toggles]\n"))
		require.NoError(t, err)

		mgmt, err := ProvideManagerService(cfg)
		require.NoError(t, err)
		require.False(t, mgmt.IsEnabledGlobally(elasticsearchCrossClusterSearch))
		require.NotContains(t, mgmt.warnings, elasticsearchCrossClusterSearch)
	})

	t.Run("an old config value does not block startup", func(t *testing.T) {
		cfg, err := setting.NewCfgFromBytes([]byte(`
[feature_toggles]
elasticsearchCrossClusterSearch = true
`))
		require.NoError(t, err)

		mgmt, err := ProvideManagerService(cfg)
		require.NoError(t, err)
		require.Equal(t, "unknown flag in config", mgmt.warnings[elasticsearchCrossClusterSearch])
		// Config still applies the value. Older plugin builds that opted in keep
		// seeing the flag; current builds no longer read it.
		require.True(t, mgmt.IsEnabledGlobally(elasticsearchCrossClusterSearch))
	})
}

func TestElasticsearchCrossClusterSearchTombstone(t *testing.T) {
	body, err := os.ReadFile("toggles_gen.json")
	require.NoError(t, err)

	var list featuretoggleapi.FeatureList
	require.NoError(t, json.Unmarshal(body, &list))

	var found *featuretoggleapi.Feature
	for i := range list.Items {
		if list.Items[i].Name == elasticsearchCrossClusterSearch {
			found = &list.Items[i]
			break
		}
	}

	require.NotNil(t, found)
	require.NotNil(t, found.DeletionTimestamp)
	require.Equal(t, "Enables cross cluster search in the Elasticsearch data source", found.Spec.Description)
	require.Equal(t, "GA", found.Spec.Stage)
}
