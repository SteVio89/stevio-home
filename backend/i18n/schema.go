package i18n

import "embed"

// I18nSchema contains CREATE TABLE statements for locales, locale_translations,
// entity_translations, and mail_templates.
// Apps embed this into their own migration pipeline.
//
//go:embed migrations/001_i18n.sql
var I18nSchema string

// MigrationFiles is the embedded FS for use with migrate.RunMigrations.
// Apps can use this directly:
//
//	migrate.RunMigrations(db, "i18n", i18n.MigrationFiles, logger)
//
//go:embed migrations/*.sql
var MigrationFiles embed.FS
