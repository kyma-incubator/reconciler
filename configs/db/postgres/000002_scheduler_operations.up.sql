--DDL for scheduler operations:
CREATE TABLE IF NOT EXISTS scheduler_operations (
    "scheduling_id" uuid NOT NULL,
    "correlation_id" uuid NOT NULL,
    "config_version" int NOT NULL,
    "component" varchar(255) NOT NULL,
    "state" varchar(255) NOT NULL,
    "reason" text,
    "created" TIMESTAMP WITHOUT TIME ZONE DEFAULT (NOW() AT TIME ZONE 'utc'),
    "updated" TIMESTAMP WITHOUT TIME ZONE DEFAULT (NOW() AT TIME ZONE 'utc'),
    CONSTRAINT scheduler_operations_pk PRIMARY KEY ("scheduling_id", "correlation_id"),
    FOREIGN KEY("config_version") REFERENCES inventory_cluster_configs("version") ON UPDATE CASCADE ON DELETE CASCADE
)