//go:build cssstub

// Package input provides the BasicRequestParser and its dependencies.
package input

import (
	"fmt"
)

// BasicRequestParserArgs contains the dependencies for BasicRequestParser.
type BasicRequestParserArgs struct {
	TargetExtractor    interface{} // TODO: define TargetExtractor interface
	PreferenceParser   interface{} // TODO: define PreferenceParser interface
	MetadataParser     interface{} // TODO: define MetadataParser interface
	ConditionsParser   interface{} // TODO: define ConditionsParser interface
	BodyParser         interface{} // TODO: define BodyParser interface
}

// BasicRequestParser aggregates input parsers to create an Operation from an HttpRequest.
type BasicRequestParser struct {
	targetExtractor    interface{}
	preferenceParser   interface{}
	metadataParser     interface{}
	conditionsParser   interface{}
	bodyParser         interface{}
}

// NewBasicRequestParser constructs a BasicRequestParser from its dependencies.
func NewBasicRequestParser(args BasicRequestParserArgs) *BasicRequestParser {
	return &BasicRequestParser{
		targetExtractor:  args.TargetExtractor,
		preferenceParser: args.PreferenceParser,
		metadataParser:   args.MetadataParser,
		conditionsParser: args.ConditionsParser,
		bodyParser:       args.BodyParser,
	}
}

// Handle creates an Operation from an HttpRequest by aggregating the results of the input parsers.
func (p *BasicRequestParser) Handle(request map[string]interface{}) (map[string]interface{}, error) {
	method, ok := request["method"].(string)
	if !ok || method == "" {
		return nil, fmt.Errorf("No method specified on the HTTP request")
	}
	target, err := callHandleSafe(p.targetExtractor, map[string]interface{}{ "request": request })
	if err != nil {
		return nil, err
	}
	preferences, err := callHandleSafe(p.preferenceParser, map[string]interface{}{ "request": request })
	if err != nil {
		return nil, err
	}
	metadata := map[string]interface{}{ "identifier": target }
	_, err = callHandleSafe(p.metadataParser, map[string]interface{}{ "request": request, "metadata": metadata })
	if err != nil {
		return nil, err
	}
	conditions, err := callHandleSafe(p.conditionsParser, request)
	if err != nil {
		return nil, err
	}
	body, err := callHandleSafe(p.bodyParser, map[string]interface{}{ "request": request, "metadata": metadata })
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"method":      method,
		"target":      target,
		"preferences": preferences,
		"conditions":  conditions,
		"body":        body,
	}, nil
}

// callHandleSafe is a helper to call a HandleSafe method on a parser.
func callHandleSafe(parser interface{}, input interface{}) (interface{}, error) {
	if parser == nil {
		return nil, fmt.Errorf("parser is nil")
	}
	if h, ok := parser.(interface{ HandleSafe(interface{}) (interface{}, error) }); ok {
		return h.HandleSafe(input)
	}
	if h, ok := parser.(interface{ HandleSafe(interface{}) error }); ok {
		return nil, h.HandleSafe(input)
	}
	return nil, fmt.Errorf("parser does not implement HandleSafe")
}
