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
	"value" text NULL,
	"username" varchar(255) NOT NULL,
	"created" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT config_values_pk UNIQUE ("bucket", "key", "version"),
	FOREIGN KEY ("key", "key_version") REFERENCES config_keys ("key", "version")
);

