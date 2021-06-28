--CONFIGURATION MANAGEMENT

--DDL for configuration key entities:
CREATE TABLE config_keys (
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
CREATE TABLE config_values (
	"version" SERIAL UNIQUE, --can also be used as unique identifier for a config value
	"key" text NOT NULL,
	"key_version" integer NOT NULL,
	"bucket" text NOT NULL,
	"data_type" varchar(255) NOT NULL,
	"value" text NULL,
	"username" varchar(255) NOT NULL,
	"created" TIMESTAMP WITHOUT TIME ZONE DEFAULT (NOW() AT TIME ZONE 'utc'),
	CONSTRAINT config_values_pk PRIMARY KEY ("bucket", "key", "version"),
	FOREIGN KEY ("key", "key_version") REFERENCES config_keys ("key", "version")
);

--DDL for configuration cache-entry entities:
CREATE TABLE config_cache (
	"id" SERIAL UNIQUE, --just another unique identifer for a cache-entry
	"label" text NOT NULL,
	"cluster" text NOT NULL,
	"data" text NOT NULL,
	"checksum" text NOT NULL,
	"created" TIMESTAMP WITHOUT TIME ZONE DEFAULT (NOW() AT TIME ZONE 'utc'),
	CONSTRAINT config_cache_pk PRIMARY KEY ("label", "cluster")
);

--DDL for configuration cache-dependency entities:
CREATE TABLE config_cachedeps (
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

CREATE INDEX config_cachedeps_idx_cacheid ON config_cachedeps ("cache_id");

--DDL for clusters:
CREATE TABLE clusters (
     id uuid PRIMARY KEY CHECK (id <> '00000000-0000-0000-0000-000000000000'),
	"cluster" text NOT NULL,
	"runtime_name" text,
	"runtime_description" text,
    "global_account_id" text,
    "sub_account_id"    text,
    "service_id"       text,
    "service_plan_id"   text,
    "shoot_name"       text,
    "instance_id"      text,
	"created" TIMESTAMP WITHOUT TIME ZONE DEFAULT (NOW() AT TIME ZONE 'utc')
);

CREATE TABLE configurations (
     id uuid PRIMARY KEY CHECK (id <> '00000000-0000-0000-0000-000000000000'),
    "cluster_id" uuid NOT NULL,
    "kyma_version" text,
    "kyma_profile" text,
    "created" TIMESTAMP WITHOUT TIME ZONE DEFAULT (NOW() AT TIME ZONE 'utc'),
    CONSTRAINT fk_clusters FOREIGN KEY(cluster_id) REFERENCES clusters(id) ON DELETE CASCADE
);

CREATE TABLE components (
      "id" uuid PRIMARY KEY CHECK (id <> '00000000-0000-0000-0000-000000000000'),
      "configuration_id" uuid NOT NULL,
      "component" text ,
      "namespace" text ,
      "created" TIMESTAMP WITHOUT TIME ZONE DEFAULT (NOW() AT TIME ZONE 'utc'),
      CONSTRAINT fk_configurations FOREIGN KEY(configuration_id) REFERENCES configurations(id) ON DELETE CASCADE
);

CREATE TABLE component_configurations (
    "id" uuid PRIMARY KEY CHECK (id <> '00000000-0000-0000-0000-000000000000'),
    "component_id" uuid NOT NULL,
    "key" text ,
    "value" text ,
    "secret" boolean ,
    "created" TIMESTAMP WITHOUT TIME ZONE DEFAULT (NOW() AT TIME ZONE 'utc'),
    CONSTRAINT fk_components FOREIGN KEY(component_id) REFERENCES components(id) ON DELETE CASCADE
);

CREATE TABLE cluster_administrators
(
    "id" uuid PRIMARY KEY CHECK (id <> '00000000-0000-0000-0000-000000000000'),
    "configuration_id" uuid NOT NULL,
    user_id text NOT NULL,
    "created" TIMESTAMP WITHOUT TIME ZONE DEFAULT (NOW() AT TIME ZONE 'utc'),
    CONSTRAINT fk_configurations FOREIGN KEY(configuration_id) REFERENCES configurations(id) ON DELETE CASCADE
);

CREATE TABLE statuses
(
    "id" uuid PRIMARY KEY CHECK (id <> '00000000-0000-0000-0000-000000000000'),
    "configuration_id" uuid NOT NULL,
    "status" text NOT NULL,
    "created" TIMESTAMP WITHOUT TIME ZONE DEFAULT (NOW() AT TIME ZONE 'utc'),
    CONSTRAINT fk_configurations FOREIGN KEY(configuration_id) REFERENCES configurations(id) ON DELETE CASCADE
);