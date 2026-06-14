package provider

import (
	"errors"
	"fmt"
	"strings"
)

func boolArgument(arguments map[string]any, key string) (bool, bool) {
	value, ok := arguments[key].(bool)
	return value, ok
}

func stringArgument(arguments map[string]any, key string) (string, bool) {
	value, ok := arguments[key].(string)
	value = strings.TrimSpace(value)
	return value, ok && value != ""
}

func volumeArgument(arguments map[string]any) (int, error) {
	if v, ok := arguments["volume"].(float64); ok {
		return int(v), nil
	}
	if v, ok := arguments["volume"].(int); ok {
		return v, nil
	}
	return 0, errors.New("missing or invalid volume parameter")
}

func positionArgument(arguments map[string]any) (int, error) {
	if v, ok := arguments["position_ms"].(float64); ok {
		return max(0, int(v)), nil
	}
	if v, ok := arguments["position_ms"].(int); ok {
		return max(0, v), nil
	}
	return 0, errors.New("missing or invalid position_ms parameter")
}

func shuffleArgument(arguments map[string]any) (bool, error) {
	if v, ok := arguments["state"].(bool); ok {
		return v, nil
	}
	if v, ok := arguments["shuffle"].(bool); ok {
		return v, nil
	}
	return false, errors.New("missing or invalid shuffle state parameter")
}

func repeatArgument(arguments map[string]any) (string, error) {
	value, ok := stringArgument(arguments, "state")
	if !ok {
		value, ok = stringArgument(arguments, "repeat_state")
	}
	if !ok {
		return "", errors.New("missing or invalid repeat state parameter")
	}
	switch value {
	case "off", "track", "context":
		return value, nil
	default:
		return "", fmt.Errorf("unsupported repeat state: %s", value)
	}
}
