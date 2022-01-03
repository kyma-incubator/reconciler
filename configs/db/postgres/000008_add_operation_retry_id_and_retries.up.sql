ALTER TABLE scheduler_operations
    ADD COLUMN "retry_id" VARCHAR(255);

do $$
declare
  tmp record;
begin
    FOR tmp in (SELECT scheduling_id, correlation_id FROM scheduler_operations) LOOP
        UPDATE scheduler_operations SET retry_id = CONCAT(scheduling_id, correlation_id)
            where scheduling_id = tmp.scheduling_id AND correlation_id = tmp.correlation_id;
    END LOOP;
end $$;

ALTER TABLE scheduler_operations
    ALTER COLUMN "retry_id" SET NOT NULL;

ALTER TABLE scheduler_operations
    ADD COLUMN "retries" int;

UPDATE scheduler_operations
SET retries = 0;

ALTER TABLE scheduler_operations
    ALTER COLUMN "retries" SET NOT NULL;
    