--CONFIGURATION MANAGEMENT

--DDL for configuration key entities:
CREATE TABLE config_keys (
	"version" SERIAL UNIQUE,
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
	"version" SERIAL UNIQUE,
	"key" text NOT NULL,
	"key_version" integer NOT NULL,
	"bucket" text NOT NULL,
	"value" text NULL,
	"username" varchar(255) NOT NULL,
	"created" TIMESTAMP WITHOUT TIME ZONE DEFAULT (NOW() AT TIME ZONE 'utc'),
	CONSTRAINT config_values_pk PRIMARY KEY ("bucket", "key", "version"),
	FOREIGN KEY ("key", "key_version") REFERENCES config_keys ("key", "version")
);

--DDL for configuration cache-entry entities:
CREATE TABLE config_cache (
	"cache_id" text NOT NULL,
	"cluster" text NOT NULL,
	"buckets" text NOT NULL, --additional information just for better traceability
	"data" text NOT NULL,
	"checksum" text NOT NULL,
	"created" TIMESTAMP WITHOUT TIME ZONE DEFAULT (NOW() AT TIME ZONE 'utc'),
	CONSTRAINT config_cache_pk PRIMARY KEY ("cache_id", "cluster")
);

--DDL for configuration cache-dependency entities:
CREATE TABLE config_cachedeps (
	"bucket" text NOT NULL,
	"key" text NOT NULL,
	"cache_id" text NOT NULL,
	"cluster" text NOT NULL,
	"created" TIMESTAMP WITHOUT TIME ZONE DEFAULT (NOW() AT TIME ZONE 'utc'),
	CONSTRAINT config_cachedep_pk PRIMARY KEY ("bucket", "key", "cache_id", "cluster"),
	FOREIGN KEY ("cache_id", "cluster") REFERENCES config_cache ("cache_id", "cluster") ON DELETE CASCADE
);
