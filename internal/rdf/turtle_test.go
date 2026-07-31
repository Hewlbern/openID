package rdf

import "testing"

func TestParseSerializeTurtle(t *testing.T) {
	src := `
@prefix foaf: <http://xmlns.com/foaf/0.1/> .
<http://example.org/card#me> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://xmlns.com/foaf/0.1/Person> .
<http://example.org/card#me> <http://xmlns.com/foaf/0.1/name> "Ada" .
`
	g, err := ParseTurtle(src)
	if err != nil {
		t.Fatal(err)
	}
	if !g.Has("http://example.org/card#me", "http://xmlns.com/foaf/0.1/name", "Ada") {
		t.Fatalf("missing name triple: %+v", g.Triples)
	}
	out := SerializeTurtle(g, map[string]string{"foaf": "http://xmlns.com/foaf/0.1/"})
	g2, err := ParseTurtle(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(g2.Triples) < 2 {
		t.Fatalf("expected >=2 triples, got %d (%q)", len(g2.Triples), out)
	}
}

func TestSPARQLInsert(t *testing.T) {
	g, err := ParseTurtle(`<http://ex/a> <http://ex/b> <http://ex/c> .`)
	if err != nil {
		t.Fatal(err)
	}
	out, err := ApplySPARQLUpdate(g, `INSERT DATA { <http://ex/a> <http://ex/b> <http://ex/d> . }`)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Has("http://ex/a", "http://ex/b", "http://ex/d") {
		t.Fatalf("insert missing: %+v", out.Triples)
	}
}

func TestHashInIRINotComment(t *testing.T) {
	g, err := ParseTurtle(`<http://ex/#me> <http://ex/#name> "x" .`)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Triples) != 1 {
		t.Fatalf("got %d triples: %+v", len(g.Triples), g.Triples)
	}
}
