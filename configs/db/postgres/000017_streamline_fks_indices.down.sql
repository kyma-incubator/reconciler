---- cluster status table
-- drop fk dependency
ALTER TABLE scheduler_reconciliations
DROP CONSTRAINT scheduler_reconciliations_cluster_config_status_fkey;

-- drop "id" as primary key
ALTER TABLE inventory_cluster_config_statuses
DROP CONSTRAINT inventory_cluster_config_statuses_pkey;

-- add "id" as key
ALTER TABLE inventory_cluster_config_statuses
    ADD UNIQUE (id);

---- scheduler_reconciliations table
-- drop index for cluster status fk constraint
DROP INDEX if EXISTS scheduler_reconciliations_cluster_config_status_idx;

-- add fk to cluster status
ALTER TABLE scheduler_reconciliations
    ADD FOREIGN KEY (cluster_config_status) references inventory_cluster_config_statuses (id);

-- add cluster config fk
ALTER TABLE scheduler_reconciliations
    ADD FOREIGN KEY (cluster_config) references inventory_cluster_configs(version);

---- scheduler_operations table
-- add cluster config fk
ALTER TABLE scheduler_operations
    ADD FOREIGN KEY (cluster_config) references inventory_cluster_configs(version);