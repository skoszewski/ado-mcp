package tools

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

const (
	optimizeSmallModel = "small-model"
	optimizeLogType    = "log-type"
)

var (
	optimizationNamePattern = regexp.MustCompile(`^[a-z][a-z-]*$`)
	logTypes                = []string{"job", "task", "all"}
)

// Optimizations are the --optimize settings the tools adapt their behaviour to.
type Optimizations struct {
	// SmallModel makes ado_get_run_log return the whole log, or its tail, instead of a page.
	SmallModel bool
	// LogType is the log type ado_list_run_logs reports when a call does not name one.
	LogType string
}

// DefaultOptimizations returns the settings in effect when --optimize names none.
func DefaultOptimizations() Optimizations {
	return Optimizations{LogType: "job"}
}

// ParseOptimizations parses a comma-separated --optimize value of switches named on their own
// and keys given as name=value, e.g. "small-model,log-type=task".
func ParseOptimizations(value string) (Optimizations, error) {
	optimizations := DefaultOptimizations()
	for _, item := range strings.Split(value, ",") {
		name, rawValue, hasValue := strings.Cut(strings.TrimSpace(item), "=")
		name = strings.TrimSpace(name)
		rawValue = strings.TrimSpace(rawValue)
		if name == "" {
			continue
		}
		if !optimizationNamePattern.MatchString(name) {
			return optimizations, fmt.Errorf("invalid --optimize name %q; names are lower-case letters and dashes", name)
		}

		switch name {
		case optimizeSmallModel:
			if hasValue {
				return optimizations, fmt.Errorf("--optimize %s is a switch: name it on its own, without a value", name)
			}
			optimizations.SmallModel = true
		case optimizeLogType:
			if rawValue == "" {
				return optimizations, fmt.Errorf("--optimize %[1]s needs a value, as %[1]s=<value>", name)
			}
			if !slices.Contains(logTypes, rawValue) {
				return optimizations, fmt.Errorf("--optimize %s=%q is not one of: %s", name, rawValue, strings.Join(logTypes, ", "))
			}
			optimizations.LogType = rawValue
		default:
			return optimizations, fmt.Errorf("unknown --optimize setting %q; supported: %s, %s", name, optimizeLogType, optimizeSmallModel)
		}
	}
	return optimizations, nil
}

// String describes the settings that differ from their defaults, for the startup banner.
func (o Optimizations) String() string {
	var named []string
	if o.SmallModel {
		named = append(named, optimizeSmallModel)
	}
	if o.LogType != DefaultOptimizations().LogType {
		named = append(named, optimizeLogType+"="+o.LogType)
	}
	return strings.Join(named, ", ")
}
