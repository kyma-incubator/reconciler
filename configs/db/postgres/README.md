# Please hold DDLs in sync

Be aware that Postgres is used for productive purposes, but SQLite is used in unit tests.
If you change the DDL for Postgres, reflect the changes also in the SQLite DDL files:

`../sqlite/configuration-management.sql`

**Thank you! :)**
