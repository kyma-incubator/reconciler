---- cluster status table
-- drop fk dependency
ALTER TABLE scheduler_reconciliations
    DROP CONSTRAINT scheduler_reconciliations_cluster_config_status_fkey;

-- drop "id" as key
ALTER SEQUENCE inventory_cluster_config_statuses_id_seq AS INTEGER ;

ALTER TABLE inventory_cluster_config_statuses
    DROP CONSTRAINT inventory_cluster_config_statuses_id_key;

-- add "id" a primary key
ALTER TABLE inventory_cluster_config_statuses
    ADD PRIMARY KEY (id);

---- scheduler_reconciliations table
-- create index for cluster status fk constraint
CREATE INDEX ON scheduler_reconciliations
    USING btree(cluster_config_status);

-- add cascade delete / update fk to cluster status
ALTER TABLE scheduler_reconciliations
    ADD FOREIGN KEY (cluster_config_status) references inventory_cluster_config_statuses (id)
        ON UPDATE cascade ON DELETE cascade;

-- drop cluster config fk
ALTER TABLE scheduler_reconciliations
    DROP CONSTRAINT scheduler_reconciliations_cluster_config_fkey;

---- scheduler_operations table
ALTER TABLE scheduler_operations
    DROP CONSTRAINT scheduler_operations_cluster_config_fkey;