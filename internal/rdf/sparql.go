package rdf

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// SPARQLResult is a SPARQL 1.1 JSON results document.
type SPARQLResult struct {
	Head struct {
		Vars []string `json:"vars"`
	} `json:"head"`
	Results struct {
		Bindings []map[string]SPARQLValue `json:"bindings"`
	} `json:"results"`
}

// SPARQLValue is one binding cell.
type SPARQLValue struct {
	Type     string `json:"type"`
	Value    string `json:"value"`
	Datatype string `json:"datatype,omitempty"`
	Lang     string `json:"xml:lang,omitempty"`
}

type sparqlQuery struct {
	prefixes map[string]string
	vars     []string
	star     bool
	distinct bool
	patterns []triplePat
	optional [][]triplePat
	filters  []string
	order    []orderKey
	limit    int
	offset   int
}

type triplePat struct {
	s, p, o sparqlTerm
}

type sparqlTerm struct {
	kind    termKind
	value   string
	literal bool
}

type termKind int

const (
	termVar termKind = iota
	termIRI
	termLit
)

type orderKey struct {
	variable string
	desc     bool
}

type binding map[string]boundVal

type boundVal struct {
	iri     string
	lit     string
	isLit   bool
	dt      string
	lang    string
}

// DefaultSPARQLPrefixes are available without PREFIX in the query.
func DefaultSPARQLPrefixes(baseURL string) map[string]string {
	oid := strings.TrimRight(baseURL, "/") + "/ns/records#"
	return map[string]string{
		"rdf":    "http://www.w3.org/1999/02/22-rdf-syntax-ns#",
		"rdfs":   "http://www.w3.org/2000/01/rdf-schema#",
		"xsd":    "http://www.w3.org/2001/XMLSchema#",
		"schema": "https://schema.org/",
		"dcat":   "http://www.w3.org/ns/dcat#",
		"dct":    "http://purl.org/dc/terms/",
		"foaf":   "http://xmlns.com/foaf/0.1/",
		"oid":    oid,
	}
}

// QuerySPARQL evaluates a SPARQL SELECT subset against g.
func QuerySPARQL(g *Graph, query string, extraPrefixes map[string]string) (*SPARQLResult, error) {
	q, err := parseSPARQL(query, extraPrefixes)
	if err != nil {
		return nil, err
	}
	rows := []binding{{}}
	for _, p := range q.patterns {
		rows = joinPattern(g, rows, p)
	}
	for _, group := range q.optional {
		var next []binding
		for _, row := range rows {
			ext := []binding{row}
			for _, p := range group {
				ext = joinPattern(g, ext, p)
			}
			if len(ext) == 0 {
				next = append(next, row)
				continue
			}
			next = append(next, ext...)
		}
		rows = next
	}
	var filtered []binding
	for _, row := range rows {
		ok := true
		for _, f := range q.filters {
			pass, err := evalFilter(f, row)
			if err != nil {
				return nil, err
			}
			if !pass {
				ok = false
				break
			}
		}
		if ok {
			filtered = append(filtered, row)
		}
	}
	rows = filtered

	proj := q.vars
	if q.star || len(proj) == 0 {
		seen := map[string]bool{}
		for _, row := range rows {
			for k := range row {
				if !seen[k] {
					seen[k] = true
					proj = append(proj, k)
				}
			}
		}
		sort.Strings(proj)
	}

	if q.distinct {
		rows = distinctRows(rows, proj)
	}
	if len(q.order) > 0 {
		sort.SliceStable(rows, func(i, j int) bool {
			return lessBinding(rows[i], rows[j], q.order)
		})
	}
	if q.offset > 0 {
		if q.offset >= len(rows) {
			rows = nil
		} else {
			rows = rows[q.offset:]
		}
	}
	if q.limit > 0 && len(rows) > q.limit {
		rows = rows[:q.limit]
	}

	out := &SPARQLResult{}
	out.Head.Vars = proj
	for _, row := range rows {
		cell := map[string]SPARQLValue{}
		for _, v := range proj {
			val, ok := row[v]
			if !ok {
				continue
			}
			cell[v] = val.toSPARQL()
		}
		out.Results.Bindings = append(out.Results.Bindings, cell)
	}
	if out.Results.Bindings == nil {
		out.Results.Bindings = []map[string]SPARQLValue{}
	}
	return out, nil
}

func (v boundVal) toSPARQL() SPARQLValue {
	if v.isLit {
		return SPARQLValue{Type: "literal", Value: v.lit, Datatype: v.dt, Lang: v.lang}
	}
	if strings.HasPrefix(v.iri, "_:") {
		return SPARQLValue{Type: "bnode", Value: strings.TrimPrefix(v.iri, "_:")}
	}
	return SPARQLValue{Type: "uri", Value: v.iri}
}

func (v boundVal) str() string {
	if v.isLit {
		return v.lit
	}
	return v.iri
}

func joinPattern(g *Graph, rows []binding, p triplePat) []binding {
	var next []binding
	for _, row := range rows {
		sWant, sOK := bindTerm(p.s, row)
		pWant, pOK := bindTerm(p.p, row)
		oWant, oOK := bindTerm(p.o, row)
		if (p.s.kind != termVar && !sOK) || (p.p.kind != termVar && !pOK) || (p.o.kind != termVar && !oOK) {
			continue
		}
		for _, t := range g.Triples {
			if sOK && t.Subject != sWant.iri {
				continue
			}
			if pOK && t.Predicate != pWant.iri {
				continue
			}
			if oOK {
				if oWant.isLit {
					if !t.ObjectLit || t.Object != oWant.lit {
						continue
					}
				} else if t.Object != oWant.iri {
					continue
				}
			}
			nb := cloneBinding(row)
			if p.s.kind == termVar {
				if !bindIRI(nb, p.s.value, t.Subject) {
					continue
				}
			}
			if p.p.kind == termVar {
				if !bindIRI(nb, p.p.value, t.Predicate) {
					continue
				}
			}
			if p.o.kind == termVar {
				if !bindObject(nb, p.o.value, t) {
					continue
				}
			}
			next = append(next, nb)
		}
	}
	return next
}

func bindTerm(term sparqlTerm, row binding) (boundVal, bool) {
	if term.kind == termVar {
		v, ok := row[term.value]
		return v, ok
	}
	if term.kind == termLit || term.literal {
		return boundVal{lit: term.value, isLit: true}, true
	}
	return boundVal{iri: term.value}, true
}

func bindIRI(row binding, name, iri string) bool {
	if cur, ok := row[name]; ok {
		return !cur.isLit && cur.iri == iri
	}
	row[name] = boundVal{iri: iri}
	return true
}

func bindObject(row binding, name string, t Triple) bool {
	var v boundVal
	if t.ObjectLit {
		v = boundVal{lit: t.Object, isLit: true, dt: t.Datatype, lang: t.Lang}
	} else {
		v = boundVal{iri: t.Object}
	}
	if cur, ok := row[name]; ok {
		return cur.isLit == v.isLit && cur.str() == v.str()
	}
	row[name] = v
	return true
}

func cloneBinding(row binding) binding {
	out := make(binding, len(row)+4)
	for k, v := range row {
		out[k] = v
	}
	return out
}

func distinctRows(rows []binding, vars []string) []binding {
	seen := map[string]bool{}
	var out []binding
	for _, row := range rows {
		var b strings.Builder
		for _, v := range vars {
			val, ok := row[v]
			b.WriteByte('|')
			if !ok {
				continue
			}
			if val.isLit {
				b.WriteString("L:")
				b.WriteString(val.lit)
			} else {
				b.WriteString("I:")
				b.WriteString(val.iri)
			}
		}
		key := b.String()
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, row)
	}
	return out
}

func lessBinding(a, b binding, keys []orderKey) bool {
	for _, k := range keys {
		av, aok := a[k.variable]
		bv, bok := b[k.variable]
		if !aok && !bok {
			continue
		}
		if !aok {
			return !k.desc
		}
		if !bok {
			return k.desc
		}
		cmp := strings.Compare(av.str(), bv.str())
		if an, aok := strconv.ParseFloat(av.str(), 64); aok == nil {
			if bn, bok := strconv.ParseFloat(bv.str(), 64); bok == nil {
				if an < bn {
					cmp = -1
				} else if an > bn {
					cmp = 1
				} else {
					cmp = 0
				}
			}
		}
		if cmp == 0 {
			continue
		}
		if k.desc {
			return cmp > 0
		}
		return cmp < 0
	}
	return false
}

func parseSPARQL(src string, extra map[string]string) (*sparqlQuery, error) {
	q := &sparqlQuery{
		prefixes: DefaultSPARQLPrefixes(""),
		limit:    0,
	}
	for k, v := range extra {
		q.prefixes[k] = v
	}
	s := stripSPARQLComments(src)
	s = strings.TrimSpace(s)
	for {
		s = strings.TrimSpace(s)
		upper := strings.ToUpper(s)
		if strings.HasPrefix(upper, "PREFIX") {
			rest := strings.TrimSpace(s[6:])
			name, rest, err := readPrefixedName(rest)
			if err != nil {
				return nil, err
			}
			iri, rest, err := readIRI(rest)
			if err != nil {
				return nil, err
			}
			q.prefixes[name] = iri
			s = rest
			continue
		}
		break
	}
	s = strings.TrimSpace(s)
	upper := strings.ToUpper(s)
	if !strings.HasPrefix(upper, "SELECT") {
		return nil, fmt.Errorf("sparql: only SELECT queries are supported")
	}
	s = strings.TrimSpace(s[6:])
	if strings.HasPrefix(strings.ToUpper(s), "DISTINCT") {
		q.distinct = true
		s = strings.TrimSpace(s[8:])
	}
	if strings.HasPrefix(s, "*") {
		q.star = true
		s = strings.TrimSpace(s[1:])
	} else {
		for {
			s = strings.TrimSpace(s)
			if s == "" || strings.HasPrefix(strings.ToUpper(s), "WHERE") || s[0] == '{' {
				break
			}
			if s[0] != '?' && s[0] != '$' {
				return nil, fmt.Errorf("sparql: expected variable in SELECT")
			}
			name, rest := readVar(s)
			q.vars = append(q.vars, name)
			s = rest
		}
	}
	s = strings.TrimSpace(s)
	if strings.HasPrefix(strings.ToUpper(s), "WHERE") {
		s = strings.TrimSpace(s[5:])
	}
	if !strings.HasPrefix(s, "{") {
		return nil, fmt.Errorf("sparql: expected WHERE { ... }")
	}
	block, rest, err := readBrace(s)
	if err != nil {
		return nil, err
	}
	if err := parseGraphPattern(block, q); err != nil {
		return nil, err
	}
	s = strings.TrimSpace(rest)
	for s != "" {
		upper = strings.ToUpper(s)
		switch {
		case strings.HasPrefix(upper, "ORDER BY"):
			s = strings.TrimSpace(s[8:])
			for s != "" {
				s = strings.TrimSpace(s)
				u := strings.ToUpper(s)
				if strings.HasPrefix(u, "LIMIT") || strings.HasPrefix(u, "OFFSET") {
					break
				}
				desc := false
				if strings.HasPrefix(u, "DESC") || strings.HasPrefix(u, "ASC") {
					fn := "DESC"
					if strings.HasPrefix(u, "ASC") {
						fn = "ASC"
					}
					desc = fn == "DESC"
					s = strings.TrimSpace(s[len(fn):])
					if strings.HasPrefix(s, "(") {
						inner, more, err := readParen(s)
						if err != nil {
							return nil, err
						}
						name, _ := readVar(strings.TrimSpace(inner))
						q.order = append(q.order, orderKey{variable: name, desc: desc})
						s = more
						continue
					}
				}
				if s == "" || (s[0] != '?' && s[0] != '$') {
					break
				}
				name, more := readVar(s)
				q.order = append(q.order, orderKey{variable: name, desc: desc})
				s = more
			}
		case strings.HasPrefix(upper, "LIMIT"):
			n, more, err := readInt(strings.TrimSpace(s[5:]))
			if err != nil {
				return nil, err
			}
			q.limit = n
			s = more
		case strings.HasPrefix(upper, "OFFSET"):
			n, more, err := readInt(strings.TrimSpace(s[6:]))
			if err != nil {
				return nil, err
			}
			q.offset = n
			s = more
		default:
			return nil, fmt.Errorf("sparql: unexpected %q", clip(s, 40))
		}
	}
	return q, nil
}

func parseGraphPattern(src string, q *sparqlQuery) error {
	s := strings.TrimSpace(src)
	var pending *triplePat
	semi := false
	for strings.TrimSpace(s) != "" {
		s = strings.TrimSpace(s)
		upper := strings.ToUpper(s)
		if strings.HasPrefix(upper, "OPTIONAL") {
			rest := strings.TrimSpace(s[8:])
			block, more, err := readBrace(rest)
			if err != nil {
				return err
			}
			sub := &sparqlQuery{prefixes: q.prefixes}
			if err := parseGraphPattern(block, sub); err != nil {
				return err
			}
			q.optional = append(q.optional, sub.patterns)
			q.filters = append(q.filters, sub.filters...)
			s = more
			pending = nil
			semi = false
			continue
		}
		if strings.HasPrefix(upper, "FILTER") {
			rest := strings.TrimSpace(s[6:])
			if !strings.HasPrefix(rest, "(") {
				return fmt.Errorf("sparql: FILTER must use parentheses")
			}
			inner, more, err := readParen(rest)
			if err != nil {
				return err
			}
			q.filters = append(q.filters, strings.TrimSpace(inner))
			s = more
			pending = nil
			semi = false
			continue
		}
		if s[0] == '.' {
			s = s[1:]
			pending = nil
			semi = false
			continue
		}
		if s[0] == ';' {
			if pending == nil {
				return fmt.Errorf("sparql: ';' without a subject")
			}
			semi = true
			s = s[1:]
			continue
		}
		if s[0] == ',' {
			return fmt.Errorf("sparql: comma lists are not supported")
		}
		t, rest, err := readTerm(s, q.prefixes)
		if err != nil {
			return err
		}
		s = rest
		if pending == nil || !semi {
			p, rest, err := readTerm(s, q.prefixes)
			if err != nil {
				return err
			}
			o, rest, err := readTerm(rest, q.prefixes)
			if err != nil {
				return err
			}
			pat := triplePat{s: t, p: p, o: o}
			q.patterns = append(q.patterns, pat)
			pending = &q.patterns[len(q.patterns)-1]
			s = rest
			semi = false
			continue
		}
		// same subject, new predicate/object
		o, rest, err := readTerm(s, q.prefixes)
		if err != nil {
			return err
		}
		pat := triplePat{s: pending.s, p: t, o: o}
		q.patterns = append(q.patterns, pat)
		pending = &q.patterns[len(q.patterns)-1]
		s = rest
		semi = false
	}
	return nil
}

func readTerm(s string, prefixes map[string]string) (sparqlTerm, string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return sparqlTerm{}, s, fmt.Errorf("sparql: unexpected end of term")
	}
	if s[0] == '?' || s[0] == '$' {
		name, rest := readVar(s)
		return sparqlTerm{kind: termVar, value: name}, rest, nil
	}
	if strings.HasPrefix(s, "_:") {
		i := 2
		for i < len(s) && (isVarChar(s[i])) {
			i++
		}
		return sparqlTerm{kind: termIRI, value: s[:i]}, s[i:], nil
	}
	if s[0] == '<' {
		iri, rest, err := readIRI(s)
		if err != nil {
			return sparqlTerm{}, s, err
		}
		return sparqlTerm{kind: termIRI, value: iri}, rest, nil
	}
	if s[0] == '"' || s[0] == '\'' {
		lit, rest, err := readQuoted(s)
		if err != nil {
			return sparqlTerm{}, s, err
		}
		return sparqlTerm{kind: termLit, value: lit, literal: true}, rest, nil
	}
	if strings.HasPrefix(s, "a") && (len(s) == 1 || isSep(s[1])) {
		return sparqlTerm{kind: termIRI, value: rdfType}, s[1:], nil
	}
	// number or prefixed name
	if s[0] == '-' || s[0] == '+' || unicode.IsDigit(rune(s[0])) {
		i := 0
		if s[0] == '-' || s[0] == '+' {
			i++
		}
		for i < len(s) && (unicode.IsDigit(rune(s[i])) || s[i] == '.') {
			i++
		}
		return sparqlTerm{kind: termLit, value: s[:i], literal: true}, s[i:], nil
	}
	i := 0
	for i < len(s) && !isSep(s[i]) {
		i++
	}
	raw := s[:i]
	if raw == "" {
		return sparqlTerm{}, s, fmt.Errorf("sparql: empty term")
	}
	return sparqlTerm{kind: termIRI, value: expand(raw, prefixes)}, s[i:], nil
}

func readVar(s string) (string, string) {
	s = strings.TrimSpace(s)
	if s == "" || (s[0] != '?' && s[0] != '$') {
		return "", s
	}
	i := 1
	for i < len(s) && isVarChar(s[i]) {
		i++
	}
	return s[1:i], s[i:]
}

func isVarChar(c byte) bool {
	return unicode.IsLetter(rune(c)) || unicode.IsDigit(rune(c)) || c == '_'
}

func isSep(c byte) bool {
	return unicode.IsSpace(rune(c)) || c == '.' || c == ';' || c == ',' || c == '{' || c == '}' || c == '(' || c == ')'
}

func readIRI(s string) (string, string, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "<") {
		return "", s, fmt.Errorf("sparql: expected IRI")
	}
	end := strings.IndexByte(s, '>')
	if end < 0 {
		return "", s, fmt.Errorf("sparql: unclosed IRI")
	}
	return s[1:end], s[end+1:], nil
}

func readPrefixedName(s string) (string, string, error) {
	s = strings.TrimSpace(s)
	end := strings.IndexByte(s, ':')
	if end < 0 {
		return "", s, fmt.Errorf("sparql: expected prefix name")
	}
	return strings.TrimSpace(s[:end]), s[end+1:], nil
}

func readQuoted(s string) (string, string, error) {
	quote := s[0]
	i := 1
	var b strings.Builder
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			b.WriteByte(s[i+1])
			i += 2
			continue
		}
		if s[i] == quote {
			i++
			for i < len(s) && (s[i] == '@' || s[i] == '^' || s[i] == '<' || s[i] == ':' || isVarChar(s[i])) {
				i++
			}
			return b.String(), s[i:], nil
		}
		b.WriteByte(s[i])
		i++
	}
	return "", s, fmt.Errorf("sparql: unclosed string")
}

func readBrace(s string) (string, string, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "{") {
		return "", s, fmt.Errorf("sparql: expected '{'")
	}
	depth := 0
	inQ := byte(0)
	esc := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if esc {
			esc = false
			continue
		}
		if inQ != 0 {
			if c == '\\' {
				esc = true
				continue
			}
			if c == inQ {
				inQ = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			inQ = c
			continue
		}
		if c == '{' {
			depth++
		}
		if c == '}' {
			depth--
			if depth == 0 {
				return s[1:i], s[i+1:], nil
			}
		}
	}
	return "", s, fmt.Errorf("sparql: unclosed '{'")
}

func readParen(s string) (string, string, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "(") {
		return "", s, fmt.Errorf("sparql: expected '('")
	}
	depth := 0
	inQ := byte(0)
	esc := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if esc {
			esc = false
			continue
		}
		if inQ != 0 {
			if c == '\\' {
				esc = true
				continue
			}
			if c == inQ {
				inQ = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			inQ = c
			continue
		}
		if c == '(' {
			depth++
		}
		if c == ')' {
			depth--
			if depth == 0 {
				return s[1:i], s[i+1:], nil
			}
		}
	}
	return "", s, fmt.Errorf("sparql: unclosed '('")
}

func readInt(s string) (int, string, error) {
	s = strings.TrimSpace(s)
	i := 0
	for i < len(s) && unicode.IsDigit(rune(s[i])) {
		i++
	}
	if i == 0 {
		return 0, s, fmt.Errorf("sparql: expected integer")
	}
	n, _ := strconv.Atoi(s[:i])
	return n, s[i:], nil
}

func stripSPARQLComments(s string) string {
	var b strings.Builder
	inQ := byte(0)
	esc := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if esc {
			b.WriteByte(c)
			esc = false
			continue
		}
		if inQ != 0 {
			if c == '\\' {
				esc = true
				b.WriteByte(c)
				continue
			}
			if c == inQ {
				inQ = 0
			}
			b.WriteByte(c)
			continue
		}
		if c == '"' || c == '\'' {
			inQ = c
			b.WriteByte(c)
			continue
		}
		if c == '#' {
			for i < len(s) && s[i] != '\n' {
				i++
			}
			if i < len(s) {
				b.WriteByte('\n')
			}
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func evalFilter(expr string, row binding) (bool, error) {
	v, err := evalExpr(expr, row)
	if err != nil {
		return false, err
	}
	return v.truth(), nil
}

type evalVal struct {
	str   string
	num   float64
	isNum bool
	isBool bool
	truthy bool
}

func (v evalVal) truth() bool {
	if v.isBool {
		return v.truthy
	}
	if v.isNum {
		return v.num != 0
	}
	return v.str != ""
}

func evalExpr(expr string, row binding) (evalVal, error) {
	expr = strings.TrimSpace(expr)
	orParts := splitTop(expr, "||")
	if len(orParts) > 1 {
		for _, p := range orParts {
			v, err := evalExpr(p, row)
			if err != nil {
				return evalVal{}, err
			}
			if v.truth() {
				return evalVal{isBool: true, truthy: true}, nil
			}
		}
		return evalVal{isBool: true, truthy: false}, nil
	}
	andParts := splitTop(expr, "&&")
	if len(andParts) > 1 {
		for _, p := range andParts {
			v, err := evalExpr(p, row)
			if err != nil {
				return evalVal{}, err
			}
			if !v.truth() {
				return evalVal{isBool: true, truthy: false}, nil
			}
		}
		return evalVal{isBool: true, truthy: true}, nil
	}
	for _, op := range []string{"!=", "<=", ">=", "=", "<", ">"} {
		parts := splitTop(expr, op)
		if len(parts) == 2 {
			l, err := evalPrimary(parts[0], row)
			if err != nil {
				return evalVal{}, err
			}
			r, err := evalPrimary(parts[1], row)
			if err != nil {
				return evalVal{}, err
			}
			cmp := compareEval(l, r)
			ok := false
			switch op {
			case "=":
				ok = cmp == 0
			case "!=":
				ok = cmp != 0
			case "<":
				ok = cmp < 0
			case ">":
				ok = cmp > 0
			case "<=":
				ok = cmp <= 0
			case ">=":
				ok = cmp >= 0
			}
			return evalVal{isBool: true, truthy: ok}, nil
		}
	}
	return evalPrimary(expr, row)
}

func evalPrimary(expr string, row binding) (evalVal, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return evalVal{}, fmt.Errorf("sparql: empty FILTER expression")
	}
	if expr[0] == '(' {
		inner, rest, err := readParen(expr)
		if err != nil {
			return evalVal{}, err
		}
		if strings.TrimSpace(rest) != "" {
			return evalVal{}, fmt.Errorf("sparql: trailing %q in FILTER", rest)
		}
		return evalExpr(inner, row)
	}
	upper := strings.ToUpper(expr)
	switch {
	case strings.HasPrefix(upper, "CONTAINS"):
		args, err := funcArgs(expr, "CONTAINS")
		if err != nil {
			return evalVal{}, err
		}
		if len(args) != 2 {
			return evalVal{}, fmt.Errorf("sparql: CONTAINS takes 2 arguments")
		}
		a, err := evalPrimary(args[0], row)
		if err != nil {
			return evalVal{}, err
		}
		b, err := evalPrimary(args[1], row)
		if err != nil {
			return evalVal{}, err
		}
		return evalVal{isBool: true, truthy: strings.Contains(a.str, b.str)}, nil
	case strings.HasPrefix(upper, "LCASE"):
		args, err := funcArgs(expr, "LCASE")
		if err != nil {
			return evalVal{}, err
		}
		v, err := evalPrimary(args[0], row)
		if err != nil {
			return evalVal{}, err
		}
		v.str = strings.ToLower(v.str)
		return v, nil
	case strings.HasPrefix(upper, "UCASE"):
		args, err := funcArgs(expr, "UCASE")
		if err != nil {
			return evalVal{}, err
		}
		v, err := evalPrimary(args[0], row)
		if err != nil {
			return evalVal{}, err
		}
		v.str = strings.ToUpper(v.str)
		return v, nil
	case strings.HasPrefix(upper, "STR"):
		args, err := funcArgs(expr, "STR")
		if err != nil {
			return evalVal{}, err
		}
		return evalPrimary(args[0], row)
	case strings.HasPrefix(upper, "REGEX"):
		args, err := funcArgs(expr, "REGEX")
		if err != nil {
			return evalVal{}, err
		}
		if len(args) < 2 {
			return evalVal{}, fmt.Errorf("sparql: REGEX takes 2 or 3 arguments")
		}
		a, err := evalPrimary(args[0], row)
		if err != nil {
			return evalVal{}, err
		}
		pat, err := evalPrimary(args[1], row)
		if err != nil {
			return evalVal{}, err
		}
		flags := ""
		if len(args) > 2 {
			f, err := evalPrimary(args[2], row)
			if err != nil {
				return evalVal{}, err
			}
			flags = f.str
		}
		pattern := pat.str
		if strings.Contains(flags, "i") && !strings.HasPrefix(pattern, "(?") {
			pattern = "(?i)" + pattern
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return evalVal{}, fmt.Errorf("sparql: REGEX: %w", err)
		}
		return evalVal{isBool: true, truthy: re.MatchString(a.str)}, nil
	case strings.HasPrefix(upper, "BOUND"):
		args, err := funcArgs(expr, "BOUND")
		if err != nil {
			return evalVal{}, err
		}
		name, _ := readVar(strings.TrimSpace(args[0]))
		_, ok := row[name]
		return evalVal{isBool: true, truthy: ok}, nil
	}
	if expr[0] == '?' || expr[0] == '$' {
		name, rest := readVar(expr)
		if strings.TrimSpace(rest) != "" {
			return evalVal{}, fmt.Errorf("sparql: unexpected %q after variable", rest)
		}
		val, ok := row[name]
		if !ok {
			return evalVal{}, nil
		}
		return stringEval(val.str()), nil
	}
	if expr[0] == '"' || expr[0] == '\'' {
		lit, rest, err := readQuoted(expr)
		if err != nil {
			return evalVal{}, err
		}
		if strings.TrimSpace(rest) != "" {
			return evalVal{}, fmt.Errorf("sparql: trailing %q after literal", rest)
		}
		return stringEval(lit), nil
	}
	if expr[0] == '<' {
		iri, rest, err := readIRI(expr)
		if err != nil {
			return evalVal{}, err
		}
		if strings.TrimSpace(rest) != "" {
			return evalVal{}, fmt.Errorf("sparql: trailing %q after IRI", rest)
		}
		return stringEval(iri), nil
	}
	if n, err := strconv.ParseFloat(expr, 64); err == nil {
		return evalVal{str: expr, num: n, isNum: true}, nil
	}
	if strings.EqualFold(expr, "true") {
		return evalVal{isBool: true, truthy: true, str: "true"}, nil
	}
	if strings.EqualFold(expr, "false") {
		return evalVal{isBool: true, truthy: false, str: "false"}, nil
	}
	return stringEval(expr), nil
}

func stringEval(s string) evalVal {
	if n, err := strconv.ParseFloat(s, 64); err == nil {
		return evalVal{str: s, num: n, isNum: true}
	}
	return evalVal{str: s}
}

func compareEval(a, b evalVal) int {
	if a.isNum && b.isNum {
		if a.num < b.num {
			return -1
		}
		if a.num > b.num {
			return 1
		}
		return 0
	}
	return strings.Compare(a.str, b.str)
}

func funcArgs(expr, name string) ([]string, error) {
	expr = strings.TrimSpace(expr)
	if !strings.HasPrefix(strings.ToUpper(expr), strings.ToUpper(name)) {
		return nil, fmt.Errorf("sparql: expected %s", name)
	}
	rest := strings.TrimSpace(expr[len(name):])
	inner, leftover, err := readParen(rest)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(leftover) != "" {
		return nil, fmt.Errorf("sparql: trailing %q after %s", leftover, name)
	}
	return splitTop(inner, ","), nil
}

func splitTop(s, sep string) []string {
	var parts []string
	depthP, depthB := 0, 0
	inQ := byte(0)
	esc := false
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if esc {
			esc = false
			continue
		}
		if inQ != 0 {
			if c == '\\' {
				esc = true
				continue
			}
			if c == inQ {
				inQ = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			inQ = c
			continue
		}
		if c == '(' {
			depthP++
			continue
		}
		if c == ')' {
			depthP--
			continue
		}
		if c == '{' {
			depthB++
			continue
		}
		if c == '}' {
			depthB--
			continue
		}
		if depthP == 0 && depthB == 0 && strings.HasPrefix(s[i:], sep) {
			parts = append(parts, strings.TrimSpace(s[start:i]))
			i += len(sep) - 1
			start = i + 1
		}
	}
	parts = append(parts, strings.TrimSpace(s[start:]))
	if len(parts) == 1 {
		return parts
	}
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
