package main

import (
	"text/template"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/encoding/protojson"
)

type tplMessage struct {
	*protogen.Message
	*protojson.MarshalOptions
	*protojson.UnmarshalOptions
}

var messageTemplate = template.Must(template.New("message").Parse(`
// MarshalJSON implements json.Marshaler
func (msg *{{.GoIdent.GoName}}) MarshalJSON() ([]byte,error) {
	return protojson.MarshalOptions {
		UseEnumNumbers: {{.UseEnumNumbers}},
		EmitUnpopulated: {{.EmitUnpopulated}},
		UseProtoNames: {{.UseProtoNames}},
	}.Marshal(msg)
}

// UnmarshalJSON implements json.Unmarshaler
func (msg *{{.GoIdent.GoName}}) UnmarshalJSON(b []byte) error {
	return protojson.UnmarshalOptions {
		DiscardUnknown: {{.DiscardUnknown}},
	}.Unmarshal(b, msg)
}
`))
