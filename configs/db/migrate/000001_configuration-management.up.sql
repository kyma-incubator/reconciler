--CONFIGURATION MANAGEMENT

--DDL for configuration key entities:
CREATE TABLE config_keys (
	"version" SERIAL UNIQUE,
	"key" text NOT NULL,
	"data_type" varchar(255) NOT NULL,
	"encrypted" boolean DEFAULT FALSE,
	"user" varchar(255) NOT NULL,
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
	"user" varchar(255) NOT NULL,
	"created" TIMESTAMP WITHOUT TIME ZONE DEFAULT (NOW() AT TIME ZONE 'utc'),
	CONSTRAINT config_values_pk PRIMARY KEY ("bucket", "key", "version"),
	FOREIGN KEY ("key", "key_version") REFERENCES config_keys ("key", "version")
);

