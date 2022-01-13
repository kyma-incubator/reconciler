ALTER TABLE scheduler_operations
    ADD COLUMN "retry_id" VARCHAR(255);

UPDATE scheduler_operations
SET retry_id = CONCAT(scheduling_id, correlation_id);

ALTER TABLE scheduler_operations
    ALTER COLUMN "retry_id" SET NOT NULL;

ALTER TABLE scheduler_operations
    ADD COLUMN "retries" int;

UPDATE scheduler_operations
SET retries = 0;

ALTER TABLE scheduler_operations
    ALTER COLUMN "retries" SET NOT NULL;
    