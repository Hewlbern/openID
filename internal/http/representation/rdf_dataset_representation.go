//go:build cssstub

// Package representation provides the RdfDatasetRepresentation struct.
package representation

// Dataset is a placeholder for an RDF dataset (e.g., from an RDF library).
type Dataset interface{}

// RdfDatasetRepresentation contains an RDF dataset instead of a raw data stream.
type RdfDatasetRepresentation interface {
	Representation
	Dataset() Dataset
}

// BasicRdfDatasetRepresentation is a concrete implementation of RdfDatasetRepresentation.
type BasicRdfDatasetRepresentation struct {
	Metadata *RepresentationMetadata
	Dataset  interface{} // TODO: Replace with concrete Dataset type
	Binary   bool
}

func (r *BasicRdfDatasetRepresentation) GetMetadata() *RepresentationMetadata { return r.Metadata }
func (r *BasicRdfDatasetRepresentation) GetData() io.Reader                  { return nil }
func (r *BasicRdfDatasetRepresentation) IsBinary() bool                      { return r.Binary }
func (r *BasicRdfDatasetRepresentation) IsEmpty() bool                       { return r.Dataset == nil }
func (r *BasicRdfDatasetRepresentation) Dataset() Dataset                    { return r.Dataset }
