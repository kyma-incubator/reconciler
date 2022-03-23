----------------------view cluster cleanup
CREATE OR replace VIEW v_inventory_cluster_cleanup AS

-- deleted status more than 20 days old
SELECT status.id, status.runtime_id, status.cluster_version, status.config_version, status.status, status.created
FROM inventory_cluster_config_statuses AS status
WHERE deleted = true AND created < ( now() at time zone 'utc'- interval '20 DAY' )

UNION

-- deleted cluster versions more than 20 days old (required to keep scheduler_reconciliations -> scheduler_operations clean, foreign key constraint)
SELECT status.id, status.runtime_id, status.cluster_version, status.config_version, status.status, status.created
FROM inventory_cluster_config_statuses AS status
    JOIN inventory_clusters ic ON status.runtime_id = ic.runtime_id AND status.cluster_version = ic.version
WHERE ic.deleted = true AND ic.created < ( now() at time zone 'utc'- interval '20 DAY' );
---------------------------end view