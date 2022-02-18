ALTER TABLE scheduler_operations ADD COLUMN "picked_up" TIMESTAMP WITHOUT TIME ZONE DEFAULT (NOW() AT TIME ZONE 'utc');
ALTER TABLE scheduler_operations ADD COLUMN "processing_duration" int;