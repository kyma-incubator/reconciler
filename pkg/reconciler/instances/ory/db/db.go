package db

import (
	"crypto/rand"
	"encoding/base64"

	"fmt"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	dsnOpts      = "sslmode=disable&max_conn_lifetime=10s"
	mysqlDsnOpts = "parseTime=true&max_conn_lifetime=10s"
	dsnTemplate  = "%s://%s:%s@%s/%s?%s"
	postgresURL  = "ory-postgresql.kyma-system.svc.cluster.local:5432"
)

func newDBConfig(chartValues map[string]interface{}) (*Config, error) {
	data, err := yaml.Marshal(chartValues)
	if err != nil {
		return nil, errors.Wrap(err, "unable to marshal Ory chart values")
	}

	val := &Config{}
	if err := yaml.Unmarshal(data, &val); err != nil {
		return nil, errors.Wrap(err, "failed to parse Ory values")
	}

	return val, nil
}

// Get fetches Kubernetes Secret object with data matching the provided Helm values to the Reconciler.
func Get(name types.NamespacedName, chartValues map[string]interface{}, logger *zap.SugaredLogger) (*v1.Secret, error) {
	cfg, err := newDBConfig(chartValues)
	if err != nil {
		return nil, err
	}

	data := cfg.prepareStringData()

	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
		StringData: data,
	}, nil
}

// IsUpdate overwrites secret with new values from config and returns true if secret was changed
func IsUpdate(name types.NamespacedName, chartValues map[string]interface{}, secret *v1.Secret, logger *zap.SugaredLogger) (bool, error) {
	cfg, err := newDBConfig(chartValues)
	if err != nil {
		return false, err
	}

	newConfig := cfg.updateSecretData(secret, logger)

	if len(newConfig) > 0 {
		secret.StringData = newConfig
		return true, nil
	}

	return false, nil
}

func (c *Config) updateSecretData(secret *v1.Secret, logger *zap.SugaredLogger) map[string]string {
	data := make(map[string]string)
	dsn := string(secret.Data["dsn"])

	c.Global.Ory.Hydra.Persistence.SecretsSystem = string(secret.Data["secretsSystem"])
	if c.Global.Ory.Hydra.Persistence.SecretsSystem == "" {
		c.Global.Ory.Hydra.Persistence.SecretsSystem = generateRandomString(32)
	}

	if !c.Global.Ory.Hydra.Persistence.Enabled {
		if dsn == "memory" {
			logger.Info("Ory Hydra persistence is already disabled")
			return data
		}

		logger.Info("Disabling persistence for Ory Hydra")
		return c.generateSecretDataMemory()
	}

	if c.Global.Ory.Hydra.Persistence.PostgresqlFlag.Enabled {
		secretPostgresqlPassword := string(secret.Data["postgresql-password"])
		if secretPostgresqlPassword != "" && c.Global.PostgresCfg.Password == "" {
			c.Global.PostgresCfg.Password = secretPostgresqlPassword
		}
		if c.preparePostgresDSN() != dsn {
			logger.Info("Enabling postgresql persistence")
			return c.generateSecretDataPostgresql()
		}
		return data
	}

	if c.Global.Ory.Hydra.Persistence.DBType == "mysql" {
		if c.prepareMySQLDSN() != dsn {
			logger.Info("Enabling mysql persistence")
			return c.generateSecretDataMysql()
		}
		return data
	}

	if c.Global.Ory.Hydra.Persistence.Gcloud.Enabled {
		if c.prepareGenericDSN() != dsn {
			logger.Info("Enabling gcloud persistence")
			return c.generateSecretDataGcloud()
		}
		return data
	}

	if c.prepareGenericDSN() != dsn {
		logger.Info("Enabling custom db persistence")
		return c.generateSecretDataGeneric()
	}

	return data
}

func (c *Config) preparePostgresDSN() string {
	return fmt.Sprintf(dsnTemplate, "postgres", c.Global.PostgresCfg.User,
		c.Global.PostgresCfg.Password, postgresURL, c.Global.PostgresCfg.DBName, dsnOpts)
}

func (c *Config) prepareMySQLDSN() string {
	dbURL := fmt.Sprintf("tcp(%s)", c.Global.Ory.Hydra.Persistence.URL)
	return fmt.Sprintf(dsnTemplate, c.Global.Ory.Hydra.Persistence.DBType,
		c.Global.Ory.Hydra.Persistence.Username, c.Global.Ory.Hydra.Persistence.Password,
		dbURL, c.Global.Ory.Hydra.Persistence.DBName, mysqlDsnOpts)
}

func (c *Config) prepareGenericDSN() string {
	return fmt.Sprintf(dsnTemplate, c.Global.Ory.Hydra.Persistence.DBType,
		c.Global.Ory.Hydra.Persistence.Username, c.Global.Ory.Hydra.Persistence.Password,
		c.Global.Ory.Hydra.Persistence.URL, c.Global.Ory.Hydra.Persistence.DBName, dsnOpts)
}

func (c *Config) prepareStringData() map[string]string {
	c.Global.Ory.Hydra.Persistence.SecretsSystem = generateRandomString(32)

	if c.Global.Ory.Hydra.Persistence.Enabled {
		if c.Global.Ory.Hydra.Persistence.PostgresqlFlag.Enabled {
			return c.generateSecretDataPostgresql()
		}
		if c.Global.Ory.Hydra.Persistence.DBType == "mysql" {
			return c.generateSecretDataMysql()
		}
		if c.Global.Ory.Hydra.Persistence.Gcloud.Enabled {
			return c.generateSecretDataGcloud()
		}
		return c.generateSecretDataGeneric()
	}

	return c.generateSecretDataMemory()
}

func (c *Config) generateSecretDataMemory() map[string]string {
	return map[string]string{
		"secretsSystem": c.Global.Ory.Hydra.Persistence.SecretsSystem,
		"secretsCookie": generateRandomString(32),
		"dsn":           "memory",
	}
}

func (c *Config) generateSecretDataPostgresql() map[string]string {
	if c.Global.PostgresCfg.Password == "" {
		c.Global.PostgresCfg.Password = generateRandomString(10)
	}

	return map[string]string{
		"secretsSystem":                   c.Global.Ory.Hydra.Persistence.SecretsSystem,
		"secretsCookie":                   generateRandomString(32),
		"dsn":                             c.preparePostgresDSN(),
		"postgresql-password":             c.Global.PostgresCfg.Password,
		"postgresql-replication-password": generateRandomString(10),
	}
}

func (c *Config) generateSecretDataMysql() map[string]string {
	return map[string]string{
		"secretsSystem": c.Global.Ory.Hydra.Persistence.SecretsSystem,
		"secretsCookie": generateRandomString(32),
		"dsn":           c.prepareMySQLDSN(),
		"dbPassword":    c.Global.Ory.Hydra.Persistence.Password,
	}
}

func (c *Config) generateSecretDataGcloud() map[string]string {
	return map[string]string{
		"secretsSystem": c.Global.Ory.Hydra.Persistence.SecretsSystem,
		"secretsCookie": generateRandomString(32),
		"gcp-sa.json":   c.Global.Ory.Hydra.Persistence.Gcloud.SAJson,
		"dsn":           c.prepareGenericDSN(),
	}
}

func (c *Config) generateSecretDataGeneric() map[string]string {
	return map[string]string{
		"secretsSystem": c.Global.Ory.Hydra.Persistence.SecretsSystem,
		"secretsCookie": generateRandomString(32),
		"dsn":           c.prepareGenericDSN(),
		"dbPassword":    c.Global.Ory.Hydra.Persistence.Password,
	}
}

func generateRandomString(length int) string {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return ""
	}

	return base64.URLEncoding.EncodeToString(b)
}
