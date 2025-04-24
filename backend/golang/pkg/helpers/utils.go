package helpers

import (
	"encoding/json"
	"reflect"

	"github.com/invopop/jsonschema"
)

var (
	jsonSchemaReflector = jsonschema.Reflector{
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
)

func ConverToInputSchema(args any) (map[string]any, error) {
	jsonSchema := jsonSchemaReflector.ReflectFromType(reflect.TypeOf(args))

	// Convert *jsonschema.Schema into a generic map[string]any
	schemaBytes, err := json.Marshal(jsonSchema)
	if err != nil {
		return nil, err
	}
	var inputSchema map[string]any
	if err := json.Unmarshal(schemaBytes, &inputSchema); err != nil {
		return nil, err
	}

	return inputSchema, nil
}

func ConvertToBytes(args any) ([]byte, error) {

	// First, convert the incoming arguments (which should be a generic JSON-like
	// structure) into a map[string]any so that we have an "inputSchema" equivalent.
	var inputSchema map[string]any

	argsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(argsJSON, &inputSchema); err != nil {
		return nil, err
	}

	// Now convert that map into the strongly-typed ListEmailsArguments struct.
	argsSchemaBytes, err := json.Marshal(inputSchema)
	if err != nil {
		return nil, err
	}

	return argsSchemaBytes, nil
}
