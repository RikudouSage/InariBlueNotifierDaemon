package config

import (
	"InariBlueNotifier/patterns"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/samber/lo"
	"gopkg.in/ini.v1"
)

const (
	patternKeyCall         = "call"
	patternKeyFavoriteCall = "favoriteCall"
	patternKeyNotification = "notification"

	sectionStoredPatterns = "storedPatterns"
	sectionActivePatterns = "activePatterns"
)

type Config struct {
	cfgFilePath string

	StoredPatterns map[string]patterns.Pattern

	CallPattern         patterns.Pattern
	FavoriteCallPattern patterns.Pattern
	NotificationPattern patterns.Pattern
}

func New() (*Config, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		cfgDir, err = os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("could not find config dir or home dir: %w", err)
		}
		cfgDir = filepath.Join(cfgDir, ".config")
	}
	cfgDir = filepath.Join(cfgDir, "dev.rikudou", "inari-blue-notifier")
	cfgFilePath := filepath.Join(cfgDir, "config_v3.ini")
	cfgFile, err := os.Open(cfgFilePath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("could not open config file: %w", err)
	}
	if err == nil {
		defer cfgFile.Close()
	}

	instance := &Config{
		cfgFilePath:    cfgFilePath,
		StoredPatterns: make(map[string]patterns.Pattern),
	}

	changedConfig := false

	var cfgRaw *ini.File
	if cfgFile != nil {
		cfgRaw, err = ini.Load(cfgFile)
	} else {
		cfgRaw, err = ini.Load(make([]byte, 0))
		if err != nil {
			return nil, fmt.Errorf("could not create empty config file: %w", err)
		}

		initialPatterns := []patterns.Pattern{
			patterns.SlowBlink,
			patterns.RapidBlink,
			patterns.MediumLengthBreath,
			patterns.QuickFlash,
			patterns.GentleGlow,
			patterns.DelayedBeacon,
		}

		instance.StoredPatterns = lo.KeyBy(initialPatterns, func(item patterns.Pattern) string {
			return item.Name
		})

		changedConfig = true
	}
	if err != nil {
		return nil, fmt.Errorf("could not load config file: %w", err)
	}
	activePatterns := cfgRaw.Section(sectionActivePatterns)

	parsePattern := func(raw string) (patterns.Pattern, error) {
		decoded, err := base64.RawStdEncoding.DecodeString(raw)
		if err != nil {
			return patterns.Pattern{}, fmt.Errorf("could not base64 decode pattern: %w", err)
		}
		var result patterns.Pattern
		if err := json.Unmarshal(decoded, &result); err != nil {
			return patterns.Pattern{}, fmt.Errorf("failed parsing pattern json: %w", err)
		}

		if err = result.Validate(); err != nil {
			return patterns.Pattern{}, fmt.Errorf("failed validating pattern: %w", err)
		}

		return result, nil
	}

	type patternConfig struct {
		key        string
		defaultVal patterns.Pattern
	}

	configPatterns := map[*patterns.Pattern]patternConfig{
		&instance.CallPattern:         {key: patternKeyCall, defaultVal: patterns.DefaultCallPattern},
		&instance.FavoriteCallPattern: {key: patternKeyFavoriteCall, defaultVal: patterns.DefaultCallPattern},
		&instance.NotificationPattern: {key: patternKeyNotification, defaultVal: patterns.DefaultNotificationPattern},
	}

	for property, patternData := range configPatterns {
		if patternRaw := activePatterns.Key(patternData.key).String(); patternRaw != "" {
			pattern, err := parsePattern(patternRaw)
			if err != nil {
				*property = patternData.defaultVal
				changedConfig = true
			} else {
				*property = pattern
			}
		} else {
			*property = patterns.DefaultCallPattern
			changedConfig = true
		}
	}

	storedPatterns := cfgRaw.Section(sectionStoredPatterns)
	for _, value := range storedPatterns.Keys() {
		pattern, err := parsePattern(value.String())
		if err != nil {
			changedConfig = true
			continue
		}
		if err = pattern.Validate(); err != nil {
			changedConfig = true
			continue
		}

		instance.StoredPatterns[pattern.Name] = pattern
	}

	if changedConfig {
		_ = cfgFile.Close()
		if err = instance.Save(); err != nil {
			return nil, fmt.Errorf("could not save config file: %w", err)
		}
	}

	return instance, nil
}

func (receiver *Config) Save() error {
	if receiver.cfgFilePath == "" {
		return errors.New("no config file set, please construct using config.New()")
	}

	activePatterns := map[string]patterns.Pattern{
		patternKeyCall:         receiver.CallPattern,
		patternKeyFavoriteCall: receiver.FavoriteCallPattern,
		patternKeyNotification: receiver.NotificationPattern,
	}

	if _, err := os.Stat(filepath.Dir(receiver.cfgFilePath)); os.IsNotExist(err) {
		if err = os.MkdirAll(filepath.Dir(receiver.cfgFilePath), 0700); err != nil {
			return fmt.Errorf("could not create directory %s: %w", filepath.Dir(receiver.cfgFilePath), err)
		}
	}

	cfgFile, err := os.OpenFile(receiver.cfgFilePath, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("could not open config file: %w", err)
	}
	defer cfgFile.Close()

	cfgRaw, err := ini.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("could not load config file: %w", err)
	}

	for name, pattern := range activePatterns {
		if err := pattern.Validate(); err != nil {
			return fmt.Errorf("the %s pattern is not valid: %w", name, err)
		}
		patternJson := lo.Must(json.Marshal(pattern))
		cfgRaw.Section(sectionActivePatterns).Key(name).SetValue(base64.RawStdEncoding.EncodeToString(patternJson))
	}

	for name, pattern := range receiver.StoredPatterns {
		if err := pattern.Validate(); err != nil {
			return fmt.Errorf("the %s pattern is not valid: %w", name, err)
		}
		patternJson := lo.Must(json.Marshal(pattern))
		cfgRaw.Section(sectionStoredPatterns).Key(name).SetValue(base64.RawStdEncoding.EncodeToString(patternJson))
	}

	_ = cfgFile.Close()
	if err = cfgRaw.SaveTo(receiver.cfgFilePath); err != nil {
		return fmt.Errorf("could not save config file: %w", err)
	}

	return nil
}
