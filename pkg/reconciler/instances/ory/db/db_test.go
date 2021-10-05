package db

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/types"
)

const (
	postgresYaml = `
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
                  enabled: true`

	gcloudYaml = `
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
                dbType: postgres`

	customDBYaml = `
        global:
          ory:
            hydra:
              persistence:
                enabled: true
                gcloud:
                  enabled: false
                user: hydra
                password: secretpw
                dbUrl: mydb.mynamespace.svc.cluster.local:1234
                dbName: db4hydra
                dbType: mysql`

	noDBYaml = `
        global:
          ory:
            hydra:
              persistence:
                enabled: false`
)

func TestDBSecret(t *testing.T) {
	t.Run("Deployment with Postgres SQL in cluster", func(t *testing.T) {
		// given
		t.Parallel()
		name := types.NamespacedName{Name: "test-postgres-secret", Namespace: "test"}
		var values map[string]interface{}
		err := yaml.Unmarshal([]byte(postgresYaml), &values)
		require.NoError(t, err)

		// when
		cfg, errNew := new(values)
		dsnExpected := cfg.preparePostgresDSN()
		secret, errGet := Get(name, values)

		// then
		require.NoError(t, errNew)
		require.NoError(t, errGet)
		assert.Equal(t, secret.StringData["postgresql-password"], "secretpw")
		assert.Equal(t, secret.StringData["dsn"], dsnExpected)

	})

	t.Run("Deployment with GCloud SQL", func(t *testing.T) {
		// given
		t.Parallel()
		name := types.NamespacedName{Name: "test-gcloud-secret", Namespace: "test"}
		var values map[string]interface{}
		err := yaml.Unmarshal([]byte(gcloudYaml), &values)
		require.NoError(t, err)

		// when
		cfg, errNew := new(values)
		dsnExpected := cfg.prepareGenericDSN()
		secret, errGet := Get(name, values)

		// then
		require.NoError(t, errNew)
		require.NoError(t, errGet)
		assert.Equal(t, secret.StringData["dsn"], dsnExpected)
		assert.Equal(t, secret.StringData["gcp-sa.json"], "testsa")

	})

	t.Run("Deployment with Custom DB", func(t *testing.T) {
		// given
		t.Parallel()
		name := types.NamespacedName{Name: "test-customDB-secret", Namespace: "test"}
		var values map[string]interface{}
		err := yaml.Unmarshal([]byte(customDBYaml), &values)
		require.NoError(t, err)

		// when
		cfg, errNew := new(values)
		dsnExpected := cfg.prepareGenericDSN()
		secret, errGet := Get(name, values)

		// then
		require.NoError(t, errNew)
		require.NoError(t, errGet)
		assert.Equal(t, secret.StringData["dsn"], dsnExpected)
		assert.Equal(t, secret.StringData["dbPassword"], "secretpw")
	})

	t.Run("Deployment without database", func(t *testing.T) {
		// given
		t.Parallel()
		name := types.NamespacedName{Name: "test-memory-secret", Namespace: "test"}
		var values map[string]interface{}
		err := yaml.Unmarshal([]byte(noDBYaml), &values)
		require.NoError(t, err)

		// when
		secret, err := Get(name, values)

		// then
		require.NoError(t, err)
		assert.Equal(t, secret.StringData["dsn"], "memory")
	})

}
