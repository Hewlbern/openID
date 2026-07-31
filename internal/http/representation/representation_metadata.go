//go:build cssstub

// Package representation provides the RepresentationMetadata struct.
package representation

// RepresentationMetadata stores metadata triples and provides methods for access.
type RepresentationMetadata struct {
	Identifier string
	Store      map[string]interface{}
}

// NewRepresentationMetadata creates a new RepresentationMetadata.
func NewRepresentationMetadata(identifier string) *RepresentationMetadata {
	return &RepresentationMetadata{
		Identifier: identifier,
		Store:      make(map[string]interface{}),
	}
}

// Add adds a metadata entry and returns the metadata for chaining.
func (m *RepresentationMetadata) Add(key string, value interface{}) *RepresentationMetadata {
	m.Store[key] = value
	return m
}

// Get retrieves a metadata entry.
func (m *RepresentationMetadata) Get(key string) (interface{}, bool) {
	v, ok := m.Store[key]
	return v, ok
}

// Remove removes a metadata entry and returns the metadata for chaining.
func (m *RepresentationMetadata) Remove(key string) *RepresentationMetadata {
	delete(m.Store, key)
	return m
}

// SetIdentifier sets the identifier.
func (m *RepresentationMetadata) SetIdentifier(id string) *RepresentationMetadata {
	m.Identifier = id
	return m
}

// GetIdentifier gets the identifier for the metadata.
func (m *RepresentationMetadata) GetIdentifier() string {
	return m.Identifier
}

// IsRepresentationMetadata checks if the object is a RepresentationMetadata.
func IsRepresentationMetadata(obj interface{}) bool {
	_, ok := obj.(*RepresentationMetadata)
	return ok
}

// Quad is a placeholder for RDF quads.
type Quad interface{}

// AddQuad adds a quad to the metadata and returns the metadata for chaining.
func (m *RepresentationMetadata) AddQuad(quad Quad) *RepresentationMetadata {
	if m.Store["quads"] == nil {
		m.Store["quads"] = []Quad{}
	}
	m.Store["quads"] = append(m.Store["quads"].([]Quad), quad)
	return m
}

// AddQuads adds multiple quads to the metadata and returns the metadata for chaining.
func (m *RepresentationMetadata) AddQuads(quads []Quad) *RepresentationMetadata {
	for _, q := range quads {
		m.AddQuad(q)
	}
	return m
}

// RemoveQuad removes a quad from the metadata and returns the metadata for chaining.
func (m *RepresentationMetadata) RemoveQuad(quad Quad) *RepresentationMetadata {
	if m.Store["quads"] == nil {
		return m
	}
	quads := m.Store["quads"].([]Quad)
	newQuads := []Quad{}
	for _, q := range quads {
		if q != quad {
			newQuads = append(newQuads, q)
		}
	}
	m.Store["quads"] = newQuads
	return m
}

// RemoveQuads removes multiple quads from the metadata and returns the metadata for chaining.
func (m *RepresentationMetadata) RemoveQuads(quads []Quad) *RepresentationMetadata {
	for _, q := range quads {
		m.RemoveQuad(q)
	}
	return m
}

// Quads returns all quads in the metadata.
func (m *RepresentationMetadata) Quads() []Quad {
	if m.Store["quads"] == nil {
		return nil
	}
	return m.Store["quads"].([]Quad)
}
