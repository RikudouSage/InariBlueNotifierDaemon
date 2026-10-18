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
	"strings"
	"time"

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
	patternKeyMatrix       = "matrix"
	patternKeyOff          = "off"

	sectionStoredPatterns      = "storedPatterns"
	sectionActivePatterns      = "activePatterns"
	sectionEvents              = "events"
	sectionConstraints         = "constraints"
	sectionNotificationFilters = "notificationFilters"
	sectionInternal            = "internal"
	sectionKnock               = "knock"

	constraintNameStartTime = "startTime"
	constraintNameEndTime   = "endTime"

	internalKeyMigratedVersion = "migratedVersion"

	knockKeyEnabled   = "enabled"
	knockKeyPattern   = "pattern"
	knockKeyTimeout   = "timeout"
	knockKeyProtocol  = "protocol"
	knockKeyPort      = "port"
	knockKeyAddresses = "addresses"
)

type EventType string

const (
	EventCall         EventType = patternKeyCall
	EventFavoriteCall EventType = patternKeyFavoriteCall
	EventNotification EventType = patternKeyNotification
	EventSMS          EventType = patternKeySMS
	EventEmail        EventType = patternKeyEmail
	EventSignal       EventType = patternKeySignal
	EventMatrix       EventType = patternKeyMatrix
)

type Constraints struct {
	StartTime dto.TimeOnly
	EndTime   dto.TimeOnly
}

type Knock struct {
	Enabled   bool
	Pattern   patterns.Pattern
	Port      uint16
	Timeout   time.Duration
	Protocols []string
	Addresses []string
}

type Config struct {
	cfgFilePath string

	MigratedVersion uint

	StoredPatterns       map[string]patterns.Pattern
	NotificationFilters  map[string]dto.CustomAppFilter
	CustomActivePatterns map[string]patterns.Pattern

	CallPattern         patterns.Pattern
	FavoriteCallPattern *patterns.Pattern
	NotificationPattern patterns.Pattern
	SMSPattern          *patterns.Pattern
	EmailPattern        *patterns.Pattern
	SignalPattern       *patterns.Pattern
	MatrixPattern       *patterns.Pattern
	OffPattern          patterns.Pattern

	Events      map[EventType]bool
	Constraints Constraints
	Knock       Knock
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
		patternKeyMatrix: {
			get: func() *patterns.Pattern { return receiver.MatrixPattern },
			set: func(pattern *patterns.Pattern) { receiver.MatrixPattern = pattern },
		},
		patternKeyOff: {
			defaultValue: new(patterns.Off),
			get:          func() *patterns.Pattern { return &receiver.OffPattern },
			set:          func(pattern *patterns.Pattern) { receiver.OffPattern = *pattern },
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
		cfgFilePath:          cfgFilePath,
		StoredPatterns:       make(map[string]patterns.Pattern),
		Events:               make(map[EventType]bool, 8),
		NotificationFilters:  make(map[string]dto.CustomAppFilter),
		CustomActivePatterns: make(map[string]patterns.Pattern),
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
			patterns.Steady,
			patterns.Off,
		}

		instance.StoredPatterns = lo.KeyBy(initialPatterns, func(item patterns.Pattern) string {
			return item.Name
		})

		changedConfig = true
	}
	if err != nil {
		return nil, fmt.Errorf("could not load config file: %w", err)
	}

	internal := cfgRaw.Section(sectionInternal)
	migratedVersion, err := internal.Key(internalKeyMigratedVersion).Uint()
	if err != nil {
		instance.MigratedVersion = 0
		changedConfig = true
	} else {
		instance.MigratedVersion = migratedVersion
	}

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

	activePatterns := cfgRaw.Section(sectionActivePatterns)
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

	customNotificationFilters := cfgRaw.Section(sectionNotificationFilters)
	for _, key := range customNotificationFilters.Keys() {
		raw := key.String()
		decoded, err := base64.RawStdEncoding.DecodeString(raw)
		if err != nil {
			continue
		}

		var filter dto.CustomAppFilter
		if err := json.Unmarshal(decoded, &filter); err != nil {
			continue
		}

		instance.NotificationFilters[key.Name()] = filter
	}

	for filterKey, filter := range instance.NotificationFilters {
		filterName := filter.FilterName
		if filterName == "" {
			filterName = filterKey
		}

		patternRaw := activePatterns.Key(filterName).String()
		if patternRaw == "" {
			continue
		}

		pattern, err := parsePattern(patternRaw)
		if err != nil {
			activePatterns.DeleteKey(filterName)
			changedConfig = true
			continue
		}

		instance.CustomActivePatterns[filterName] = pattern
	}

	events := cfgRaw.Section(sectionEvents)
	for _, key := range events.Keys() {
		instance.Events[EventType(key.Name())], err = key.Bool()
		if err != nil {
			instance.Events[EventType(key.Name())] = false
		}
	}
	defaultKeys := []EventType{EventCall, EventFavoriteCall, EventNotification, EventSMS, EventEmail, EventSignal, EventMatrix}
	for _, key := range defaultKeys {
		if _, ok := instance.Events[key]; !ok {
			instance.Events[key] = !slices.Contains([]EventType{
				EventNotification,
				EventSMS,
				EventEmail,
				EventSignal,
				EventMatrix,
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

	knock := cfgRaw.Section(sectionKnock)
	knockMap := map[string]any{
		knockKeyEnabled:   &instance.Knock.Enabled,
		knockKeyPattern:   &instance.Knock.Pattern,
		knockKeyPort:      &instance.Knock.Port,
		knockKeyTimeout:   &instance.Knock.Timeout,
		knockKeyProtocol:  &instance.Knock.Protocols,
		knockKeyAddresses: &instance.Knock.Addresses,
	}
	for key, ref := range knockMap {
		val := knock.Key(key)
		switch ref.(type) {
		case *bool:
			*(ref.(*bool)), err = val.Bool()
			if err != nil {
				*(ref.(*bool)) = false
				changedConfig = true
			}
		case *uint16:
			uintVal, err := val.Uint()
			if err != nil {
				*(ref.(*uint16)) = 10093
				changedConfig = true
				break
			}
			*(ref.(*uint16)) = uint16(uintVal)
		case *patterns.Pattern:
			patternVal, err := parsePattern(val.String())
			if err != nil {
				*(ref.(*patterns.Pattern)) = patterns.MediumLengthBreath
				changedConfig = true
				break
			}
			*(ref.(*patterns.Pattern)) = patternVal
		case *time.Duration:
			*(ref.(*time.Duration)), err = val.Duration()
			if err != nil {
				// Older versions saved duration values as unitless nanoseconds.
				nanoseconds, legacyErr := strconv.ParseInt(val.String(), 10, 64)
				if legacyErr == nil {
					*(ref.(*time.Duration)) = time.Duration(nanoseconds)
				} else {
					*(ref.(*time.Duration)) = time.Minute
				}
				changedConfig = true
			}
		case *[]string:
			*(ref.(*[]string)) = strings.Split(val.String(), ",")
		}
	}
	if instance.Knock.Pattern.Name == "" {
		instance.Knock.Pattern = patterns.MediumLengthBreath
		changedConfig = true
	}
	if instance.Knock.Port == 0 {
		instance.Knock.Port = 10093
	}
	originalProtocolsLen := len(instance.Knock.Protocols)
	instance.Knock.Protocols = lo.Filter(instance.Knock.Protocols, func(item string, _ int) bool {
		return item == "tcp" || item == "udp"
	})
	if len(instance.Knock.Protocols) != originalProtocolsLen {
		changedConfig = true
	}
	if len(instance.Knock.Protocols) == 0 {
		instance.Knock.Protocols = []string{"tcp", "udp"}
	}
	// Preserve invalid addresses so the listener fails closed instead of
	// silently falling back to a wildcard binding.
	for i, address := range instance.Knock.Addresses {
		instance.Knock.Addresses[i] = strings.TrimSpace(address)
	}
	if len(instance.Knock.Addresses) == 1 && instance.Knock.Addresses[0] == "" {
		instance.Knock.Addresses = nil
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

	for name, filter := range receiver.NotificationFilters {
		filterJson := lo.Must(json.Marshal(filter))
		cfgRaw.Section(sectionNotificationFilters).Key(name).SetValue(base64.RawStdEncoding.EncodeToString(filterJson))
	}

	for filterKey, filter := range receiver.NotificationFilters {
		filterName := filter.FilterName
		if filterName == "" {
			filterName = filterKey
		}
		if _, configured := receiver.CustomActivePatterns[filterName]; !configured {
			cfgRaw.Section(sectionActivePatterns).DeleteKey(filterName)
		}
	}
	for name, pattern := range receiver.CustomActivePatterns {
		if err := pattern.Validate(); err != nil {
			return fmt.Errorf("the custom pattern for filter %q is not valid: %w", name, err)
		}
		patternJSON := lo.Must(json.Marshal(pattern))
		cfgRaw.Section(sectionActivePatterns).Key(name).SetValue(base64.RawStdEncoding.EncodeToString(patternJSON))
	}

	timeConstraints := map[string]dto.TimeOnly{
		constraintNameStartTime: receiver.Constraints.StartTime,
		constraintNameEndTime:   receiver.Constraints.EndTime,
	}
	for key, value := range timeConstraints {
		cfgRaw.Section(sectionConstraints).Key(key).SetValue(value.String())
	}

	knock := cfgRaw.Section(sectionKnock)
	knock.Key(knockKeyEnabled).SetValue(strconv.FormatBool(receiver.Knock.Enabled))
	if err := receiver.Knock.Pattern.Validate(); err != nil {
		return fmt.Errorf("the custom pattern for filter knock is not valid: %w", err)
	}
	knock.Key(knockKeyPattern).SetValue(base64.RawStdEncoding.EncodeToString(lo.Must(json.Marshal(receiver.Knock.Pattern))))
	knock.Key(knockKeyPort).SetValue(fmt.Sprintf("%d", receiver.Knock.Port))
	knock.Key(knockKeyTimeout).SetValue(receiver.Knock.Timeout.String())
	knock.Key(knockKeyProtocol).SetValue(strings.Join(receiver.Knock.Protocols, ","))
	knock.Key(knockKeyAddresses).SetValue(strings.Join(receiver.Knock.Addresses, ","))

	cfgRaw.Section(sectionInternal).Key(internalKeyMigratedVersion).SetValue(strconv.FormatInt(int64(receiver.MigratedVersion), 10))

	_ = cfgFile.Close()
	if err = cfgRaw.SaveTo(receiver.cfgFilePath); err != nil {
		return fmt.Errorf("could not save config file: %w", err)
	}

	return nil
}
