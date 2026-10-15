package migrations

import (
	"InariBlueNotifier/config"
	"fmt"
	"slices"
)

var instance = &Migrator{}

type Migrator struct {
	migrations []Migration
}

func GetMigrator() *Migrator {
	return instance
}

func (migrator *Migrator) Migrate(cfg *config.Config) error {
	changed := false
	for _, migration := range migrator.findVersions(cfg.MigratedVersion, migrator.findLatestVersion()) {
		if err := migration.Apply(cfg); err != nil {
			return fmt.Errorf("failed migrating to %d: %w", migration.Version(), err)
		}
		changed = true
	}

	if changed {
		cfg.MigratedVersion = migrator.findLatestVersion()
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("failed saving config: %w", err)
		}
	}

	return nil
}

func (migrator *Migrator) findLatestVersion() uint {
	var result uint
	for _, migration := range migrator.migrations {
		if migration.Version() > result {
			result = migration.Version()
		}
	}

	return result
}

// findVersions returns all versions between min and max (min exclusive, max inclusive)
func (migrator *Migrator) findVersions(min, max uint) []Migration {
	result := make([]Migration, 0)
	for _, migration := range migrator.migrations {
		if migration.Version() <= min || migration.Version() > max {
			continue
		}
		result = append(result, migration)
	}

	slices.SortFunc(result, func(a, b Migration) int {
		if a.Version() < b.Version() {
			return -1
		}

		return 1
	})

	return result
}
