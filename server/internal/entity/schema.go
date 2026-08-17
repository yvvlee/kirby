package entity

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	commonv1 "github.com/yvvlee/kirby/server/gen/kirby/common/v1"
	"github.com/yvvlee/kirby/server/internal/set"
)

type Schema struct {
	structures map[string]*commonv1.Structure
	enums      map[string]*commonv1.ConfigEnum
}

var (
	schemaKeyPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`)
	enumValuePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
)

func NewSchema(structures []*commonv1.Structure, enums []*commonv1.ConfigEnum) (*Schema, error) {
	result := &Schema{structures: make(map[string]*commonv1.Structure, len(structures)), enums: make(map[string]*commonv1.ConfigEnum, len(enums))}
	for _, item := range structures {
		if item == nil || !validKey(item.Key) || !validName(item.Name) || utf8.RuneCountInString(item.Description) > 255 {
			return nil, Invalid("structure key is required")
		}
		if _, exists := result.structures[item.Key]; exists {
			return nil, Conflict("duplicate structure key %s", item.Key)
		}
		result.structures[item.Key] = item
	}
	for _, item := range enums {
		if item == nil || !validKey(item.Key) || !validName(item.Name) || utf8.RuneCountInString(item.Description) > 255 || len(item.Values) == 0 {
			return nil, Invalid("enum key and values are required")
		}
		if _, exists := result.enums[item.Key]; exists {
			return nil, Conflict("duplicate enum key %s", item.Key)
		}
		seen := set.New[string]()
		for _, option := range item.Values {
			if option == nil || !enumValuePattern.MatchString(option.Value) || !validName(option.Label) || utf8.RuneCountInString(option.Description) > 255 || seen.Contains(option.Value) {
				return nil, Invalid("enum %s contains an invalid or duplicate value", item.Key)
			}
			seen.Add(option.Value)
		}
		result.enums[item.Key] = item
	}
	for _, item := range structures {
		seen := set.New[string]()
		for _, field := range item.Fields {
			if field == nil || !validKey(field.Key) || !validName(field.Name) || utf8.RuneCountInString(field.Description) > 255 || seen.Contains(field.Key) {
				return nil, Invalid("structure %s contains an invalid or duplicate field", item.Key)
			}
			seen.Add(field.Key)
			if err := result.validateType(field.Type); err != nil {
				return nil, err
			}
		}
	}
	if err := result.validateCycles(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Schema) ValidateConfig(config *commonv1.Config) error {
	if config == nil || config.Type == nil || !validKey(config.Key) {
		return Invalid("config and type are required")
	}
	if err := s.validateType(config.Type); err != nil {
		return err
	}
	value, err := decodeJSON(config.Value)
	if err != nil {
		return Invalid("config value is not valid JSON: %v", err)
	}
	root := &commonv1.Field{Key: config.Key, Name: config.Key, IsArray: config.IsArray, Type: config.Type}
	return s.validateValue(root, value, "$", set.New[string]())
}

func validKey(value string) bool {
	return len(value) <= 64 && schemaKeyPattern.MatchString(value)
}

func validName(value string) bool {
	length := utf8.RuneCountInString(value)
	return strings.TrimSpace(value) != "" && length <= 64
}

func (s *Schema) DefaultValue(configType *commonv1.Field_Type, isArray bool) (string, error) {
	if err := s.validateType(configType); err != nil {
		return "", err
	}
	field := &commonv1.Field{Key: "value", Name: "value", Type: configType, IsArray: isArray}
	value, err := s.defaultValue(field, set.New[string]())
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode default value: %w", err)
	}
	return string(encoded), nil
}

func (s *Schema) BuildTree(config *commonv1.Config) (*commonv1.TreeNode, error) {
	if config == nil || config.Type == nil {
		return nil, Invalid("config and type are required")
	}
	if err := s.validateType(config.Type); err != nil {
		return nil, err
	}
	root := &commonv1.TreeNode{Value: &commonv1.Field{
		Key: config.Key, Name: "配置值", Description: config.Description, IsArray: config.IsArray, Type: config.Type,
	}}
	children, err := s.children(config.Type, set.New[string]())
	if err != nil {
		return nil, err
	}
	root.Children = children
	return root, nil
}

func FilterStructureChoices(items []*commonv1.Structure, ignoreID int64) ([]*commonv1.Structure, error) {
	if ignoreID <= 0 {
		return items, nil
	}
	var ignored *commonv1.Structure
	for _, item := range items {
		if item.Id == ignoreID {
			ignored = item
			break
		}
	}
	if ignored == nil {
		return items, nil
	}
	rejected := set.New(ignored.Key)
	changed := true
	for changed {
		changed = false
		for _, item := range items {
			if rejected.Contains(item.Key) {
				continue
			}
			for _, field := range item.Fields {
				if rejected.Contains(field.GetType().GetStructureKey()) {
					rejected.Add(item.Key)
					changed = true
					break
				}
			}
		}
	}
	result := make([]*commonv1.Structure, 0, len(items))
	for _, item := range items {
		if !rejected.Contains(item.Key) {
			result = append(result, item)
		}
	}
	return result, nil
}

func (s *Schema) validateType(fieldType *commonv1.Field_Type) error {
	if fieldType == nil || fieldType.Kind == nil {
		return Invalid("field type is required")
	}
	switch value := fieldType.Kind.(type) {
	case *commonv1.Field_Type_BaseType:
		if value.BaseType < commonv1.Field_STRING || value.BaseType > commonv1.Field_FILE {
			return Invalid("unsupported base type")
		}
	case *commonv1.Field_Type_StructureKey:
		if _, exists := s.structures[value.StructureKey]; !exists {
			return Invalid("referenced structure %s does not exist", value.StructureKey)
		}
	case *commonv1.Field_Type_EnumKey:
		if _, exists := s.enums[value.EnumKey]; !exists {
			return Invalid("referenced enum %s does not exist", value.EnumKey)
		}
	default:
		return Invalid("unsupported field type")
	}
	return nil
}

func (s *Schema) validateCycles() error {
	visiting, visited := set.New[string](), set.New[string]()
	var walk func(string, []string) error
	walk = func(key string, path []string) error {
		if visiting.Contains(key) {
			return Conflict("structure cycle: %s", strings.Join(append(path, key), " -> "))
		}
		if visited.Contains(key) {
			return nil
		}
		visiting.Add(key)
		for _, field := range s.structures[key].Fields {
			dependency := field.GetType().GetStructureKey()
			if dependency != "" {
				if err := walk(dependency, append(path, key)); err != nil {
					return err
				}
			}
		}
		visiting.Remove(key)
		visited.Add(key)
		return nil
	}
	for key := range s.structures {
		if err := walk(key, nil); err != nil {
			return err
		}
	}
	return nil
}

func (s *Schema) children(fieldType *commonv1.Field_Type, path *set.Set[string]) ([]*commonv1.TreeNode, error) {
	key := fieldType.GetStructureKey()
	if key == "" {
		return nil, nil
	}
	if path.Contains(key) {
		return nil, Conflict("structure cycle contains %s", key)
	}
	path.Add(key)
	defer path.Remove(key)
	structure := s.structures[key]
	result := make([]*commonv1.TreeNode, 0, len(structure.Fields))
	for _, field := range structure.Fields {
		children, err := s.children(field.Type, path)
		if err != nil {
			return nil, err
		}
		result = append(result, &commonv1.TreeNode{Value: field, Children: children})
	}
	return result, nil
}

func (s *Schema) validateValue(field *commonv1.Field, value any, path string, structures *set.Set[string]) error {
	if field.IsArray {
		items, ok := value.([]any)
		if !ok {
			return Invalid("%s must be an array", path)
		}
		clone := &commonv1.Field{Key: field.Key, Name: field.Name, Description: field.Description, Type: field.Type}
		for index, item := range items {
			if err := s.validateValue(clone, item, fmt.Sprintf("%s[%d]", path, index), structures); err != nil {
				return err
			}
		}
		return nil
	}
	switch kind := field.Type.Kind.(type) {
	case *commonv1.Field_Type_BaseType:
		return validateBaseValue(kind.BaseType, value, path)
	case *commonv1.Field_Type_EnumKey:
		text, ok := value.(string)
		if !ok {
			return Invalid("%s must be an enum string", path)
		}
		for _, option := range s.enums[kind.EnumKey].Values {
			if option.Value == text {
				return nil
			}
		}
		return Invalid("%s contains an unknown enum value", path)
	case *commonv1.Field_Type_StructureKey:
		object, ok := value.(map[string]any)
		if !ok {
			return Invalid("%s must be an object", path)
		}
		if structures.Contains(kind.StructureKey) {
			return Conflict("structure cycle contains %s", kind.StructureKey)
		}
		structures.Add(kind.StructureKey)
		defer structures.Remove(kind.StructureKey)
		definition := s.structures[kind.StructureKey]
		if len(object) != len(definition.Fields) {
			return Invalid("%s does not match structure %s", path, kind.StructureKey)
		}
		for _, child := range definition.Fields {
			childValue, exists := object[child.Key]
			if !exists {
				return Invalid("%s.%s is required", path, child.Key)
			}
			if err := s.validateValue(child, childValue, path+"."+child.Key, structures); err != nil {
				return err
			}
		}
		return nil
	default:
		return Invalid("%s has no field type", path)
	}
}

func (s *Schema) defaultValue(field *commonv1.Field, structures *set.Set[string]) (any, error) {
	if field.IsArray {
		return []any{}, nil
	}
	switch kind := field.Type.Kind.(type) {
	case *commonv1.Field_Type_BaseType:
		switch kind.BaseType {
		case commonv1.Field_INT, commonv1.Field_DECIMAL:
			return 0, nil
		case commonv1.Field_BOOLEAN:
			return false, nil
		case commonv1.Field_TIME_RANGE, commonv1.Field_DATE_RANGE, commonv1.Field_DATETIME_RANGE:
			return []any{}, nil
		default:
			return "", nil
		}
	case *commonv1.Field_Type_EnumKey:
		return s.enums[kind.EnumKey].Values[0].Value, nil
	case *commonv1.Field_Type_StructureKey:
		if structures.Contains(kind.StructureKey) {
			return nil, Conflict("structure cycle contains %s", kind.StructureKey)
		}
		structures.Add(kind.StructureKey)
		defer structures.Remove(kind.StructureKey)
		result := make(map[string]any)
		for _, child := range s.structures[kind.StructureKey].Fields {
			value, err := s.defaultValue(child, structures)
			if err != nil {
				return nil, err
			}
			result[child.Key] = value
		}
		return result, nil
	default:
		return nil, Invalid("field type is required")
	}
}

func validateBaseValue(baseType commonv1.Field_BaseType, value any, path string) error {
	valid := false
	switch baseType {
	case commonv1.Field_STRING, commonv1.Field_IMAGE, commonv1.Field_VIDEO, commonv1.Field_FILE:
		_, valid = value.(string)
	case commonv1.Field_INT:
		number, ok := value.(json.Number)
		if ok {
			_, err := strconv.ParseInt(number.String(), 10, 64)
			valid = err == nil
		}
	case commonv1.Field_DECIMAL:
		number, ok := value.(json.Number)
		if ok {
			_, err := strconv.ParseFloat(number.String(), 64)
			valid = err == nil
		}
	case commonv1.Field_BOOLEAN:
		_, valid = value.(bool)
	case commonv1.Field_DATE:
		valid = validTime(value, "2006-01-02")
	case commonv1.Field_TIME:
		valid = validTime(value, "15:04:05")
	case commonv1.Field_DATETIME:
		valid = validTime(value, "2006-01-02 15:04:05")
	case commonv1.Field_DATE_RANGE:
		valid = validRange(value, "2006-01-02")
	case commonv1.Field_TIME_RANGE:
		valid = validRange(value, "15:04:05")
	case commonv1.Field_DATETIME_RANGE:
		valid = validRange(value, "2006-01-02 15:04:05")
	}
	if !valid {
		return Invalid("%s does not match %s", path, baseType.String())
	}
	return nil
}

func validTime(value any, layout string) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	_, err := time.Parse(layout, text)
	return err == nil
}

func validRange(value any, layout string) bool {
	items, ok := value.([]any)
	if !ok || len(items) != 2 {
		return false
	}
	return validTime(items[0], layout) && validTime(items[1], layout)
}

func decodeJSON(value string) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.UseNumber()
	var result any
	if err := decoder.Decode(&result); err != nil {
		return nil, err
	}
	if decoder.More() {
		return nil, fmt.Errorf("multiple JSON values")
	}
	if remaining := bytes.TrimSpace([]byte(value)[decoder.InputOffset():]); len(remaining) != 0 {
		return nil, fmt.Errorf("trailing JSON data")
	}
	return result, nil
}
