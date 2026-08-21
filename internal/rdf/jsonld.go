package rdf

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	rdfType = "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"
	vocabSchema = "https://schema.org/"
)

// GraphFromJSONLD expands a compact JSON-LD document into triples.
func GraphFromJSONLD(data []byte) (*Graph, error) {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	g := NewGraph()
	b := &jsonldBuilder{g: g, prefixes: map[string]string{
		"rdf":    "http://www.w3.org/1999/02/22-rdf-syntax-ns#",
		"rdfs":   "http://www.w3.org/2000/01/rdf-schema#",
		"xsd":    "http://www.w3.org/2001/XMLSchema#",
		"schema": vocabSchema,
		"dcat":   "http://www.w3.org/ns/dcat#",
		"dct":    "http://purl.org/dc/terms/",
		"foaf":   "http://xmlns.com/foaf/0.1/",
	}}
	b.walk(v, b.ctx)
	return g, nil
}

type jsonldBuilder struct {
	g        *Graph
	ctx      map[string]string
	prefixes map[string]string
	vocab    string
	blank    int
}

func (b *jsonldBuilder) nextBlank() string {
	b.blank++
	return fmt.Sprintf("_:b%d", b.blank)
}

func (b *jsonldBuilder) walk(v interface{}, parentCtx map[string]string) {
	switch n := v.(type) {
	case []interface{}:
		for _, item := range n {
			b.walk(item, parentCtx)
		}
	case map[string]interface{}:
		ctx := parentCtx
		if raw, ok := n["@context"]; ok {
			ctx = b.mergeContext(parentCtx, raw)
		}
		if graph, ok := n["@graph"]; ok {
			b.walk(graph, ctx)
			// catalog-level properties sit beside @graph
			if id, ok := n["@id"].(string); ok && id != "" {
				b.emitNode(id, n, ctx, true)
			}
			return
		}
		id := nodeID(n, b)
		b.emitNode(id, n, ctx, false)
	}
}

func (b *jsonldBuilder) emitNode(id string, n map[string]interface{}, ctx map[string]string, skipGraph bool) {
	for k, v := range n {
		if k == "@context" || k == "@id" || k == "@graph" && skipGraph {
			continue
		}
		if k == "@type" {
			for _, t := range asList(v) {
				if s, ok := t.(string); ok {
					b.g.AddIRI(id, rdfType, b.expand(s, ctx))
				}
			}
			continue
		}
		pred := b.expand(k, ctx)
		for _, item := range asList(v) {
			b.emitValue(id, pred, item, ctx)
		}
	}
}

func (b *jsonldBuilder) emitValue(subj, pred string, v interface{}, ctx map[string]string) {
	switch n := v.(type) {
	case nil:
		return
	case map[string]interface{}:
		if raw, ok := n["@value"]; ok {
			b.g.AddLiteral(subj, pred, stringify(raw))
			return
		}
		id := nodeID(n, b)
		b.g.AddIRI(subj, pred, id)
		b.emitNode(id, n, ctx, false)
	case string:
		if looksLikeIRI(n) || strings.HasPrefix(n, "_:") {
			b.g.AddIRI(subj, pred, n)
			return
		}
		b.g.AddLiteral(subj, pred, n)
	case bool, float64, json.Number:
		b.g.AddLiteral(subj, pred, stringify(n))
	}
}

func (b *jsonldBuilder) mergeContext(parent map[string]string, raw interface{}) map[string]string {
	out := map[string]string{}
	for k, v := range parent {
		out[k] = v
	}
	obj, ok := raw.(map[string]interface{})
	if !ok {
		return out
	}
	if vocab, ok := obj["@vocab"].(string); ok {
		b.vocab = vocab
	}
	for k, v := range obj {
		s, ok := v.(string)
		if !ok || strings.HasPrefix(k, "@") {
			continue
		}
		if strings.HasSuffix(s, "#") || strings.HasSuffix(s, "/") {
			out[k] = s
			b.prefixes[k] = s
		}
	}
	for k, v := range obj {
		if strings.HasPrefix(k, "@") {
			continue
		}
		switch t := v.(type) {
		case string:
			if strings.HasSuffix(t, "#") || strings.HasSuffix(t, "/") {
				continue
			}
			iri := t
			if strings.Contains(t, ":") && !strings.Contains(t, "://") {
				iri = b.expand(t, out)
			}
			out[k] = iri
		case map[string]interface{}:
			if id, ok := t["@id"].(string); ok {
				out[k] = b.expand(id, out)
			}
		}
	}
	return out
}

func (b *jsonldBuilder) expand(term string, ctx map[string]string) string {
	term = strings.TrimSpace(term)
	if term == "" {
		return term
	}
	if term == "a" {
		return rdfType
	}
	if strings.HasPrefix(term, "<") && strings.HasSuffix(term, ">") {
		return term[1 : len(term)-1]
	}
	if strings.Contains(term, "://") || strings.HasPrefix(term, "_:") {
		return term
	}
	if mapped, ok := ctx[term]; ok {
		if strings.Contains(mapped, "://") || strings.HasPrefix(mapped, "_:") {
			return mapped
		}
		return b.expand(mapped, ctx)
	}
	if i := strings.IndexByte(term, ':'); i > 0 {
		pre, local := term[:i], term[i+1:]
		if iri, ok := b.prefixes[pre]; ok {
			return iri + local
		}
		if iri, ok := ctx[pre]; ok && (strings.HasSuffix(iri, "#") || strings.HasSuffix(iri, "/")) {
			return iri + local
		}
	}
	if b.vocab != "" {
		return b.vocab + term
	}
	if !strings.Contains(term, ":") {
		return vocabSchema + term
	}
	return term
}

func nodeID(n map[string]interface{}, b *jsonldBuilder) string {
	if id, ok := n["@id"].(string); ok && id != "" {
		return id
	}
	return b.nextBlank()
}

func asList(v interface{}) []interface{} {
	if v == nil {
		return nil
	}
	if a, ok := v.([]interface{}); ok {
		return a
	}
	return []interface{}{v}
}

func stringify(v interface{}) string {
	switch n := v.(type) {
	case string:
		return n
	case bool:
		if n {
			return "true"
		}
		return "false"
	case float64:
		if n == float64(int64(n)) {
			return fmt.Sprintf("%d", int64(n))
		}
		return fmt.Sprintf("%v", n)
	case json.Number:
		return n.String()
	default:
		return fmt.Sprint(n)
	}
}

func looksLikeIRI(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
