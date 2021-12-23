ALTER TABLE scheduler_operations DROP COLUMN "retry_id" VARCHAR(255);
ALTER TABLE scheduler_operations DROP COLUMN "retries" int;