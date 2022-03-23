CREATE INDEX inventory_cluster_config_statuses__idx_deleted_id_status_created ON "inventory_cluster_config_statuses" ("deleted","id","status","created");
CREATE INDEX inventory_cluster_config_statuses__idx_cluster_version_id ON "inventory_cluster_config_statuses" ("cluster_version","id");
CREATE INDEX inventory_cluster_config_statuses__idx_config_version ON "inventory_cluster_config_statuses" ("config_version");
CREATE INDEX inventory_cluster_configs__idx_deleted_kyma_version_cluster_version_created ON "inventory_cluster_configs" ("deleted","kyma_version","cluster_version","created");
CREATE INDEX inventory_cluster_configs__idx_cluster_version_version ON "inventory_cluster_configs" ("cluster_version","version");
CREATE INDEX inventory_cluster__idx_deleted_runtime_id ON "inventory_clusters" ("deleted", "runtime_id");
CREATE INDEX inventory_cluster__idx_deleted_created ON "inventory_clusters" ("deleted", "created");