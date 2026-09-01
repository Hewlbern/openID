package rdf

import (
	"bufio"
	"fmt"
	"strings"
)

// Triple is a simple RDF triple.
type Triple struct {
	Subject   string
	Predicate string
	Object    string
	ObjectLit bool
	Datatype  string
	Lang      string
}

// Graph is an in-memory set of triples.
type Graph struct {
	Triples []Triple
}

func NewGraph() *Graph {
	return &Graph{}
}

func (g *Graph) Add(t Triple) {
	g.Triples = append(g.Triples, t)
}

func (g *Graph) AddIRI(s, p, o string) {
	g.Add(Triple{Subject: s, Predicate: p, Object: o})
}

func (g *Graph) AddLiteral(s, p, o string) {
	g.Add(Triple{Subject: s, Predicate: p, Object: o, ObjectLit: true})
}

func (g *Graph) AddTypedLiteral(s, p, o, datatype string) {
	g.Add(Triple{Subject: s, Predicate: p, Object: o, ObjectLit: true, Datatype: datatype})
}

func (g *Graph) Objects(s, p string) []string {
	var out []string
	for _, t := range g.Triples {
		if (s == "" || t.Subject == s) && (p == "" || t.Predicate == p) {
			out = append(out, t.Object)
		}
	}
	return out
}

func (g *Graph) Has(s, p, o string) bool {
	for _, t := range g.Triples {
		if (s == "" || t.Subject == s) && (p == "" || t.Predicate == p) && (o == "" || t.Object == o) {
			return true
		}
	}
	return false
}

// SerializeTurtle writes a minimal Turtle document.
func SerializeTurtle(g *Graph, prefixes map[string]string) string {
	var b strings.Builder
	for p, iri := range prefixes {
		fmt.Fprintf(&b, "@prefix %s: <%s> .\n", p, iri)
	}
	if len(prefixes) > 0 {
		b.WriteString("\n")
	}
	for _, t := range g.Triples {
		subj := formatTerm(t.Subject, false)
		pred := formatTerm(t.Predicate, false)
		var obj string
		if t.ObjectLit {
			obj = fmt.Sprintf("%q", t.Object)
			if t.Lang != "" {
				obj += "@" + t.Lang
			} else if t.Datatype != "" {
				obj += "^^<" + t.Datatype + ">"
			}
		} else {
			obj = formatTerm(t.Object, false)
		}
		fmt.Fprintf(&b, "%s %s %s .\n", subj, pred, obj)
	}
	return b.String()
}

func formatTerm(term string, lit bool) string {
	if lit {
		return fmt.Sprintf("%q", term)
	}
	if strings.HasPrefix(term, "_:") {
		return term
	}
	if strings.Contains(term, ":") && !strings.Contains(term, "://") && !strings.HasPrefix(term, "<") {
		return term // prefixed
	}
	return "<" + strings.Trim(term, "<>") + ">"
}

// ParseTurtle parses a simplified Turtle subset (enough for ACL/WebID/patches).
func ParseTurtle(data string) (*Graph, error) {
	g := NewGraph()
	// strip comments (#...) but not inside IRIs <...> or quoted literals
	var cleaned strings.Builder
	sc := bufio.NewScanner(strings.NewReader(data))
	for sc.Scan() {
		line := stripTurtleComment(sc.Text())
		cleaned.WriteString(line)
		cleaned.WriteByte('\n')
	}
	text := cleaned.String()
	prefixes := map[string]string{}

	// @prefix / PREFIX — terminate on '.' outside IRIs (avoid matching '.' inside http://…/0.1/)
	for {
		text = strings.TrimSpace(text)
		var rest string
		switch {
		case strings.HasPrefix(text, "@prefix"):
			rest = strings.TrimSpace(text[len("@prefix"):])
		case strings.HasPrefix(text, "PREFIX"):
			rest = strings.TrimSpace(text[len("PREFIX"):])
		case strings.HasPrefix(text, "prefix"):
			rest = strings.TrimSpace(text[len("prefix"):])
		default:
			rest = ""
		}
		if rest == "" {
			break
		}
		end := indexStatementEnd(rest)
		if end < 0 {
			return nil, fmt.Errorf("invalid @prefix")
		}
		stmt := strings.TrimSpace(rest[:end])
		text = rest[end+1:]
		parts := strings.Fields(stmt)
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid @prefix parts")
		}
		name := strings.TrimSuffix(parts[0], ":")
		iri := strings.Trim(parts[1], "<>")
		prefixes[name] = iri
	}

	// statements ending with .
	for _, stmt := range splitStatements(text) {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		toks, err := tokenize(stmt)
		if err != nil {
			return nil, err
		}
		if len(toks) < 3 {
			continue
		}
		subj := expand(toks[0], prefixes)
		pred := expand(toks[1], prefixes)
		objTok := toks[2]
		t := Triple{Subject: subj, Predicate: pred}
		if strings.HasPrefix(objTok, `"`) {
			lit, lang, dt := parseLiteral(objTok)
			t.Object = lit
			t.ObjectLit = true
			t.Lang = lang
			t.Datatype = dt
		} else {
			t.Object = expand(objTok, prefixes)
		}
		g.Add(t)
	}
	return g, nil
}

func indexStatementEnd(s string) int {
	inIRI := false
	inQuote := false
	escape := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escape {
			escape = false
			continue
		}
		if inQuote {
			if c == '\\' {
				escape = true
				continue
			}
			if c == '"' {
				inQuote = false
			}
			continue
		}
		if inIRI {
			if c == '>' {
				inIRI = false
			}
			continue
		}
		switch c {
		case '<':
			inIRI = true
		case '"':
			inQuote = true
		case '.':
			return i
		}
	}
	return -1
}

func stripTurtleComment(line string) string {
	inIRI := false
	inQuote := false
	escape := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if escape {
			escape = false
			continue
		}
		if inQuote {
			if c == '\\' {
				escape = true
				continue
			}
			if c == '"' {
				inQuote = false
			}
			continue
		}
		if inIRI {
			if c == '>' {
				inIRI = false
			}
			continue
		}
		switch c {
		case '<':
			inIRI = true
		case '"':
			inQuote = true
		case '#':
			return line[:i]
		}
	}
	return line
}

func splitStatements(text string) []string {
	var out []string
	var cur strings.Builder
	inQuote := false
	inIRI := false
	escape := false
	for i := 0; i < len(text); i++ {
		c := text[i]
		if escape {
			cur.WriteByte(c)
			escape = false
			continue
		}
		if inQuote {
			if c == '\\' {
				cur.WriteByte(c)
				escape = true
				continue
			}
			if c == '"' {
				inQuote = false
			}
			cur.WriteByte(c)
			continue
		}
		if inIRI {
			cur.WriteByte(c)
			if c == '>' {
				inIRI = false
			}
			continue
		}
		switch c {
		case '"':
			inQuote = true
			cur.WriteByte(c)
		case '<':
			inIRI = true
			cur.WriteByte(c)
		case '.':
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	if s := strings.TrimSpace(cur.String()); s != "" {
		out = append(out, s)
	}
	return out
}

func tokenize(stmt string) ([]string, error) {
	var toks []string
	i := 0
	for i < len(stmt) {
		for i < len(stmt) && (stmt[i] == ' ' || stmt[i] == '\t' || stmt[i] == '\n' || stmt[i] == '\r' || stmt[i] == ';') {
			i++
		}
		if i >= len(stmt) {
			break
		}
		if stmt[i] == '<' {
			j := strings.IndexByte(stmt[i:], '>')
			if j < 0 {
				return nil, fmt.Errorf("unclosed IRI")
			}
			toks = append(toks, stmt[i:i+j+1])
			i += j + 1
			continue
		}
		if stmt[i] == '"' {
			j := i + 1
			for j < len(stmt) {
				if stmt[j] == '\\' {
					j += 2
					continue
				}
				if stmt[j] == '"' {
					j++
					break
				}
				j++
			}
			// lang/datatype
			for j < len(stmt) && stmt[j] != ' ' && stmt[j] != '\t' && stmt[j] != '\n' {
				j++
			}
			toks = append(toks, stmt[i:j])
			i = j
			continue
		}
		j := i
		for j < len(stmt) && stmt[j] != ' ' && stmt[j] != '\t' && stmt[j] != '\n' && stmt[j] != ';' {
			j++
		}
		toks = append(toks, stmt[i:j])
		i = j
	}
	return toks, nil
}

func expand(term string, prefixes map[string]string) string {
	term = strings.TrimSpace(term)
	if strings.HasPrefix(term, "<") && strings.HasSuffix(term, ">") {
		return term[1 : len(term)-1]
	}
	if strings.HasPrefix(term, "_:") {
		return term
	}
	if idx := strings.IndexByte(term, ':'); idx > 0 {
		pre := term[:idx]
		local := term[idx+1:]
		if iri, ok := prefixes[pre]; ok {
			return iri + local
		}
	}
	return term
}

func parseLiteral(tok string) (string, string, string) {
	// "foo"@en or "foo"^^<dt>
	if !strings.HasPrefix(tok, `"`) {
		return tok, "", ""
	}
	end := 1
	for end < len(tok) {
		if tok[end] == '\\' {
			end += 2
			continue
		}
		if tok[end] == '"' {
			break
		}
		end++
	}
	lit := tok[1:end]
	rest := tok[end+1:]
	if strings.HasPrefix(rest, "@") {
		return unescape(lit), rest[1:], ""
	}
	if strings.HasPrefix(rest, "^^") {
		dt := strings.TrimPrefix(rest, "^^")
		dt = strings.Trim(dt, "<>")
		return unescape(lit), "", dt
	}
	return unescape(lit), "", ""
}

func unescape(s string) string {
	s = strings.ReplaceAll(s, `\\`, `\`)
	s = strings.ReplaceAll(s, `\"`, `"`)
	s = strings.ReplaceAll(s, `\n`, "\n")
	return s
}

// ApplyN3Patch applies a minimal N3-Patch (solid:inserts / solid:deletes where clauses simplified).
func ApplyN3Patch(g *Graph, patch string) (*Graph, error) {
	pg, err := ParseTurtle(patch)
	if err != nil {
		// try as raw insert triples
		pg, err = ParseTurtle(patch)
		if err != nil {
			return nil, err
		}
	}
	// If patch contains solid:deletes / solid:inserts as subjects, handle; else treat all as inserts
	out := NewGraph()
	out.Triples = append(out.Triples, g.Triples...)
	delPred := "http://www.w3.org/ns/solid/terms#deletes"
	insPred := "http://www.w3.org/ns/solid/terms#inserts"
	hasOps := false
	for _, t := range pg.Triples {
		if t.Predicate == delPred || t.Predicate == insPred {
			hasOps = true
			break
		}
	}
	if !hasOps {
		for _, t := range pg.Triples {
			out.Add(t)
		}
		return out, nil
	}
	// Simplified: deletes remove matching S P O from graph when listed as triples in patch after marker
	// For MVP of full surface: remove all triples also present in delete set; add inserts
	var deletes, inserts []Triple
	mode := ""
	for _, t := range pg.Triples {
		switch t.Predicate {
		case delPred:
			mode = "del"
			continue
		case insPred:
			mode = "ins"
			continue
		}
		if mode == "del" {
			deletes = append(deletes, t)
		} else if mode == "ins" {
			inserts = append(inserts, t)
		}
	}
	filtered := NewGraph()
	for _, t := range out.Triples {
		drop := false
		for _, d := range deletes {
			if t.Subject == d.Subject && t.Predicate == d.Predicate && t.Object == d.Object {
				drop = true
				break
			}
		}
		if !drop {
			filtered.Add(t)
		}
	}
	for _, t := range inserts {
		filtered.Add(t)
	}
	return filtered, nil
}

// ApplySPARQLUpdate handles INSERT DATA / DELETE DATA subset.
func ApplySPARQLUpdate(g *Graph, update string) (*Graph, error) {
	u := strings.TrimSpace(update)
	out := NewGraph()
	out.Triples = append(out.Triples, g.Triples...)

	upper := strings.ToUpper(u)
	if strings.Contains(upper, "DELETE DATA") {
		block := extractBraceBlock(u, "DELETE DATA")
		dg, err := ParseTurtle(block)
		if err != nil {
			return nil, err
		}
		filtered := NewGraph()
		for _, t := range out.Triples {
			drop := false
			for _, d := range dg.Triples {
				if t.Subject == d.Subject && t.Predicate == d.Predicate && t.Object == d.Object {
					drop = true
					break
				}
			}
			if !drop {
				filtered.Add(t)
			}
		}
		out = filtered
	}
	if strings.Contains(upper, "INSERT DATA") {
		block := extractBraceBlock(u, "INSERT DATA")
		ig, err := ParseTurtle(block)
		if err != nil {
			return nil, err
		}
		for _, t := range ig.Triples {
			out.Add(t)
		}
	}
	return out, nil
}

func extractBraceBlock(src, keyword string) string {
	idx := strings.Index(strings.ToUpper(src), strings.ToUpper(keyword))
	if idx < 0 {
		return ""
	}
	rest := src[idx+len(keyword):]
	start := strings.Index(rest, "{")
	end := strings.LastIndex(rest, "}")
	if start < 0 || end < 0 || end <= start {
		return ""
	}
	return rest[start+1 : end]
}
