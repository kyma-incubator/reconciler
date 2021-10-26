package db

// Config holds the database configuration values of Ory Hydra.
type Config struct {
	Global Global
}

// Global configuration of Ory Hydra and PostgresSQL
type Global struct {
	PostgresCfg PostgresCfg `yaml:"postgresql"`
	Ory         Ory         `yaml:"ory"`
}

// PostgresSQL specific values like username, default database name and password.
type PostgresCfg struct {
	User                string `yaml:"postgresqlUsername"`
	DBName              string `yaml:"postgresqlDatabase"`
	Password            string `yaml:"postgresqlPassword"`
	ReplicationPassword string `yaml:"replicationPassword"`
}

// Ory specific values
type Ory struct {
	Hydra Hydra `yaml:"hydra"`
}

// Ory Hydra specific values
type Hydra struct {
	Persistence Persistence `yaml:"persistence"`
}

// Ory Hydra persistence configuration values
type Persistence struct {
	Enabled        bool           `yaml:"enabled"`
	PostgresqlFlag PostgresqlFlag `yaml:"postgresql"`
	Gcloud         Gcloud         `yaml:"gcloud"`
	DBType         string         `yaml:"dbType"`
	Password       string         `yaml:"password"`
	Username       string         `yaml:"user"`
	URL            string         `yaml:"dbUrl"`
	DBName         string         `yaml:"dbName"`
	SecretsSystem  string         `yaml:"secretsSystem"`
	SecretsCookie  string         `yaml:"secretsCookie"`
}

// PostgresqlFlag is a boolean to control whether PostgresSQL needs to be deployed
type PostgresqlFlag struct {
	Enabled bool `yaml:"enabled"`
}

// Gcloud contains a boolean to control whether Google Cloud SQL is used and a JSON containing Service Account.
type Gcloud struct {
	Enabled bool   `yaml:"enabled"`
	SAJson  string `yaml:"saJson"`
}
