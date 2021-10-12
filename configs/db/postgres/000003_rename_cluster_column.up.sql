ALTER TABLE inventory_clusters RENAME COLUMN "cluster" to "runtime_id";

ALTER TABLE inventory_cluster_configs RENAME COLUMN "cluster" to "runtime_id";

ALTER TABLE inventory_cluster_config_statuses RENAME COLUMN "cluster" to "runtime_id";

ALTER TABLE config_cache RENAME COLUMN "cluster" to "runtime_id";

ALTER TABLE config_cachedeps RENAME COLUMN "cluster" to "runtime_id";

ALTER TABLE scheduler_reconciliations RENAME COLUMN "cluster" to "runtime_id";

ALTER TABLE scheduler_operations RENAME COLUMN "cluster" to "runtime_id";
