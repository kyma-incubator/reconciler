ALTER TABLE scheduler_operations ALTER COLUMN processing_duration SET DEFAULT 0;
UPDATE scheduler_operations SET processing_duration=0 WHERE processing_duration is NULL;
