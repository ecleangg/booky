package support

import (
	"strconv"
	"strings"
)

func MergeStringMaps(sources ...map[string]string) map[string]string {
	merged := map[string]string{}
	for _, source := range sources {
		for key, value := range source {
			merged[key] = value
		}
	}
	return merged
}

func MapString(metadata map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(metadata[key]); value != "" {
			return value
		}
	}
	return ""
}

func MapTruthy(metadata map[string]string, keys ...string) bool {
	for _, key := range keys {
		if value := strings.TrimSpace(metadata[key]); value != "" {
			return TruthyString(value)
		}
	}
	return false
}

func ParseBool(value string) bool {
	parsed, err := strconv.ParseBool(value)
	return err == nil && parsed
}

func TruthyString(value string) bool {
	if ParseBool(value) {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "yes", "y":
		return true
	default:
		return false
	}
}
