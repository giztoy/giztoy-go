package serverpublic

import (
	"encoding/json"
	"time"
)

func reencode[T any](v any) (T, error) {
	var out T
	data, err := json.Marshal(v)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out, err
	}
	return out, nil
}

func stringPtr(v string) *string {
	if v == "" {
		return nil
	}
	out := v
	return &out
}

func boolPtr(v bool) *bool {
	if !v {
		return nil
	}
	out := v
	return &out
}

func millisPtr(ms int64) *time.Time {
	if ms == 0 {
		return nil
	}
	t := time.UnixMilli(ms).UTC()
	return &t
}

func millisTime(ms int64) time.Time {
	if ms == 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms).UTC()
}
