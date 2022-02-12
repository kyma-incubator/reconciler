--DDL for worker-pool occupancy
CREATE TABLE IF NOT EXISTS worker_pool_occupancy
(
    "worker_pool_id"       varchar(255) NOT NULL,
    "component"            varchar(255) NOT NULL,
    "running_workers"      int          NOT NULL,
    "worker_pool_capacity" int          NOT NULL,
    "created"              TIMESTAMP WITHOUT TIME ZONE DEFAULT (NOW() AT TIME ZONE 'utc'),
    CONSTRAINT worker_pool_occupancy_pk PRIMARY KEY ("worker_pool_id")
);