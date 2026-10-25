package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The config is stored in the database with one row per field.
// The key is the path of YAML field names, like "MediaServer.URL".
// Lists and maps are stored as a single value. Values are stored as JSON.

type configField struct {
	Key   string
	Value reflect.Value
}

// configFields returns every stored field of the config, in struct order
func configFields(cfg *Config) []configField {
	fields := []configField{}
	collectConfigFields(reflect.ValueOf(cfg).Elem(), "", &fields)
	return fields
}

func collectConfigFields(v reflect.Value, prefix string, fields *[]configField) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		structField := t.Field(i)
		name := yamlFieldName(structField)
		if !structField.IsExported() || name == "-" {
			continue
		}
		key := joinConfigKey(prefix, name)

		// Nested structs are flattened into their fields
		if structField.Type.Kind() == reflect.Struct {
			collectConfigFields(v.Field(i), key, fields)
			continue
		}
		*fields = append(*fields, configField{Key: key, Value: v.Field(i)})
	}
}

func yamlFieldName(structField reflect.StructField) string {
	name, _, _ := strings.Cut(structField.Tag.Get("yaml"), ",")
	if name == "" {
		return structField.Name
	}
	return name
}

func joinConfigKey(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

// ToValues returns the config as a map of field key to JSON encoded value
func (config Config) ToValues() (values map[string]string, err error) {
	values = map[string]string{}
	for _, field := range configFields(&config) {
		encoded, err := encodeConfigValue(field.Value)
		if err != nil {
			return nil, fmt.Errorf("failed to encode config field '%s': %w", field.Key, err)
		}
		values[field.Key] = encoded
	}
	return values, nil
}

// ConfigFromValues builds a config from stored values.
// Missing values and values that can't be decoded use the default value.
// Bad values and unknown keys are returned, so they can be logged.
func ConfigFromValues(values map[string]string) (cfg Config, decodeErrors map[string]string, unknownKeys []string) {
	cfg = DefaultConfig()
	decodeErrors = map[string]string{}
	knownKeys := map[string]bool{}

	for _, field := range configFields(&cfg) {
		knownKeys[field.Key] = true
		raw, found := values[field.Key]
		if !found {
			continue
		}
		if err := decodeConfigValue(raw, field.Value); err != nil {
			decodeErrors[field.Key] = err.Error()
		}
	}

	for key := range values {
		if !knownKeys[key] {
			unknownKeys = append(unknownKeys, key)
		}
	}
	sort.Strings(unknownKeys)

	return cfg, decodeErrors, unknownKeys
}

// encodeConfigValue encodes a value as JSON.
// Goes through YAML to keep json:"-" fields and match the config.yaml format.
func encodeConfigValue(v reflect.Value) (string, error) {
	// Keep nil lists and maps as null
	switch v.Kind() {
	case reflect.Slice, reflect.Map, reflect.Pointer:
		if v.IsNil() {
			return "null", nil
		}
	}

	yamlData, err := yaml.Marshal(v.Interface())
	if err != nil {
		return "", err
	}
	var generic any
	if err := yaml.Unmarshal(yamlData, &generic); err != nil {
		return "", err
	}
	jsonData, err := json.Marshal(generic)
	if err != nil {
		return "", err
	}
	return string(jsonData), nil
}

// decodeConfigValue decodes a value from encodeConfigValue into target.
// target is only changed if the value decodes.
func decodeConfigValue(raw string, target reflect.Value) error {
	var generic any
	if err := json.Unmarshal([]byte(raw), &generic); err != nil {
		return err
	}
	yamlData, err := yaml.Marshal(generic)
	if err != nil {
		return err
	}
	decoded := reflect.New(target.Type())
	if err := yaml.Unmarshal(yamlData, decoded.Interface()); err != nil {
		return err
	}
	target.Set(decoded.Elem())
	return nil
}
