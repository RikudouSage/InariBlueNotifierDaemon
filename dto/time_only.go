package dto

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type TimeOnly uint

func NewTimeOnly[TType string | int | uint | TimeOnly | time.Time](input TType) TimeOnly {
	var value uint

	switch any(input).(type) {
	case time.Time:
		return NewTimeOnly(any(input).(time.Time).Format(time.TimeOnly))
	case TimeOnly:
		return any(input).(TimeOnly)
	case uint:
		value = any(input).(uint)
	case int:
		if any(input).(int) < 0 {
			value = 0
		} else {
			value = uint(any(input).(int))
		}
	case string:
		parts := strings.Split(any(input).(string), ":")
		if len(parts) == 0 || any(input).(string) == "" {
			value = 0
			break
		}
		if len(parts) == 1 {
			parts = append(parts, "0")
		}
		if len(parts) == 2 {
			parts = append(parts, "0")
		}
		numericParts := make([]uint64, len(parts))
		for i, part := range parts {
			if len(part) == 0 {
				part = "0"
			}
			if len(part) == 2 && part[0] == '0' {
				part = part[1:]
			}

			var err error
			numericParts[i], err = strconv.ParseUint(part, 10, 32)
			if err != nil {
				numericParts[i] = 0
			}
		}

		value = uint(numericParts[0] * 3600)
		value += uint(numericParts[1] * 60)
		value += uint(numericParts[2])
	}

	return TimeOnly(value)
}

func (receiver TimeOnly) String() string {
	return fmt.Sprintf("%02d:%02d:%02d", receiver.Hour(), receiver.Minute(), receiver.Second())
}

func (receiver TimeOnly) Hour() uint {
	return uint(receiver / 3600)
}

func (receiver TimeOnly) Minute() uint {
	return (uint(receiver) - (receiver.Hour() * 3600)) / 60
}

func (receiver TimeOnly) Second() uint {
	return uint(receiver) - (receiver.Hour() * 3600) - (receiver.Minute() * 60)
}

func (receiver TimeOnly) IsZero() bool {
	return receiver == 0
}

func (receiver TimeOnly) After[TType string | int | uint | TimeOnly | time.Time](other TType) bool {
	return receiver > NewTimeOnly(other)
}

func (receiver TimeOnly) Before[TType string | int | uint | TimeOnly | time.Time](other TType) bool {
	return receiver < NewTimeOnly(other)
}
