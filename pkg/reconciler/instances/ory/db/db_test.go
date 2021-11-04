package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		assert.Equal(t, "memory", secret.StringData["dsn"])
	})

	t.Run("Deployment with yaml values error", func(t *testing.T) {
		t.Parallel()
		values, err := unmarshalTestValues(wrongYaml)
		assert.Nil(t, values)
		assert.Error(t, err)
	})
}

func TestDBSecretUpdate(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()

	t.Run("Existing secret with memory persistence should not be updated with the same config", func(t *testing.T) {
		// given
		t.Parallel()
		values, err := unmarshalTestValues(noDBYaml)
		require.NoError(t, err)
		oldSecret := fixSecretWithNameDsn("test-memory-secret", "memory")

		// when
		secretData, errUpdate := Update(values, oldSecret, logger)

		// then
		require.NoError(t, errUpdate)
		assert.Equal(t, 0, len(secretData))
	})

	t.Run("Existing secret with postgres persistence should not be updated with the same config", func(t *testing.T) {
		// given
		t.Parallel()
		values, err := unmarshalTestValues(postgresYaml)
		require.NoError(t, err)
		oldSecret := fixSecretWithNameDsn("test-postgres-secret",
			"postgres://hydra:secretpw@ory-postgresql.kyma-system.svc.cluster.local:5432/db4hydra?sslmode=disable&max_conn_lifetime=10s")

		// when
		secretData, errUpdate := Update(values, oldSecret, logger)

		// then
		require.NoError(t, errUpdate)
		assert.Equal(t, 0, len(secretData))
	})

	t.Run("Existing secret with gcloud persistence should not be updated with the same config", func(t *testing.T) {
		// given
		t.Parallel()
		values, err := unmarshalTestValues(gcloudYaml)
		require.NoError(t, err)
		oldSecret := fixSecretWithNameDsn("test-postgres-secret",
			"postgres://hydra:secretpw@ory-gcloud-sqlproxy.kyma-system:5432/db4hydra?sslmode=disable&max_conn_lifetime=10s")

		// when
		secretData, errUpdate := Update(values, oldSecret, logger)

		// then
		require.NoError(t, errUpdate)
		assert.Equal(t, 0, len(secretData))
	})

	t.Run("Existing secret with mysql persistence should not be updated with the same config", func(t *testing.T) {
		// given
		t.Parallel()
		values, err := unmarshalTestValues(mysqlDBYaml)
		require.NoError(t, err)
		oldSecret := fixSecretWithNameDsn("test-postgres-secret",
			"mysql://hydra:secretpw@tcp(mydb.mynamespace:1234)/db4hydra?parseTime=true&max_conn_lifetime=10s")

		// when
		secretData, errUpdate := Update(values, oldSecret, logger)

		// then
		require.NoError(t, errUpdate)
		assert.Equal(t, 0, len(secretData))
	})

	t.Run("Existing secret with custom persistence should not be updated with the same config", func(t *testing.T) {
		// given
		t.Parallel()
		values, err := unmarshalTestValues(customDBYaml)
		require.NoError(t, err)
		oldSecret := fixSecretWithNameDsn("test-postgres-secret",
			"cockroach://hydra:secretpw@mydb.mynamespace:1234/db4hydra?sslmode=disable&max_conn_lifetime=10s")

		// when
		secretData, errUpdate := Update(values, oldSecret, logger)

		// then
		require.NoError(t, errUpdate)
		assert.Equal(t, 0, len(secretData))
	})

	t.Run("Existing secret with memory persistence should be updated with postgres config", func(t *testing.T) {
		// given
		t.Parallel()
		values, err := unmarshalTestValues(postgresYaml)
		require.NoError(t, err)
		oldSecret := fixSecretWithNameDsn("test-memory-secret", "memory")

		// when
		secretData, errUpdate := Update(values, oldSecret, logger)

		// then
		require.NoError(t, errUpdate)
		assert.NotEqual(t, 0, len(secretData))
		assert.Contains(t, secretData, "postgresql-password")
		assert.Contains(t, secretData["dsn"], postgresURL)
	})

	t.Run("Existing secret with memory persistence should be updated with gcloud config", func(t *testing.T) {
		// given
		t.Parallel()
		values, err := unmarshalTestValues(gcloudYaml)
		require.NoError(t, err)
		oldSecret := fixSecretWithNameDsn("test-memory-secret", "memory")

		// when
		secretData, errUpdate := Update(values, oldSecret, logger)

		// then
		require.NoError(t, errUpdate)
		assert.NotEqual(t, 0, len(secretData))
		assert.Contains(t, secretData, "gcp-sa.json")
		assert.Contains(t, secretData["dsn"], "postgres")
		assert.Contains(t, secretData["dsn"], "ory-gcloud-sqlproxy.kyma-system:5432")
		assert.Contains(t, secretData["dsn"], "db4hydra")
		assert.Contains(t, secretData["dsn"], dsnOpts)
	})

	t.Run("Existing secret with memory persistence should be updated with mysql config", func(t *testing.T) {
		// given
		t.Parallel()
		values, err := unmarshalTestValues(mysqlDBYaml)
		require.NoError(t, err)
		oldSecret := fixSecretWithNameDsn("test-memory-secret", "memory")

		// when
		secretData, errUpdate := Update(values, oldSecret, logger)

		// then
		require.NoError(t, errUpdate)
		assert.NotEqual(t, 0, len(secretData))
		assert.Contains(t, secretData["dsn"], "mysql")
		assert.Contains(t, secretData["dsn"], "mydb.mynamespace:1234")
		assert.Contains(t, secretData["dsn"], "db4hydra")
		assert.Contains(t, secretData["dsn"], mysqlDsnOpts)
	})

	t.Run("Existing secret with memory persistence should be updated with custom db config", func(t *testing.T) {
		// given
		t.Parallel()
		values, err := unmarshalTestValues(customDBYaml)
		require.NoError(t, err)
		oldSecret := fixSecretWithNameDsn("test-memory-secret", "memory")

		// when
		secretData, errUpdate := Update(values, oldSecret, logger)

		// then
		require.NoError(t, errUpdate)
		assert.NotEqual(t, 0, len(secretData))
		assert.Contains(t, secretData["dsn"], "cockroach")
		assert.Contains(t, secretData["dsn"], "mydb.mynamespace:1234")
		assert.Contains(t, secretData["dsn"], "db4hydra")
		assert.Contains(t, secretData["dsn"], dsnOpts)
	})

	t.Run("Existing secret with postgresql persistence should be updated with memory config", func(t *testing.T) {
		// given
		t.Parallel()
		values, err := unmarshalTestValues(noDBYaml)
		require.NoError(t, err)
		oldSecret := fixSecretWithNameDsn("test-postgres-secret",
			"postgres://hydra:pass@ory-postgresql.kyma-system.svc.cluster.local:5432/db4hydra?sslmode=disable&max_conn_lifetime=10s")

		// when
		secretData, errUpdate := Update(values, oldSecret, logger)

		// then
		require.NoError(t, errUpdate)
		assert.NotEqual(t, 0, len(secretData))
		assert.Contains(t, secretData["dsn"], "memory")
	})

	t.Run("Existing secret with postgresql persistence should be updated with mysql config", func(t *testing.T) {
		// given
		t.Parallel()
		values, err := unmarshalTestValues(mysqlDBYaml)
		require.NoError(t, err)
		oldSecret := fixSecretWithNameDsn("test-postgres-secret",
			"postgres://hydra:pass@ory-postgresql.kyma-system.svc.cluster.local:5432/db4hydra?sslmode=disable&max_conn_lifetime=10s")

		// when
		secretData, errUpdate := Update(values, oldSecret, logger)

		// then
		require.NoError(t, errUpdate)
		assert.NotEqual(t, 0, len(secretData))
		assert.Contains(t, secretData["dsn"], "mysql")
		assert.Contains(t, secretData["dsn"], "mydb.mynamespace:1234")
		assert.Contains(t, secretData["dsn"], "db4hydra")
		assert.Contains(t, secretData["dsn"], mysqlDsnOpts)
	})

	t.Run("Existing secret with gcloud persistence should be updated with custom db config", func(t *testing.T) {
		// given
		t.Parallel()
		values, err := unmarshalTestValues(customDBYaml)
		require.NoError(t, err)
		oldSecret := fixSecretWithNameDsn("test-postgres-secret",
			"postgres://hydra:secretpw@ory-gcloud-sqlproxy.kyma-system:5432/db4hydra?sslmode=disable&max_conn_lifetime=10s")

		// when
		secretData, errUpdate := Update(values, oldSecret, logger)

		// then
		require.NoError(t, errUpdate)
		assert.NotEqual(t, 0, len(secretData))
		assert.Contains(t, secretData["dsn"], "cockroach")
		assert.Contains(t, secretData["dsn"], "mydb.mynamespace:1234")
		assert.Contains(t, secretData["dsn"], "db4hydra")
		assert.Contains(t, secretData["dsn"], dsnOpts)
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

func fixSecretWithNameDsn(name, dsn string) *v1.Secret {
	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "test",
		},
		Data: map[string][]byte{
			"dsn":           []byte(dsn),
			"secretsCookie": []byte("somesecretcookie"),
			"secretsSystem": []byte("somesecretsystem"),
		},
	}
	return &secret
}
