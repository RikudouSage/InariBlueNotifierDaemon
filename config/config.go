package config

import (
	"InariBlueNotifier/dto"
	"InariBlueNotifier/patterns"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/samber/lo"
	"gopkg.in/ini.v1"
)

const (
	patternKeyCall         = "call"
	patternKeyFavoriteCall = "favoriteCall"
	patternKeyNotification = "notification"
	patternKeySMS          = "sms"
	patternKeyEmail        = "email"
	patternKeySignal       = "signal"

	sectionStoredPatterns = "storedPatterns"
	sectionActivePatterns = "activePatterns"
	sectionEvents         = "events"
	sectionConstraints    = "constraints"

	constraintNameStartTime = "startTime"
	constraintNameEndTime   = "endTime"
)

type EventType string

const (
	EventCall         EventType = patternKeyCall
	EventFavoriteCall EventType = patternKeyFavoriteCall
	EventNotification EventType = patternKeyNotification
	EventSMS          EventType = patternKeySMS
	EventEmail        EventType = patternKeyEmail
	EventSignal       EventType = patternKeySignal
)

type Constraints struct {
	StartTime dto.TimeOnly
	EndTime   dto.TimeOnly
}

type Config struct {
	cfgFilePath string

	StoredPatterns map[string]patterns.Pattern

	CallPattern         patterns.Pattern
	FavoriteCallPattern *patterns.Pattern
	NotificationPattern patterns.Pattern
	SMSPattern          *patterns.Pattern
	EmailPattern        *patterns.Pattern
	SignalPattern       *patterns.Pattern

	Events      map[EventType]bool
	Constraints Constraints
}

type patternConfig struct {
	defaultValue *patterns.Pattern
	get          func() *patterns.Pattern
	set          func(*patterns.Pattern)
}

// patternConfigs describes every active pattern. A nil defaultValue makes an
// override optional: absent configuration remains nil and can inherit another
// pattern at the point where it is used.
func (receiver *Config) patternConfigs() map[string]patternConfig {
	return map[string]patternConfig{
		patternKeyCall: {
			defaultValue: new(patterns.DefaultCallPattern),
			get:          func() *patterns.Pattern { return &receiver.CallPattern },
			set:          func(pattern *patterns.Pattern) { receiver.CallPattern = *pattern },
		},
		patternKeyFavoriteCall: {
			get: func() *patterns.Pattern { return receiver.FavoriteCallPattern },
			set: func(pattern *patterns.Pattern) { receiver.FavoriteCallPattern = pattern },
		},
		patternKeyNotification: {
			defaultValue: new(patterns.DefaultNotificationPattern),
			get:          func() *patterns.Pattern { return &receiver.NotificationPattern },
			set:          func(pattern *patterns.Pattern) { receiver.NotificationPattern = *pattern },
		},
		patternKeySMS: {
			get: func() *patterns.Pattern { return receiver.SMSPattern },
			set: func(pattern *patterns.Pattern) { receiver.SMSPattern = pattern },
		},
		patternKeyEmail: {
			get: func() *patterns.Pattern { return receiver.EmailPattern },
			set: func(pattern *patterns.Pattern) { receiver.EmailPattern = pattern },
		},
		patternKeySignal: {
			get: func() *patterns.Pattern { return receiver.SignalPattern },
			set: func(pattern *patterns.Pattern) { receiver.SignalPattern = pattern },
		},
	}
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
		Events:         make(map[EventType]bool, 4),
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

	for key, patternData := range instance.patternConfigs() {
		if patternRaw := activePatterns.Key(key).String(); patternRaw != "" {
			pattern, err := parsePattern(patternRaw)
			if err != nil {
				patternData.set(patternData.defaultValue)
				if patternData.defaultValue == nil {
					activePatterns.DeleteKey(key)
				}
				changedConfig = true
			} else {
				patternData.set(new(pattern))
			}
		} else if patternData.defaultValue != nil {
			patternData.set(patternData.defaultValue)
			changedConfig = true
		}
	}

	storedPatterns := cfgRaw.Section(sectionStoredPatterns)
	for _, key := range storedPatterns.Keys() {
		pattern, err := parsePattern(key.String())
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

	events := cfgRaw.Section(sectionEvents)
	for _, key := range events.Keys() {
		instance.Events[EventType(key.Name())], err = key.Bool()
		if err != nil {
			instance.Events[EventType(key.Name())] = false
		}
	}
	defaultKeys := []EventType{EventCall, EventFavoriteCall, EventNotification, EventSMS, EventEmail, EventSignal}
	for _, key := range defaultKeys {
		if _, ok := instance.Events[key]; !ok {
			instance.Events[key] = !slices.Contains([]EventType{
				EventNotification,
				EventSMS,
				EventEmail,
				EventSignal,
			}, key)
			changedConfig = true
		}
	}

	constraints := cfgRaw.Section(sectionConstraints)
	timeConstraintsMap := map[string]*dto.TimeOnly{
		constraintNameStartTime: &instance.Constraints.StartTime,
		constraintNameEndTime:   &instance.Constraints.EndTime,
	}
	for key, ref := range timeConstraintsMap {
		if val := constraints.Key(key).String(); val != "" {
			*ref = dto.NewTimeOnly(val)
		} else {
			changedConfig = true
		}
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

	for name, patternData := range receiver.patternConfigs() {
		pattern := patternData.get()
		if pattern == nil {
			cfgRaw.Section(sectionActivePatterns).DeleteKey(name)
			continue
		}
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

	for name, enabled := range receiver.Events {
		cfgRaw.Section(sectionEvents).Key(string(name)).SetValue(strconv.FormatBool(enabled))
	}

	timeConstraints := map[string]dto.TimeOnly{
		constraintNameStartTime: receiver.Constraints.StartTime,
		constraintNameEndTime:   receiver.Constraints.EndTime,
	}
	for key, value := range timeConstraints {
		cfgRaw.Section(sectionConstraints).Key(key).SetValue(value.String())
	}

	_ = cfgFile.Close()
	if err = cfgRaw.SaveTo(receiver.cfgFilePath); err != nil {
		return fmt.Errorf("could not save config file: %w", err)
	}

	return nil
}
