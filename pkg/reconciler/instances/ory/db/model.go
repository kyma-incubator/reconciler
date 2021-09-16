package db

// DBConfig holds the persistence configuration of Ory Hydra.
type Config struct {
	Global Global
}
type Global struct {
	PostgresCfg PostgresCfg `yaml:"postgresql"`
	Ory         Ory         `yaml:"ory"`
}

type PostgresCfg struct {
	User     string `yaml:"postgresqlUsername"`
	DBName   string `yaml:"postgresqlDatabase"`
	Password string `yaml:"postgresqlPassword"`
}
type Ory struct {
	Hydra Hydra `yaml:"hydra"`
}
type Hydra struct {
	Persistence Persistence `yaml:"persistence"`
}
type Persistence struct {
	Enabled        bool           `yaml:"enabled"`
	PostgresqlFlag PostgresqlFlag `yaml:"postgresql"`
	Gcloud         Gcloud         `yaml:"gcloud"`
	DBType         string         `yaml:"dbType"`
	Password       string         `yaml:"password"`
	Username       string         `yaml:"user"`
	URL            string         `yaml:"dbUrl"`
	DBName         string         `yaml:"dbName"`
}
type PostgresqlFlag struct {
	Enabled bool `yaml:"enabled"`
}

type Gcloud struct {
	Enabled bool   `yaml:"enabled"`
	SAJson  string `yaml:"saJson"`
}
