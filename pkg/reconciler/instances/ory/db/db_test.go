package db

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/types"
)

func TestDBSecret(t *testing.T) {
	t.Run("Deployment with Postgres SQL in cluster", func(t *testing.T) {
		t.Parallel()

		name := types.NamespacedName{Name: "test-postgres-secret", Namespace: "test"}

		var values map[string]interface{}

		err := yaml.Unmarshal([]byte(`
        global:
          postgresql:
            postgresqlDatabase: db4hydra
            postgresqlUsername: hydra
            postgresqlPassword: secretpw
          ory:
            hydra:
              persistence:
                enabled: true
                postgresql:
                  enabled: true		
        `), &values)
		require.NoError(t, err)

		cfg, err := new(values)
		require.NoError(t, err)
		dsnExpected := cfg.preparePostgresDSN()

		secret, err := Get(name, values)
		require.NoError(t, err)
		assert.Equal(t, secret.StringData["postgresql-password"], "secretpw")
		assert.Equal(t, secret.StringData["dsn"], dsnExpected)

	})

	t.Run("Deployment with GCloud SQL", func(t *testing.T) {
		t.Parallel()

		name := types.NamespacedName{Name: "test-gcloud-secret", Namespace: "test"}

		var values map[string]interface{}

		err := yaml.Unmarshal([]byte(`
        global:
          ory:
            hydra:
              persistence:
                enabled: true
                gcloud:
                  enabled: true
                  saJson: testsa
                user: hydra
                password: secretpw
                dbUrl: ory-gcloud-sqlproxy.kyma-system:5432
                dbName: db4hydra
                dbType: postgres
        `), &values)
		require.NoError(t, err)

		cfg, err := new(values)
		require.NoError(t, err)
		dsnExpected := cfg.prepareGenericDSN()

		secret, err := Get(name, values)
		require.NoError(t, err)
		assert.Equal(t, secret.StringData["dsn"], dsnExpected)
		assert.Equal(t, secret.StringData["gcp-sa.json"], "testsa")

	})
	t.Run("Deployment without database", func(t *testing.T) {
		t.Parallel()

		name := types.NamespacedName{Name: "test-memory-secret", Namespace: "test"}

		var values map[string]interface{}

		err := yaml.Unmarshal([]byte(`
        global:
          ory:
            hydra:
              persistence:
                enabled: false
        `), &values)
		require.NoError(t, err)

		secret, err := Get(name, values)
		require.NoError(t, err)
		assert.Equal(t, secret.StringData["dsn"], "memory")
	})

}
