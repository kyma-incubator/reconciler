package dsn

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	dsnOpts     = "sslmode=disable&max_conn_lifetime=10s"
	dsnTemplate = "%s://%s:%s@%s/%s?%s"
)

type DBConfig struct {
	Enabled             bool
	Type                string
	Password            string
	Username            string
	URL                 string
	ReplicationPassword string
	DatabaseName        string
}

func GenerateRandomString(length int) string {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		log.Fatalf("failed to generate secret string: %s", err)
	}

	return base64.URLEncoding.EncodeToString(b)
}

func (c *DBConfig) prepareDsn() string {
	return fmt.Sprintf(dsnTemplate, c.Type, c.Username, c.Password, c.URL, c.DatabaseName, dsnOpts)
}

func ReadSecretFromFile(path string) string {
	secret, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalf("unable to read file: %v", err)
	}
	return string(secret)
}

func (c *DBConfig) PrepareData() map[string]string {
	if c.Enabled {
		return map[string]string{"secretsSystem": GenerateRandomString(32), "secretsCookie": GenerateRandomString(32), "dsn": c.prepareDsn(), "postgresql-password": c.Password, "postgresql-replication-password": c.ReplicationPassword}
	}
	return map[string]string{"secretsSystem": GenerateRandomString(32), "secretsCookie": GenerateRandomString(32), "dsn": "memory"}
}

func (c *DBConfig) PrepareSecret(name types.NamespacedName) v1.Secret {

	data := c.PrepareData()
	return v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
		StringData: data,
	}
}
