package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"gopkg.in/yaml.v3"
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

	mysqlDBYaml = `
        global:
          ory:
            hydra:
              persistence:
                enabled: true
                gcloud:
                  enabled: false
                user: hydra
                password: secretpw
                dbUrl: mydb.mynamespace:1234
                dbName: db4hydra
                dbType: mysql`

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
                dbUrl: mydb.mynamespace:1234
                dbName: db4hydra
                dbType: cockroach`

	noDBYaml = `
        global:
          ory:
            hydra:
              persistence:
                enabled: false`

	wrongYaml = `
    ---       
        global:
          oryt:
            hydra:
             persistence:
                enabled: true
                postgresql:
                  enabled: true`
)

func TestDBSecret(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()

	t.Run("Deployment with Postgres SQL in cluster", func(t *testing.T) {
		// given
		t.Parallel()
		name := types.NamespacedName{Name: "test-postgres-secret", Namespace: "test"}
		values, err := unmarshalTestValues(postgresYaml)
		require.NoError(t, err)
		cfg, errNew := newDBConfig(values)
		dsnExpected := cfg.preparePostgresDSN()

		// when
		secret, errGet := Get(name, values, logger)

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
		values, err := unmarshalTestValues(gcloudYaml)
		require.NoError(t, err)
		cfg, errNew := newDBConfig(values)
		dsnExpected := cfg.prepareGenericDSN()

		// when
		secret, errGet := Get(name, values, logger)

		// then
		require.NoError(t, errNew)
		require.NoError(t, errGet)
		assert.Equal(t, secret.StringData["dsn"], dsnExpected)
		assert.Equal(t, secret.StringData["gcp-sa.json"], "testsa")

	})

	t.Run("Deployment with mysql DB", func(t *testing.T) {
		// given
		t.Parallel()
		name := types.NamespacedName{Name: "test-mysqlDB-secret", Namespace: "test"}
		values, err := unmarshalTestValues(mysqlDBYaml)
		require.NoError(t, err)
		cfg, errNew := newDBConfig(values)
		dsnExpected := cfg.prepareMySQLDSN()

		// when
		secret, errGet := Get(name, values, logger)

		// then
		require.NoError(t, errNew)
		require.NoError(t, errGet)
		assert.Equal(t, secret.StringData["dsn"], dsnExpected)
		assert.Equal(t, secret.StringData["dbPassword"], "secretpw")
	})

	t.Run("Deployment with Custom DB", func(t *testing.T) {
		// given
		t.Parallel()
		name := types.NamespacedName{Name: "test-customDB-secret", Namespace: "test"}
		values, err := unmarshalTestValues(customDBYaml)
		require.NoError(t, err)
		cfg, errNew := newDBConfig(values)
		dsnExpected := cfg.prepareGenericDSN()

		// when
		secret, errGet := Get(name, values, logger)

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
		values, err := unmarshalTestValues(noDBYaml)
		require.NoError(t, err)

		// when
		secret, err := Get(name, values, logger)

		// then
		require.NoError(t, err)
		assert.Equal(t, secret.StringData["dsn"], "memory")
	})

	t.Run("Deployment with yaml values error", func(t *testing.T) {
		t.Parallel()
		values, err := unmarshalTestValues(wrongYaml)
		assert.Nil(t, values)
		assert.Error(t, err)
	})

}

func unmarshalTestValues(yamlValues string) (map[string]interface{}, error) {
	var values map[string]interface{}
	err := yaml.Unmarshal([]byte(yamlValues), &values)
	if err != nil {
		return nil, err
	}
	return values, nil
}
