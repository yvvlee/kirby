package entity

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	commonv1 "github.com/yvvlee/kirby/server/api/common"
)

type ConfigSnapshot struct {
	Config     *commonv1.Config       `json:"config"`
	Structures []*commonv1.Structure  `json:"structures"`
	Enums      []*commonv1.ConfigEnum `json:"enums"`
	Tree       *commonv1.TreeNode     `json:"tree"`
}

type configSnapshotJSON struct {
	Config     json.RawMessage   `json:"config"`
	Structures []json.RawMessage `json:"structures"`
	Enums      []json.RawMessage `json:"enums"`
	Tree       json.RawMessage   `json:"tree"`
}

// EncodeConfigSnapshot uses protobuf JSON for every nested message. The Go
// encoding/json package cannot safely round-trip protobuf oneof fields.
func EncodeConfigSnapshot(value *ConfigSnapshot) (string, error) {
	if value == nil || value.Config == nil || value.Tree == nil {
		return "", Invalid("snapshot content is incomplete")
	}
	wire := configSnapshotJSON{Structures: make([]json.RawMessage, 0, len(value.Structures)), Enums: make([]json.RawMessage, 0, len(value.Enums))}
	var err error
	if wire.Config, err = marshalProto(value.Config); err != nil {
		return "", err
	}
	if wire.Tree, err = marshalProto(value.Tree); err != nil {
		return "", err
	}
	for _, item := range value.Structures {
		encoded, err := marshalProto(item)
		if err != nil {
			return "", err
		}
		wire.Structures = append(wire.Structures, encoded)
	}
	for _, item := range value.Enums {
		encoded, err := marshalProto(item)
		if err != nil {
			return "", err
		}
		wire.Enums = append(wire.Enums, encoded)
	}
	encoded, err := json.Marshal(wire)
	if err != nil {
		return "", fmt.Errorf("encode snapshot content: %w", err)
	}
	return string(encoded), nil
}

func DecodeConfigSnapshot(content string) (*ConfigSnapshot, error) {
	decoder := json.NewDecoder(bytes.NewBufferString(content))
	decoder.DisallowUnknownFields()
	var wire configSnapshotJSON
	if err := decoder.Decode(&wire); err != nil {
		return nil, fmt.Errorf("decode snapshot content: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, Invalid("snapshot content contains trailing JSON")
		}
		return nil, fmt.Errorf("decode snapshot content: %w", err)
	}
	result := &ConfigSnapshot{Config: new(commonv1.Config), Tree: new(commonv1.TreeNode), Structures: make([]*commonv1.Structure, 0, len(wire.Structures)), Enums: make([]*commonv1.ConfigEnum, 0, len(wire.Enums))}
	if err := unmarshalProto(wire.Config, result.Config); err != nil {
		return nil, err
	}
	if err := unmarshalProto(wire.Tree, result.Tree); err != nil {
		return nil, err
	}
	for _, encoded := range wire.Structures {
		item := new(commonv1.Structure)
		if err := unmarshalProto(encoded, item); err != nil {
			return nil, err
		}
		result.Structures = append(result.Structures, item)
	}
	for _, encoded := range wire.Enums {
		item := new(commonv1.ConfigEnum)
		if err := unmarshalProto(encoded, item); err != nil {
			return nil, err
		}
		result.Enums = append(result.Enums, item)
	}
	return result, nil
}

func marshalProto(value proto.Message) (json.RawMessage, error) {
	if value == nil || !value.ProtoReflect().IsValid() {
		return nil, Invalid("snapshot contains a nil message")
	}
	encoded, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode snapshot message: %w", err)
	}
	return encoded, nil
}

func unmarshalProto(encoded json.RawMessage, value proto.Message) error {
	if len(encoded) == 0 || string(encoded) == "null" {
		return Invalid("snapshot content is incomplete")
	}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(encoded, value); err != nil {
		return fmt.Errorf("decode snapshot message: %w", err)
	}
	return nil
}
