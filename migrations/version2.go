package migrations

import (
	"InariBlueNotifier/config"
	"InariBlueNotifier/patterns"
)

type Version2 struct{}

func (receiver *Version2) Version() uint { return 2 }

func (receiver *Version2) Apply(cfg *config.Config) error {
	if _, ok := cfg.StoredPatterns[patterns.Steady.Name]; !ok {
		cfg.StoredPatterns[patterns.Steady.Name] = patterns.Steady
	}
	return nil
}

func init() {
	instance.migrations = append(instance.migrations, &Version2{})
}
