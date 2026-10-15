package migrations

import "InariBlueNotifier/config"

type Migration interface {
	Version() uint
	Apply(cfg *config.Config) error
}
