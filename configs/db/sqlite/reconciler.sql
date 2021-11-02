--CONFIGURATION MANAGEMENT

--DDL for configuration key entities:
CREATE TABLE IF NOT EXISTS config_keys (
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
CREATE TABLE IF NOT EXISTS config_values (
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
CREATE TABLE IF NOT EXISTS config_cache (
	"id" integer PRIMARY KEY AUTOINCREMENT, --just another unique identifer for a cache entry
	"label" text NOT NULL,
	"runtime_id" text NOT NULL,
	"data" text NOT NULL,
	"checksum" text NOT NULL,
	"created" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT config_cache_pk UNIQUE ("label", "runtime_id")
);

--DDL for configuration cache-dependency entities:
CREATE TABLE IF NOT EXISTS config_cachedeps (
	"id" integer PRIMARY KEY AUTOINCREMENT, --just another unique identifer for a cache entry
	"bucket" text NOT NULL,
	"key" text NOT NULL,
	"label" text NOT NULL,
	"runtime_id" text NOT NULL,
	"cache_id" integer NOT NULL,
	"created" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT config_cachedep_pk UNIQUE ("bucket", "key", "label", "runtime_id"),
	FOREIGN KEY ("label", "runtime_id") REFERENCES config_cache ("label", "runtime_id") ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS config_cachedeps_idx_cacheid ON config_cachedeps ("cache_id");

--DDL for cluster inventory:
CREATE TABLE IF NOT EXISTS inventory_clusters (
	"version" integer PRIMARY KEY AUTOINCREMENT, --can also be used as unique identifier for a cluster
	"runtime_id" text NOT NULL,
	"runtime" text NOT NULL,
	"metadata" text NOT NULL,
	"kubeconfig" text NOT NULL,
	"contract" int NOT NULL,
	"deleted" boolean DEFAULT FALSE,
	"created" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT inventory_clusters_pk UNIQUE ("runtime_id", "version")
);

CREATE TABLE IF NOT EXISTS inventory_cluster_configs (
	"version" integer PRIMARY KEY AUTOINCREMENT, --can also be used as unique identifier for a cluster config
	"runtime_id" text NOT NULL,
	"cluster_version" int NOT NULL,
	"kyma_version" text NOT NULL,
	"kyma_profile" text,
	"components" text,
	"administrators" text,
	"contract" int NOT NULL,
	"deleted" boolean DEFAULT FALSE,
	"created" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT inventory_cluster_configs_pk UNIQUE ("runtime_id", "cluster_version", "version"),
	FOREIGN KEY("runtime_id", "cluster_version") REFERENCES inventory_clusters("runtime_id", "version") ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS inventory_cluster_config_statuses (
	"id" integer PRIMARY KEY AUTOINCREMENT,
	"runtime_id" text NOT NULL,
	"cluster_version" int NOT NULL,
	"config_version" int NOT NULL,
	"status" text NOT NULL,
	"deleted" boolean DEFAULT FALSE,
	"created" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY("runtime_id", "cluster_version", "config_version") REFERENCES inventory_cluster_configs("runtime_id", "cluster_version", "version") ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS scheduler_reconciliations (
    "scheduling_id" text NOT NULL PRIMARY KEY,
    "lock" text UNIQUE, --make sure just one cluster can be reconciled at the same time
    "runtime_id" text NOT NULL,
    "cluster_config" int NOT NULL,
    "cluster_config_status" int,
    "finished" boolean DEFAULT FALSE,
    "created" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY("lock") REFERENCES inventory_clusters("runtime_id"),
    FOREIGN KEY("runtime_id") REFERENCES inventory_clusters("runtime_id") ON UPDATE CASCADE,
    FOREIGN KEY("cluster_config") REFERENCES inventory_cluster_configs("version"),
    FOREIGN KEY("cluster_config_status") REFERENCES inventory_cluster_config_statuses("id")
);

--DDL for scheduler operations:
CREATE TABLE IF NOT EXISTS scheduler_operations (
    "priority" int NOT NULL,
    "scheduling_id" text NOT NULL,
    "correlation_id" text NOT NULL,
    "runtime_id" text NOT NULL,
    "cluster_config" int NOT NULL,
    "component" text NOT NULL,
    "type" text NOT NULL,
    "state" text NOT NULL,
    "reason" text,
    "created" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    "updated" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT scheduler_operations_pk UNIQUE ("scheduling_id", "correlation_id"),
    FOREIGN KEY("scheduling_id") REFERENCES scheduler_reconciliations("scheduling_id") ON UPDATE CASCADE ON DELETE CASCADE,
    FOREIGN KEY("runtime_id") REFERENCES inventory_clusters("runtime_id") ON UPDATE CASCADE,
    FOREIGN KEY("cluster_config") REFERENCES inventory_cluster_configs("version")
)
