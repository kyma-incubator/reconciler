---- cluster status table
-- add "id" a primary key
ALTER TABLE inventory_cluster_config_statuses
    ADD PRIMARY KEY (id);

-- add foreign key dependency back
ALTER TABLE scheduler_reconciliations
    ADD FOREIGN KEY (cluster_config_status)  references inventory_cluster_config_statuses (id);

---- scheduler_reconciliations table
-- create index for cluster status fk constraint
CREATE INDEX ON scheduler_reconciliations
    USING btree(cluster_config_status);

-- add cascade delete / update from cluster status
ALTER TABLE scheduler_reconciliations
    ADD FOREIGN KEY (cluster_config_status) references inventory_cluster_config_statuses (id)
        ON UPDATE cascade ON DELETE cascade;