--DDL for scheduler reconciliations
CREATE TABLE IF NOT EXISTS scheduler_reconciliations (
    "scheduling_id" varchar(255) NOT NULL,
    "lock" varchar(255) UNIQUE, --make sure just one cluster can be reconciled at the same time
    "cluster" varchar(255) NOT NULL,
    "cluster_config" int NOT NULL,
    "cluster_config_status" int,
    "finished" boolean DEFAULT FALSE,
    "created" TIMESTAMP WITHOUT TIME ZONE DEFAULT (NOW() AT TIME ZONE 'utc'),
    "updated" TIMESTAMP WITHOUT TIME ZONE DEFAULT (NOW() AT TIME ZONE 'utc'),
    CONSTRAINT scheduler_reconciliations_pk PRIMARY KEY ("scheduling_id"),
    FOREIGN KEY("cluster_config") REFERENCES inventory_cluster_configs("version"),
    FOREIGN KEY("cluster_config_status") REFERENCES inventory_cluster_config_statuses("id")
);

--DDL for scheduler operations:
CREATE TABLE IF NOT EXISTS scheduler_operations (
    "priority" int NOT NULL,
    "scheduling_id" varchar(255) NOT NULL,
    "correlation_id" varchar(255) NOT NULL,
    "cluster" varchar(255) NOT NULL,
    "cluster_config" int NOT NULL,
    "component" varchar(255) NOT NULL,
    "state" varchar(255) NOT NULL,
    "reason" text,
    "created" TIMESTAMP WITHOUT TIME ZONE DEFAULT (NOW() AT TIME ZONE 'utc'),
    "updated" TIMESTAMP WITHOUT TIME ZONE DEFAULT (NOW() AT TIME ZONE 'utc'),
    CONSTRAINT scheduler_operations_pk PRIMARY KEY ("scheduling_id", "correlation_id"),
    FOREIGN KEY("scheduling_id") REFERENCES scheduler_reconciliations("scheduling_id") ON UPDATE CASCADE ON DELETE CASCADE,
    FOREIGN KEY("cluster_config") REFERENCES inventory_cluster_configs("version")
);