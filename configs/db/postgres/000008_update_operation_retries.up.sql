ALTER TABLE scheduler_operations
    ADD COLUMN "retries" int;

UPDATE scheduler_operations SET retries = -1;

ALTER TABLE scheduler_operations ALTER COLUMN "retries" SET NOT NULL;