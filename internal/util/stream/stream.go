//go:build cssstub

package stream

import (
	"bufio"
	"io"
	"solid-go/internal/util/n3"
	"strings"
)

// ReadableToQuads converts a readable stream to N3 quads
func ReadableToQuads(reader io.Reader) n3.Store {
	store := n3.NewBasicStore()
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse the N3 triple
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		// Extract subject, predicate, and object
		subject := parseTerm(parts[0])
		predicate := parseTerm(parts[1])
		object := parseTerm(parts[2])

		// Extract graph if present
		var graph n3.Term
		if len(parts) > 3 && parts[3] == "." {
			graph = nil
		} else if len(parts) > 3 {
			graph = parseTerm(parts[3])
		}

		// Add the quad to the store
		store.AddQuad(n3.Quad{
			Subject:   subject,
			Predicate: predicate,
			Object:    object,
			Graph:     graph,
		})
	}

	return store
}

// parseTerm parses an N3 term into a Term interface
func parseTerm(term string) n3.Term {
	// Remove trailing punctuation
	term = strings.TrimSuffix(term, ";")
	term = strings.TrimSuffix(term, ".")

	// Handle URIs
	if strings.HasPrefix(term, "<") && strings.HasSuffix(term, ">") {
		return &n3.BasicTerm{value: term[1 : len(term)-1]}
	}

	// Handle literals
	if strings.HasPrefix(term, "\"") && strings.HasSuffix(term, "\"") {
		return &n3.BasicTerm{value: term[1 : len(term)-1]}
	}

	// Handle blank nodes
	if strings.HasPrefix(term, "_:") {
		return &n3.BasicTerm{value: term}
	}

	// Handle prefixed names
	if strings.Contains(term, ":") {
		return &n3.BasicTerm{value: term}
	}

	// Default case
	return &n3.BasicTerm{value: term}
}
