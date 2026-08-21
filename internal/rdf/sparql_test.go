package rdf

import (
	"strings"
	"testing"
)

func TestGraphFromJSONLDAndSPARQL(t *testing.T) {
	doc := []byte(`{
	  "@context": {
	    "@vocab": "https://schema.org/",
	    "oid": "http://localhost:4000/ns/records#",
	    "workType": "oid:workType",
	    "package": "oid:package"
	  },
	  "@graph": [
	    {
	      "@id": "http://localhost:4000/mike/records/traces/openid-agent-identity/abc.jsonld",
	      "@type": ["CreativeWork", "oid:AgentTrace"],
	      "name": "Wire MCP into OpenID",
	      "workType": "identity-protocol",
	      "package": "openid-agent-identity",
	      "offers": { "@type": "Offer", "price": 137 }
	    },
	    {
	      "@id": "http://localhost:4000/mike/records/traces/frontline-field-product/def.jsonld",
	      "@type": ["CreativeWork", "oid:AgentTrace"],
	      "name": "Fix frontline login",
	      "workType": "debugging",
	      "package": "frontline-field-product",
	      "offers": { "@type": "Offer", "price": 4 }
	    }
	  ]
	}`)
	g, err := GraphFromJSONLD(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !g.Has("http://localhost:4000/mike/records/traces/openid-agent-identity/abc.jsonld", "https://schema.org/name", "Wire MCP into OpenID") {
		t.Fatalf("name triple missing: %+v", g.Triples)
	}
	if !g.Has("http://localhost:4000/mike/records/traces/openid-agent-identity/abc.jsonld", rdfType, "http://localhost:4000/ns/records#AgentTrace") {
		t.Fatalf("type triple missing: %+v", g.Triples)
	}

	res, err := QuerySPARQL(g, `
		SELECT ?name ?workType WHERE {
		  ?id a oid:AgentTrace ;
		      schema:name ?name ;
		      oid:workType ?workType .
		  FILTER(CONTAINS(LCASE(?name), "openid"))
		}
	`, DefaultSPARQLPrefixes("http://localhost:4000"))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Results.Bindings) != 1 {
		t.Fatalf("want 1 row, got %d (%+v)", len(res.Results.Bindings), res.Results.Bindings)
	}
	if res.Results.Bindings[0]["workType"].Value != "identity-protocol" {
		t.Fatalf("workType = %+v", res.Results.Bindings[0])
	}

	res, err = QuerySPARQL(g, `
		SELECT ?name ?price WHERE {
		  ?id schema:name ?name .
		  OPTIONAL { ?id schema:offers ?off . ?off schema:price ?price }
		}
		ORDER BY DESC(?price)
	`, DefaultSPARQLPrefixes("http://localhost:4000"))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Results.Bindings) != 2 {
		t.Fatalf("want 2 rows, got %d", len(res.Results.Bindings))
	}
	if res.Results.Bindings[0]["price"].Value != "137" {
		t.Fatalf("expected highest price first, got %+v", res.Results.Bindings)
	}

	res, err = QuerySPARQL(g, `
		SELECT DISTINCT ?package WHERE {
		  ?s oid:package ?package
		}
		ORDER BY ?package
	`, DefaultSPARQLPrefixes("http://localhost:4000"))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Results.Bindings) != 2 {
		t.Fatalf("packages: %+v", res.Results.Bindings)
	}
}

func TestSPARQLRejectsUpdate(t *testing.T) {
	g := NewGraph()
	_, err := QuerySPARQL(g, `INSERT DATA { <a> <b> <c> }`, nil)
	if err == nil || !strings.Contains(err.Error(), "SELECT") {
		t.Fatalf("expected SELECT-only error, got %v", err)
	}
}
