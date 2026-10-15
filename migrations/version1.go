package migrations

import (
	"InariBlueNotifier/config"
	"InariBlueNotifier/patterns"
)

type Version1 struct {
}

func (receiver *Version1) Version() uint {
	return 1
}

func (receiver *Version1) Apply(cfg *config.Config) error {
	if _, ok := cfg.StoredPatterns[patterns.Off.Name]; ok {
		return nil
	}

	cfg.StoredPatterns[patterns.Off.Name] = patterns.Off
	return nil
}

func init() {
	instance.migrations = append(instance.migrations, &Version1{})
}
