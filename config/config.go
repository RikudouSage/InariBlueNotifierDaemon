package config

import (
	"InariBlueNotifier/patterns"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/samber/lo"
	"gopkg.in/ini.v1"
)

const (
	callPatternKey        = "call"
	storedPatternsSection = "storedPatterns"
	activePatternsSection = "activePatterns"
)

type Config struct {
	cfgFilePath string

	StoredPatterns map[string]patterns.Pattern
	CallPattern    patterns.Pattern
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
	cfgFilePath := filepath.Join(cfgDir, "config_v2.ini")
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
			patterns.MediumLengthBreath,
		}

		instance.StoredPatterns = lo.KeyBy(initialPatterns, func(item patterns.Pattern) string {
			return item.Name
		})

		changedConfig = true
	}
	if err != nil {
		return nil, fmt.Errorf("could not load config file: %w", err)
	}
	activePatterns := cfgRaw.Section(activePatternsSection)

	parsePattern := func(raw string) (patterns.Pattern, error) {
		var result patterns.Pattern
		if err := json.Unmarshal([]byte(raw), &result); err != nil {
			return patterns.Pattern{}, fmt.Errorf("failed parsing pattern json: %w", err)
		}

		if err = result.Validate(); err != nil {
			return patterns.Pattern{}, fmt.Errorf("failed validating pattern: %w", err)
		}

		return result, nil
	}

	if callPatternRaw := activePatterns.Key(callPatternKey).String(); callPatternRaw != "" {
		callPattern, err := parsePattern(callPatternRaw)
		if err != nil {
			instance.CallPattern = patterns.DefaultCallPattern
			changedConfig = true
		} else {
			instance.CallPattern = callPattern
		}
	} else {
		instance.CallPattern = patterns.DefaultCallPattern
		changedConfig = true
	}

	storedPatterns := cfgRaw.Section(storedPatternsSection)
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
		callPatternKey: receiver.CallPattern,
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
		cfgRaw.Section(activePatternsSection).Key(name).SetValue(string(patternJson))
	}

	for name, pattern := range receiver.StoredPatterns {
		if err := pattern.Validate(); err != nil {
			return fmt.Errorf("the %s pattern is not valid: %w", name, err)
		}
		patternJson := lo.Must(json.Marshal(pattern))
		cfgRaw.Section(storedPatternsSection).Key(name).SetValue(string(patternJson))
	}

	_ = cfgFile.Close()
	if err = cfgRaw.SaveTo(receiver.cfgFilePath); err != nil {
		return fmt.Errorf("could not save config file: %w", err)
	}

	return nil
}
