--CONFIGURATION MANAGEMENT

--DDL for configuration key entities:
CREATE TABLE IF NOT EXISTS config_keys (
	"version" SERIAL UNIQUE, --can also be used as unique identifier for a config key
	"key" text NOT NULL,
	"data_type" varchar(255) NOT NULL,
	"encrypted" boolean DEFAULT FALSE,
	"username" varchar(255) NOT NULL,
	"trigger" text,
	"validator" text,
	"created" TIMESTAMP WITHOUT TIME ZONE DEFAULT (NOW() AT TIME ZONE 'utc'),
	CONSTRAINT config_keys_pk PRIMARY KEY ("key", "version")
);

--DDL for configuration value entities:
CREATE TABLE IF NOT EXISTS config_values (
	"version" SERIAL UNIQUE, --can also be used as unique identifier for a config value
	"key" text NOT NULL,
	"key_version" integer NOT NULL,
	"bucket" text NOT NULL,
	"data_type" varchar(255) NOT NULL,
	"value" text NULL,
	"username" varchar(255) NOT NULL,
	"created" TIMESTAMP WITHOUT TIME ZONE DEFAULT (NOW() AT TIME ZONE 'utc'),
	CONSTRAINT config_values_pk PRIMARY KEY ("bucket", "key", "version"),
	FOREIGN KEY ("key", "key_version") REFERENCES config_keys ("key", "version") ON UPDATE CASCADE ON DELETE CASCADE
);

--DDL for configuration cache-entry entities:
CREATE TABLE IF NOT EXISTS config_cache (
	"id" SERIAL UNIQUE, --just another unique identifer for a cache-entry
	"label" text NOT NULL,
	"cluster" text NOT NULL,
	"data" text NOT NULL,
	"checksum" text NOT NULL,
	"created" TIMESTAMP WITHOUT TIME ZONE DEFAULT (NOW() AT TIME ZONE 'utc'),
	CONSTRAINT config_cache_pk PRIMARY KEY ("label", "cluster")
);

--DDL for configuration cache-dependency entities:
CREATE TABLE IF NOT EXISTS config_cachedeps (
	"id" SERIAL UNIQUE, --just another unique identifer for a cache-dependency
	"bucket" text NOT NULL,
	"key" text NOT NULL,
	"label" text NOT NULL,
	"cluster" text NOT NULL,
	"cache_id" integer NOT NULL,
	"created" TIMESTAMP WITHOUT TIME ZONE DEFAULT (NOW() AT TIME ZONE 'utc'),
	CONSTRAINT config_cachedep_pk PRIMARY KEY ("bucket", "key", "label", "cluster"),
	FOREIGN KEY ("label", "cluster") REFERENCES config_cache ("label", "cluster") ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS config_cachedeps_idx_cacheid ON config_cachedeps ("cache_id");

--DDL for cluster inventory:
CREATE TABLE IF NOT EXISTS inventory_clusters (
	"version" SERIAL UNIQUE, --can also be used as unique identifier for a cluster
	"cluster" text NOT NULL,
	"runtime" text NOT NULL,
	"metadata" text NOT NULL,
	"kubeconfig" text NOT NULL,
	"contract" int NOT NULL,
	"deleted" boolean DEFAULT FALSE,
	"created" TIMESTAMP WITHOUT TIME ZONE DEFAULT (NOW() AT TIME ZONE 'utc'),
	CONSTRAINT inventory_clusters_pk PRIMARY KEY ("cluster", "version")
);

CREATE TABLE IF NOT EXISTS inventory_cluster_configs (
	"version" SERIAL UNIQUE, --can also be used as unique identifier for a cluster config
	"cluster" text NOT NULL,
	"cluster_version" int NOT NULL,
	"kyma_version" text NOT NULL,
	"kyma_profile" text,
	"components" text,
	"administrators" text,
	"contract" int NOT NULL,
	"deleted" boolean DEFAULT FALSE,
	"created" TIMESTAMP WITHOUT TIME ZONE DEFAULT (NOW() AT TIME ZONE 'utc'),
	CONSTRAINT inventory_cluster_configs_pk PRIMARY KEY ("cluster", "cluster_version", "version"),
	FOREIGN KEY("cluster", "cluster_version") REFERENCES inventory_clusters("cluster", "version") ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS inventory_cluster_config_statuses (
	"id" SERIAL UNIQUE,
	"cluster" text NOT NULL,
	"cluster_version" int NOT NULL,
	"config_version" int NOT NULL,
	"status" text NOT NULL,
	"created" TIMESTAMP WITHOUT TIME ZONE DEFAULT (NOW() AT TIME ZONE 'utc'),
	FOREIGN KEY("cluster", "cluster_version", "config_version") REFERENCES inventory_cluster_configs("cluster", "cluster_version", "version") ON UPDATE CASCADE ON DELETE CASCADE
);