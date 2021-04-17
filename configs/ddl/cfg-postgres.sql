--CONFIGURATION MANAGEMENT

--DDL for configuration key entities:
CREATE TABLE cfg_keys (
	"key" text NOT NULL,
	"version" SERIAL UNIQUE,
	"datatype" varchar(255) NOT NULL,
	"encrypted" boolean DEFAULT FALSE,
	"created" TIMESTAMP WITHOUT TIME ZONE DEFAULT (NOW() AT TIME ZONE 'utc'),
	"user" varchar(255) NOT NULL,
	CONSTRAINT cfg_keys_pk PRIMARY KEY ("key", "version")
);

--DDL for configuration value entities:
CREATE TABLE cfg_values (
	"key" text NOT NULL,
	"key_version" integer NOT NULL,
	"version" SERIAL UNIQUE,
	"bucket" text NOT NULL,
	"value" text NULL,
	"created" TIMESTAMP WITHOUT TIME ZONE DEFAULT (NOW() AT TIME ZONE 'utc'),
	"user" varchar(255) NOT NULL,
	CONSTRAINT cfg_values_pk PRIMARY KEY ("bucket", "key", "version"),
	FOREIGN KEY ("key", "key_version") REFERENCES cfg_keys ("key", "version"),
);

