package config

import (
	"InariBlueNotifier/led"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/ini.v1"
)

const (
	callPatternKey = "call"
)

type Config struct {
	cfgFilePath string

	CallPattern led.Pattern
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
	cfgFilePath := filepath.Join(cfgDir, "config.ini")
	cfgFile, err := os.Open(cfgFilePath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("could not open config file: %w", err)
	}
	if err == nil {
		defer cfgFile.Close()
	}

	var cfgRaw *ini.File
	if cfgFile != nil {
		cfgRaw, err = ini.Load(cfgFile)
	} else {
		cfgRaw, err = ini.Load(make([]byte, 0))
	}
	if err != nil {
		return nil, fmt.Errorf("could not load config file: %w", err)
	}
	patterns := cfgRaw.Section("patterns")

	parsePattern := func(raw string) (led.Pattern, error) {
		var result led.Pattern
		if err := json.Unmarshal([]byte(raw), &result); err != nil {
			return led.Pattern{}, fmt.Errorf("failed parsing pattern json: %w", err)
		}

		if err = result.Validate(); err != nil {
			return led.Pattern{}, fmt.Errorf("failed validating pattern: %w", err)
		}

		return result, nil
	}

	instance := &Config{
		cfgFilePath: cfgFilePath,
	}

	changed := false
	if callPatternRaw := patterns.Key(callPatternKey).String(); callPatternRaw != "" {
		callPattern, err := parsePattern(callPatternRaw)
		if err != nil {
			cfgRaw.Section("patterns").DeleteKey(callPatternKey)
			changed = true
		} else {
			instance.CallPattern = callPattern
		}
	}

	if changed {
		_ = cfgFile.Close()
		_ = cfgRaw.SaveTo(cfgFilePath)
	}

	return instance, nil
}

func (receiver *Config) Save() error {
	if receiver.cfgFilePath == "" {
		return errors.New("no config file set, please construct using config.New()")
	}

	if err := receiver.CallPattern.Validate(); err != nil {
		return fmt.Errorf("the call pattern is not valid: %w", err)
	}
	callPatternJson, err := json.Marshal(receiver.CallPattern)
	if err != nil {
		return fmt.Errorf("could not marshal call pattern: %w", err)
	}
	buf := bytes.NewBuffer(nil)
	err = json.Compact(buf, callPatternJson)
	if err != nil {
		return fmt.Errorf("could not compact call pattern: %w", err)
	}

	if _, err = os.Stat(filepath.Dir(receiver.cfgFilePath)); os.IsNotExist(err) {
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
	cfgRaw.Section("patterns").Key(callPatternKey).SetValue(string(buf.Bytes()))
	_ = cfgFile.Close()
	if err = cfgRaw.SaveTo(receiver.cfgFilePath); err != nil {
		return fmt.Errorf("could not save config file: %w", err)
	}

	return nil
}
