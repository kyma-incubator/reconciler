package db

import (
	"crypto/rand"
	"encoding/base64"

	"fmt"

	"github.com/pkg/errors"
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

func new(chartValues map[string]interface{}) (*Config, error) {
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

// Get creates Kubernetes Secret object matching the provided helm values.
func Get(name types.NamespacedName, values map[string]interface{}) (*v1.Secret, error) {
	cfg, err := new(values)
	if err != nil {
		return nil, err
	}

	if cfg.Global.PostgresCfg.Password == "" {
		cfg.Global.PostgresCfg.Password = generateRandomString(10)
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

func (c *Config) preparePostgresDSN() string {
	return fmt.Sprintf(dsnTemplate, "postgres", c.Global.PostgresCfg.User, c.Global.PostgresCfg.Password, postgresURL, c.Global.PostgresCfg.DBName, dsnOpts)
}

func (c *Config) prepareMySQLDSN() string {
	dbURL := fmt.Sprintf("tcp(%s)", c.Global.Ory.Hydra.Persistence.URL)
	return fmt.Sprintf(dsnTemplate, c.Global.Ory.Hydra.Persistence.DBType, c.Global.Ory.Hydra.Persistence.Username, c.Global.Ory.Hydra.Persistence.Password, dbURL, c.Global.Ory.Hydra.Persistence.DBName, mysqlDsnOpts)
}

func (c *Config) prepareGenericDSN() string {
	return fmt.Sprintf(dsnTemplate, c.Global.Ory.Hydra.Persistence.DBType, c.Global.Ory.Hydra.Persistence.Username, c.Global.Ory.Hydra.Persistence.Password, c.Global.Ory.Hydra.Persistence.URL, c.Global.Ory.Hydra.Persistence.DBName, dsnOpts)
}

func (c *Config) prepareStringData() map[string]string {
	if c.Global.Ory.Hydra.Persistence.Enabled {
		if c.Global.Ory.Hydra.Persistence.PostgresqlFlag.Enabled {
			return map[string]string{"secretsSystem": generateRandomString(32), "secretsCookie": generateRandomString(32), "dsn": c.preparePostgresDSN(), "postgresql-password": c.Global.PostgresCfg.Password, "postgresql-replication-password": generateRandomString(10)}
		}
		if c.Global.Ory.Hydra.Persistence.DBType == "mysql" {
			return map[string]string{"secretsSystem": generateRandomString(32), "secretsCookie": generateRandomString(32), "dsn": c.prepareMySQLDSN(), "dbPassword": c.Global.Ory.Hydra.Persistence.Password}
		}
		if c.Global.Ory.Hydra.Persistence.Gcloud.Enabled {
			return map[string]string{"secretsSystem": generateRandomString(32), "secretsCookie": generateRandomString(32), "gcp-sa.json": c.Global.Ory.Hydra.Persistence.Gcloud.SAJson, "dsn": c.prepareGenericDSN()}
		}
		return map[string]string{"secretsSystem": generateRandomString(32), "secretsCookie": generateRandomString(32), "dsn": c.prepareGenericDSN(), "dbPassword": c.Global.Ory.Hydra.Persistence.Password}
	}

	return map[string]string{"secretsSystem": generateRandomString(32), "secretsCookie": generateRandomString(32), "dsn": "memory"}
}

func generateRandomString(length int) string {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return ""
	}

	return base64.URLEncoding.EncodeToString(b)
}
