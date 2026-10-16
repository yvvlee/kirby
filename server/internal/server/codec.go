package server

import (
	kratosjson "github.com/go-kratos/kratos/v2/encoding/json"
	"google.golang.org/protobuf/encoding/protojson"
)

func init() {
	// Configure the shared Kratos codec before any HTTP server starts.
	kratosjson.MarshalOptions = protojson.MarshalOptions{
		UseProtoNames:   true,
		EmitUnpopulated: true,
	}
	kratosjson.UnmarshalOptions = protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}
}
