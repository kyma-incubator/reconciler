---- cluster status table
-- drop fk dependency
ALTER TABLE scheduler_reconciliations
    DROP CONSTRAINT scheduler_reconciliations_cluster_config_status_fkey;

-- drop "id" as key
ALTER SEQUENCE inventory_cluster_config_statuses_id_seq AS INTEGER ;

ALTER TABLE inventory_cluster_config_statuses
    DROP CONSTRAINT inventory_cluster_config_statuses_id_key;

---- scheduler_reconciliations table
-- drop cluster config fk
ALTER TABLE scheduler_reconciliations
    DROP CONSTRAINT scheduler_reconciliations_cluster_config_fkey;

-- drop fk to cluster status
ALTER TABLE scheduler_reconciliations
    DROP CONSTRAINT scheduler_reconciliations_cluster_config_status_fkey;

---- scheduler_operations table
ALTER TABLE scheduler_operations
    DROP CONSTRAINT scheduler_operations_cluster_config_fkey;