# Please hold DDLs in sync

Be aware that Postgres is used for productive purposes, but unit tests use SQLite.
If you change the DDL for Postgres, reflect the changes also in the SQLite DDL files:

`$reconciler/pkg/config/test/configuration-management.sql`

**Thank you! :)**
