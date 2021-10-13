ALTER TABLE inventory_clusters RENAME COLUMN "runtime_id" to "cluster";

ALTER TABLE inventory_cluster_configs RENAME COLUMN "runtime_id" to "cluster";

ALTER TABLE inventory_cluster_config_statuses RENAME COLUMN "runtime_id" to "cluster";

ALTER TABLE config_cache RENAME COLUMN "runtime_id" to "cluster";

ALTER TABLE config_cachedeps RENAME COLUMN "runtime_id" to "cluster";

ALTER TABLE scheduler_reconciliations RENAME COLUMN "runtime_id" to "cluster";

ALTER TABLE scheduler_operations RENAME COLUMN "runtime_id" to "cluster";
