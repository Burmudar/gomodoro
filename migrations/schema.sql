CREATE TABLE IF NOT EXISTS "schema_migration" (
"version" TEXT NOT NULL
);
CREATE UNIQUE INDEX "schema_migration_version_idx" ON "schema_migration" (version);
CREATE TABLE IF NOT EXISTS "timer_configs" (
"id" TEXT PRIMARY KEY,
"created_at" DATETIME NOT NULL,
"updated_at" DATETIME NOT NULL,
"focus_time" INTEGER NOT NULL DEFAULT '0',
"break_time" INTEGER NOT NULL DEFAULT '0',
"interval" INTEGER NOT NULL DEFAULT '0'
);
CREATE TABLE IF NOT EXISTS "timer_clients" (
"id" TEXT PRIMARY KEY,
"created_at" DATETIME NOT NULL,
"updated_at" DATETIME NOT NULL
);
