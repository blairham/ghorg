package configs

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// ReadConfigFile reads a YAML config file and returns all key-value pairs
// as dot-notation keys. Supports both the new nested format and legacy flat format.
func ReadConfigFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	result := make(map[string]string)
	flatten("", raw, result)
	return result, nil
}

// flatten recursively walks the YAML map and produces dot-notation keys.
func flatten(prefix string, m map[string]any, out map[string]string) {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case map[string]any:
			flatten(key, val, out)
		default:
			out[key] = fmt.Sprintf("%v", val)
		}
	}
}

// ReadConfigValue reads a single dot-notation key from a config file.
// It checks both nested YAML and legacy GHORG_* flat keys.
func ReadConfigValue(path, dotKey string) (string, bool, error) {
	values, err := ReadConfigFile(path)
	if err != nil {
		return "", false, err
	}

	// Try dot-notation key first (new format)
	if val, ok := values[dotKey]; ok {
		return val, true, nil
	}

	// Fall back to legacy env var key (old format)
	envVar := DotToEnvVar(dotKey)
	if envVar != "" {
		if val, ok := values[envVar]; ok {
			return val, true, nil
		}
	}

	return "", false, nil
}

// WriteConfigValue sets a dot-notation key in a YAML config file.
// Creates the file if it doesn't exist. Uses nested YAML structure.
func WriteConfigValue(path, dotKey, value string) error {
	var raw map[string]any

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		raw = make(map[string]any)
	} else {
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}
		if raw == nil {
			raw = make(map[string]any)
		}
	}

	// Set nested value
	parts := strings.Split(dotKey, ".")
	setNestedValue(raw, parts, value)

	return writeYAML(path, raw)
}

// UnsetConfigValue removes a dot-notation key from a YAML config file.
func UnsetConfigValue(path, dotKey string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}

	parts := strings.Split(dotKey, ".")
	if !deleteNestedValue(raw, parts) {
		// Also try legacy key
		envVar := DotToEnvVar(dotKey)
		if envVar != "" {
			delete(raw, envVar)
		}
	}

	return writeYAML(path, raw)
}

// ListConfigValues reads a config file and returns all values mapped to
// their dot-notation keys. Legacy GHORG_* keys are translated.
func ListConfigValues(path string) (map[string]string, error) {
	values, err := ReadConfigFile(path)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string, len(values))
	for k, v := range values {
		// Translate legacy keys to dot notation
		if strings.HasPrefix(k, "GHORG_") {
			dot := EnvVarToDot(k)
			if dot != "" {
				result[dot] = v
				continue
			}
		}
		result[k] = v
	}
	return result, nil
}

// setNestedValue sets a value in a nested map structure.
func setNestedValue(m map[string]any, parts []string, value string) {
	if len(parts) == 1 {
		m[parts[0]] = value
		return
	}

	child, ok := m[parts[0]]
	if !ok {
		child = make(map[string]any)
		m[parts[0]] = child
	}

	childMap, ok := child.(map[string]any)
	if !ok {
		childMap = make(map[string]any)
		m[parts[0]] = childMap
	}

	setNestedValue(childMap, parts[1:], value)
}

// deleteNestedValue removes a key from a nested map. Returns true if found and deleted.
func deleteNestedValue(m map[string]any, parts []string) bool {
	if len(parts) == 1 {
		if _, ok := m[parts[0]]; ok {
			delete(m, parts[0])
			return true
		}
		return false
	}

	child, ok := m[parts[0]]
	if !ok {
		return false
	}

	childMap, ok := child.(map[string]any)
	if !ok {
		return false
	}

	deleted := deleteNestedValue(childMap, parts[1:])

	// Clean up empty parent sections
	if deleted && len(childMap) == 0 {
		delete(m, parts[0])
	}

	return deleted
}

// writeYAML marshals a map to YAML and writes it to a file.
func writeYAML(path string, data map[string]any) error {
	out, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshaling YAML: %w", err)
	}
	return os.WriteFile(path, out, 0o600)
}

// FormatConfigList formats config key-value pairs for display, sorted by key.
// Secret values are redacted unless showSecrets is true.
func FormatConfigList(values map[string]string, showSecrets bool) string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		v := values[k]
		if !showSecrets {
			ck := LookupByDot(k)
			if ck != nil && ck.IsSecret && v != "" {
				v = "********"
			}
		}
		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteString(v)
		sb.WriteByte('\n')
	}
	return sb.String()
}
