// Package timeutil provides shared time parsing utilities.
package timeutil

import (
	"strconv"
	"strings"
	"time"
)

// ParseDuration extends time.ParseDuration with "d" for days.
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}
