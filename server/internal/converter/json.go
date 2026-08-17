package converter

import (
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"

	commonv1 "github.com/yvvlee/kirby/server/gen/kirby/common/v1"
)

func EncodeFieldType(value *commonv1.Field_Type) (string, error) {
	if value == nil {
		return "", fmt.Errorf("field type is nil")
	}
	encoded, err := protojson.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode field type: %w", err)
	}
	return string(encoded), nil
}

func DecodeFieldType(value string) (*commonv1.Field_Type, error) {
	result := new(commonv1.Field_Type)
	if err := protojson.Unmarshal([]byte(value), result); err != nil {
		return nil, fmt.Errorf("decode field type: %w", err)
	}
	return result, nil
}

func EncodeFields(values []*commonv1.Field) (string, error) {
	encodedValues := make([]json.RawMessage, 0, len(values))
	for _, value := range values {
		if value == nil {
			return "", fmt.Errorf("encode fields: nil field")
		}
		encoded, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(value)
		if err != nil {
			return "", fmt.Errorf("encode fields: %w", err)
		}
		encodedValues = append(encodedValues, encoded)
	}
	encoded, err := json.Marshal(encodedValues)
	if err != nil {
		return "", fmt.Errorf("encode fields: %w", err)
	}
	return string(encoded), nil
}

func DecodeFields(value string) ([]*commonv1.Field, error) {
	encodedValues := make([]json.RawMessage, 0)
	if err := json.Unmarshal([]byte(value), &encodedValues); err != nil {
		return nil, fmt.Errorf("decode fields: %w", err)
	}
	result := make([]*commonv1.Field, 0, len(encodedValues))
	for _, encoded := range encodedValues {
		item := new(commonv1.Field)
		if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(encoded, item); err != nil {
			return nil, fmt.Errorf("decode fields: %w", err)
		}
		result = append(result, item)
	}
	return result, nil
}

func EncodeOptions(values []*commonv1.SelectOption) (string, error) {
	encoded, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("encode enum values: %w", err)
	}
	return string(encoded), nil
}

func DecodeOptions(value string) ([]*commonv1.SelectOption, error) {
	result := make([]*commonv1.SelectOption, 0)
	if err := json.Unmarshal([]byte(value), &result); err != nil {
		return nil, fmt.Errorf("decode enum values: %w", err)
	}
	return result, nil
}

func EncodeTags(values []commonv1.Snapshot_Tag) (string, error) {
	encoded, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("encode snapshot tags: %w", err)
	}
	return string(encoded), nil
}

func DecodeTags(value string) ([]commonv1.Snapshot_Tag, error) {
	result := make([]commonv1.Snapshot_Tag, 0)
	if err := json.Unmarshal([]byte(value), &result); err != nil {
		return nil, fmt.Errorf("decode snapshot tags: %w", err)
	}
	return result, nil
}
