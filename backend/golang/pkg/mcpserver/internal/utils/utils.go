package utils

import (
	"encoding/json"
	"reflect"

	"github.com/invopop/jsonschema"
)

var jsonSchemaReflector = jsonschema.Reflector{
	BaseSchemaID:              "",
	Anonymous:                 true,
	AssignAnchor:              false,
	AllowAdditionalProperties: true,
	DoNotReference:            true,
	ExpandedStruct:            true,
	FieldNameTag:              "",
	IgnoredTypes:              nil,
	Lookup:                    nil,
	Mapper:                    nil,
	Namer:                     nil,
	KeyNamer:                  nil,
	AdditionalFields:          nil,
	CommentMap:                nil,
}

func ConverToInputSchema(args any) (json.RawMessage, error) {
	jsonSchema := jsonSchemaReflector.ReflectFromType(reflect.TypeOf(args))
	jsonSchema.Version = "" // Remove $schema field

	// Convert *jsonschema.Schema into a generic map[string]any
	schemaBytes, err := json.Marshal(jsonSchema)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(schemaBytes), nil
}
