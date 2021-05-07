# Please hold DDLs in sync

Be aware that Postgres is used for productive purposes. But unit-tests are using SQLite.
If you change the DDL for Postgres, please reflect the changes also in the SQLite DDL files:

`$reconciler/pkg/config/test/configuration-management.sql`

**Thanks you! :)**