--CONFIGURATION MANAGEMENT

--DDL for configuration key entities:
CREATE TABLE config_keys (
	"version" integer PRIMARY KEY AUTOINCREMENT,
	"key" text NOT NULL,
	"data_type" varchar(255) NOT NULL,
	"encrypted" boolean DEFAULT FALSE,
	"username" varchar(255) NOT NULL,
	"trigger" text,
	"validator" text,
	"created" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT config_keys_pk UNIQUE ("key", "version")
);

--DDL for configuration value entities:
CREATE TABLE config_values (
	"version" integer PRIMARY KEY AUTOINCREMENT,
	"key" text NOT NULL,
	"key_version" integer NOT NULL,
	"bucket" text NOT NULL,
	"data_type" varchar(255) NOT NULL,
	"value" text NULL,
	"username" varchar(255) NOT NULL,
	"created" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT config_values_pk UNIQUE ("bucket", "key", "version"),
	FOREIGN KEY ("key", "key_version") REFERENCES config_keys ("key", "version") ON UPDATE CASCADE ON DELETE CASCADE
);

--DDL for configuration cache-entry entities:
CREATE TABLE config_cache (
	"id" integer PRIMARY KEY AUTOINCREMENT, --just another unique identifer for a cache entry
	"label" text NOT NULL,
	"cluster" text NOT NULL,
	"data" text NOT NULL,
	"checksum" text NOT NULL,
	"created" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT config_cache_pk UNIQUE ("label", "cluster")
);

--DDL for configuration cache-dependency entities:
CREATE TABLE config_cachedeps (
	"id" integer PRIMARY KEY AUTOINCREMENT, --just another unique identifer for a cache entry
	"bucket" text NOT NULL,
	"key" text NOT NULL,
	"label" text NOT NULL,
	"cluster" text NOT NULL,
	"cache_id" integer NOT NULL,
	"created" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT config_cachedep_pk UNIQUE ("bucket", "key", "label", "cluster"),
	FOREIGN KEY ("label", "cluster") REFERENCES config_cache ("label", "cluster") ON DELETE CASCADE
);

CREATE INDEX config_cachedeps_idx_cacheid ON config_cachedeps ("cache_id");

--DDL for cluster inventory:
CREATE TABLE inventory_clusters (
	"version" integer PRIMARY KEY AUTOINCREMENT, --can also be used as unique identifier for a cluster
	"cluster" text NOT NULL,
	"runtime" text NOT NULL,
	"metadata" text NOT NULL,
	"contract" int NOT NULL,
	"deleted" boolean DEFAULT FALSE,
	"created" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT inventory_clusters_pk UNIQUE ("cluster", "version")
);

CREATE TABLE inventory_cluster_configs (
	"version" integer PRIMARY KEY AUTOINCREMENT, --can also be used as unique identifier for a cluster config
	"cluster" text NOT NULL,
	"cluster_version" int NOT NULL,
	"kyma_version" text NOT NULL,
	"kyma_profile" text,
	"components" text,
	"administrators" text,
	"contract" int NOT NULL,
	"deleted" boolean DEFAULT FALSE,
	"created" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT inventory_cluster_configs_pk UNIQUE ("cluster", "cluster_version", "version"),
	FOREIGN KEY("cluster", "cluster_version") REFERENCES inventory_clusters("cluster", "version") ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE TABLE inventory_cluster_config_statuses (
	"id" integer PRIMARY KEY AUTOINCREMENT,
	"cluster" text NOT NULL,
	"cluster_version" int NOT NULL,
	"config_version" int NOT NULL,
	"status" text NOT NULL,
	"created" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY("cluster", "cluster_version", "config_version") REFERENCES inventory_cluster_configs("cluster", "cluster_version", "version") ON DELETE CASCADE
);