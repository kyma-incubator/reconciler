ALTER TABLE inventory_cluster_config_statuses ADD COLUMN "deleted" boolean DEFAULT FALSE;

UPDATE inventory_cluster_config_statuses SET deleted=TRUE WHERE runtime_id IN (SELECT runtime_id FROM inventory_clusters WHERE deleted=TRUE)