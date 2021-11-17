ALTER TABLE scheduler_reconciliations ADD COLUMN "status" text;

do $$
declare 
  tmp record;
begin
	FOR tmp in (SELECT r.scheduling_id, s.status from scheduler_reconciliations as r inner join inventory_cluster_config_statuses as s on r.cluster_config_status = s.id) LOOP
	    update scheduler_reconciliations set status=tmp.status where scheduling_id=scheduling_id;
	END LOOP;
end $$;

ALTER TABLE scheduler_reconciliations ALTER COLUMN "status" SET NOT NULL;