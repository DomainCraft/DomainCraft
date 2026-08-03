//go:build js && wasm

package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/DomainCraft/DomainCraft/internal/lexer"
	"github.com/DomainCraft/DomainCraft/internal/parser"
	"github.com/DomainCraft/DomainCraft/internal/specmeta"
	"github.com/DomainCraft/DomainCraft/internal/validator"
)

var version = "dev" // set via -ldflags "-X main.version=v0.2.0"

type validationError struct {
	Entity  string `json:"entity,omitempty"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
	Warning bool   `json:"warning"`
}

type validationResult struct {
	Errors []validationError `json:"errors"`
	Parse  bool              `json:"parse"`
}

func main() {
	c := make(chan struct{})
	js.Global().Set("goValidate", js.FuncOf(validate))
	js.Global().Set("goParseField", js.FuncOf(parseField))
	js.Global().Set("goParseDomain", js.FuncOf(parseDomain))
	js.Global().Set("goSpecmeta", js.FuncOf(specmetaFunc))
	js.Global().Set("goVersion", js.FuncOf(versionFunc))
	<-c
}

func versionFunc(_ js.Value, _ []js.Value) interface{} {
	return version
}

func specmetaFunc(_ js.Value, _ []js.Value) interface{} {
	return marshal(specmeta.SpecmetaJSON())
}

func validate(_ js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return marshal(validationResult{
			Errors: []validationError{{Message: "no YAML input provided"}},
		})
	}
	schema, err := parser.ParseYAML([]byte(args[0].String()))
	if err != nil {
		return marshal(validationResult{
			Errors: []validationError{{Message: err.Error()}},
			Parse:  true,
		})
	}
	errs := validator.New(schema).Validate()
	result := make([]validationError, 0, len(errs))
	for _, e := range errs {
		result = append(result, validationError{
			Entity:  e.Entity,
			Field:   e.Field,
			Message: e.Error(),
			Warning: e.Warning,
		})
	}
	return marshal(validationResult{Errors: result})
}

func parseField(_ js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return marshal(map[string]string{"error": "no field definition provided"})
	}
	def := args[0].String()
	name := ""
	if len(args) > 1 {
		name = args[1].String()
	}
	fd, err := lexer.ParseFieldString(def)
	if err != nil {
		return marshal(map[string]string{"error": err.Error()})
	}
	fd.Name = name
	return marshal(fd)
}

func parseDomain(_ js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return marshal(map[string]string{"error": "no YAML input provided"})
	}
	raw, err := parser.ParseRawSchema([]byte(args[0].String()))
	if err != nil {
		return marshal(map[string]string{"error": err.Error()})
	}
	return marshal(raw)
}

func marshal(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		data, _ = json.Marshal(map[string]string{"error": "marshal failed"})
	}
	return string(data)
}
