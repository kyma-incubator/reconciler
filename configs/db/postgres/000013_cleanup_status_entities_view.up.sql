----------------------view status cleanup
CREATE OR replace VIEW v_inventory_status_cleanup AS

-- latest non-deleted status for non-deleted cluster configs
WITH t_active_status AS (
    SELECT icss.config_version AS cluster_config_id, MAX(icss.id) AS status_id
    FROM inventory_cluster_config_statuses icss
             JOIN inventory_cluster_configs icc ON icss.config_version = icc.version
    WHERE icss.deleted = false AND icc.deleted = false
    GROUP BY icss.config_version
)

-- no longer referenced by a reconciliation entity
SELECT status.id, status.runtime_id, status.cluster_version, status.config_version, status.status, status.created
FROM inventory_cluster_config_statuses status
         LEFT OUTER JOIN scheduler_reconciliations sr ON status.id = sr.cluster_config_status
WHERE sr.cluster_config_status IS NULL
-- not referenced by an active status
  AND status.id NOT IN (SELECT status_id FROM t_active_status);

---------------------------end view