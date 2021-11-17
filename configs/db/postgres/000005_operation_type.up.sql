ALTER TABLE scheduler_operations ADD COLUMN "type" VARCHAR(255);

UPDATE scheduler_operations SET "type" = 'reconcile';

ALTER TABLE scheduler_operations ALTER COLUMN "type" SET NOT NULL;
