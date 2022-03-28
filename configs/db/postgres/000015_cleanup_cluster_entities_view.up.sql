----------------------view cluster cleanup
CREATE OR replace VIEW v_inventory_cluster_cleanup AS
-- deleted cluster versions more than X days old (view required to keep scheduler_reconciliations -> scheduler_operations clean, foreign key constraint)
select status.id as status_id, status.runtime_id, status.cluster_version as cluster_id, status.config_version as config_id, status.status, status.created
FROM inventory_cluster_config_statuses AS status
    JOIN inventory_clusters ic ON status.runtime_id = ic.runtime_id AND status.cluster_version = ic.version
WHERE status.deleted = true AND ic.deleted = true;
---------------------------end view