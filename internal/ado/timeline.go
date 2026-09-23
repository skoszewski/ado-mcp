package ado

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// LogInfo is the name and type of the timeline record that produced a log.
type LogInfo struct {
	Name string
	Type string
}

// Step is one timeline record as the tools report it.
type Step struct {
	ID              *string          `json:"id"`
	ParentID        *string          `json:"parent_id"`
	Type            *string          `json:"type"`
	Name            *string          `json:"name"`
	Order           *int             `json:"order"`
	State           *string          `json:"state"`
	Result          *string          `json:"result"`
	StartTime       *string          `json:"start_time"`
	FinishTime      *string          `json:"finish_time"`
	DurationSeconds *float64         `json:"duration_seconds"`
	LogID           *int             `json:"log_id"`
	Issues          []map[string]any `json:"issues"`
}

// MapLogsToTimeline maps each log ID to the name and type of the timeline record that produced
// it. The flat log list carries neither, so this is the only way to know what a log is.
func MapLogsToTimeline(records []TimelineRecord) map[int]LogInfo {
	mapping := make(map[int]LogInfo)
	for _, record := range records {
		if record.Log != nil {
			mapping[record.Log.ID] = LogInfo{Name: deref(record.Name), Type: deref(record.Type)}
		}
	}
	return mapping
}

// IsCombinedJobLog reports whether a log belongs to a Job record, whose log is the combined
// console output of every step in that job.
func IsCombinedJobLog(info LogInfo, known bool) bool {
	return known && info.Type == "Job"
}

// DescribeLog returns a human-readable label for a log.
func DescribeLog(logID int, logInfo map[int]LogInfo) string {
	info, known := logInfo[logID]
	switch {
	case !known:
		return fmt.Sprintf("Log %d", logID)
	case IsCombinedJobLog(info, known):
		return fmt.Sprintf("%s (combined job output)", info.Name)
	default:
		return fmt.Sprintf("%s (%s)", info.Name, info.Type)
	}
}

// SelectLogs filters logs by logType: "job" keeps combined job logs, "task" keeps the others,
// and "all" keeps every log.
func SelectLogs(logs []RunLog, logInfo map[int]LogInfo, logType string) ([]RunLog, error) {
	if logType == "all" {
		return logs, nil
	}
	if logType != "job" && logType != "task" {
		return nil, fmt.Errorf("unknown log_type '%s'; use 'job', 'task' or 'all'", logType)
	}
	selected := []RunLog{}
	for _, log := range logs {
		info, known := logInfo[log.ID]
		if IsCombinedJobLog(info, known) == (logType == "job") {
			selected = append(selected, log)
		}
	}
	return selected, nil
}

// DurationSeconds returns the seconds between two ISO 8601 timestamps, or nil if either is
// missing or unparseable.
func DurationSeconds(start, finish *string) *float64 {
	if start == nil || finish == nil {
		return nil
	}
	startTime, err := time.Parse(time.RFC3339Nano, *start)
	if err != nil {
		return nil
	}
	finishTime, err := time.Parse(time.RFC3339Nano, *finish)
	if err != nil {
		return nil
	}
	seconds := finishTime.Sub(startTime).Seconds()
	return &seconds
}

// StepEntries converts timeline records into steps sorted by their timeline order, keeping
// those logType selects: "job" keeps records whose log is a combined job log, "task" keeps the
// others, and "all" keeps every record.
func StepEntries(records []TimelineRecord, logInfo map[int]LogInfo, logType string) []Step {
	steps := []Step{}
	for _, record := range records {
		var logID *int
		combined := false
		if record.Log != nil {
			logID = &record.Log.ID
			info, known := logInfo[record.Log.ID]
			combined = IsCombinedJobLog(info, known)
		}
		if logType != "all" && combined != (logType == "job") {
			continue
		}
		issues := record.Issues
		if issues == nil {
			issues = []map[string]any{}
		}
		steps = append(steps, Step{
			ID: record.ID, ParentID: record.ParentID, Type: record.Type, Name: record.Name,
			Order: record.Order, State: record.State, Result: record.Result,
			StartTime: record.StartTime, FinishTime: record.FinishTime,
			DurationSeconds: DurationSeconds(record.StartTime, record.FinishTime),
			LogID:           logID, Issues: issues,
		})
	}

	order := func(step Step) int {
		if step.Order == nil {
			return math.MaxInt
		}
		return *step.Order
	}
	sort.SliceStable(steps, func(i, j int) bool { return order(steps[i]) < order(steps[j]) })
	return steps
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
