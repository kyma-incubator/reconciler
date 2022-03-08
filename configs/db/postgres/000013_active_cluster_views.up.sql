CREATE OR REPLACE VIEW v_active_inventory_cluster_configs
AS
SELECT max(icc.version)         AS config_id,
       max(icc.cluster_version) AS cluster_id,
       icc.runtime_id
FROM inventory_cluster_configs icc
WHERE icc.deleted = false
GROUP BY icc.runtime_id;


-- noinspection SqlResolve @ column/"config_id"

CREATE OR REPLACE VIEW v_active_inventory_cluster_config_status_history
AS
SELECT DISTINCT ON (config_id) -- if this shows an error for you, its not a problem, config_id is defined below
                               id              AS status_id,
                               config_version  AS config_id,
                               cluster_version AS cluster_id,
                               status,
                               created         AS status_created_at
FROM inventory_cluster_config_statuses
WHERE deleted = false
GROUP BY status_id, cluster_id, config_id, status, status_created_at
ORDER BY config_id, status_id DESC;

CREATE OR REPLACE VIEW v_active_inventory_cluster_latest_status
AS
SELECT vaicc.config_id,
       vaicc.cluster_id,
       vaicc.runtime_id,
       vaiccsh.status_id,
       vaiccsh.status,
       vaiccsh.status_created_at
FROM v_active_inventory_cluster_configs vaicc
         JOIN v_active_inventory_cluster_config_status_history vaiccsh
              ON vaicc.config_id = vaiccsh.config_id;

CREATE OR REPLACE VIEW v_active_inventory_cluster_latest_status_details
AS
SELECT latest_status.config_id,
       latest_status.cluster_id,
       latest_status.runtime_id,
       latest_status.status_id,
       latest_status.status,
       latest_status.status_created_at,
       icc.kyma_version,
       icc.kyma_profile,
       icc.components,
       icc.administrators,
       icc.created as config_created_at,
       ic.runtime,
       ic.metadata,
       ic.kubeconfig,
       ic.contract,
       ic.created  as cluster_created_at
FROM v_active_inventory_cluster_latest_status latest_status
         JOIN inventory_cluster_configs icc
              ON
                          latest_status.cluster_id = icc.cluster_version
                      AND latest_status.config_id = icc."version"
         JOIN inventory_clusters ic
              ON latest_status.cluster_id = ic."version";