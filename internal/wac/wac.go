package wac

import (
	"context"
	"strings"

	"solid-go/internal/rdf"
	"solid-go/internal/resourcestore"
)

const (
	ACL   = "http://www.w3.org/ns/auth/acl#"
	FOAF  = "http://xmlns.com/foaf/0.1/"
	ModeRead    = ACL + "Read"
	ModeWrite   = ACL + "Write"
	ModeAppend  = ACL + "Append"
	ModeControl = ACL + "Control"
	AgentClassAuthenticated = ACL + "AuthenticatedAgent"
	AgentClassAgent         = FOAF + "Agent"
)

// Mode represents WAC access modes.
type Mode struct {
	Read, Write, Append, Control bool
}

func (m Mode) Allows(required Mode) bool {
	if required.Read && !m.Read && !m.Write && !m.Control {
		return false
	}
	if required.Append && !m.Append && !m.Write && !m.Control {
		return false
	}
	if required.Write && !m.Write && !m.Control {
		return false
	}
	if required.Control && !m.Control {
		return false
	}
	// Read implied by Write/Control already handled above with OR
	if required.Read && !(m.Read || m.Write || m.Control) {
		return false
	}
	return true
}

func ModeFromHTTP(method string, isPatchAppend bool) Mode {
	switch strings.ToUpper(method) {
	case "GET", "HEAD", "OPTIONS":
		return Mode{Read: true}
	case "PUT", "DELETE":
		return Mode{Write: true}
	case "POST":
		return Mode{Append: true}
	case "PATCH":
		if isPatchAppend {
			return Mode{Append: true}
		}
		return Mode{Write: true}
	default:
		return Mode{Read: true}
	}
}

// Checker evaluates Web Access Control.
type Checker struct {
	Store *resourcestore.Store
}

func NewChecker(store *resourcestore.Store) *Checker {
	return &Checker{Store: store}
}

// Allowed returns whether agentWebID may perform required modes on resourcePath.
// Empty agentWebID means public/unauthenticated.
func (c *Checker) Allowed(ctx context.Context, resourcePath, agentWebID string, required Mode) (Mode, bool, error) {
	effective, err := c.EffectiveModes(ctx, resourcePath, agentWebID)
	if err != nil {
		return Mode{}, false, err
	}
	return effective, effective.Allows(required), nil
}

// EffectiveModes walks .acl inheritance.
func (c *Checker) EffectiveModes(ctx context.Context, resourcePath, agentWebID string) (Mode, error) {
	path := strings.TrimPrefix(resourcePath, "/")
	candidates := aclCandidates(path)
	for _, aclPath := range candidates {
		exists, err := c.Store.Exists(ctx, aclPath)
		if err != nil {
			return Mode{}, err
		}
		if !exists {
			continue
		}
		res, err := c.Store.Get(ctx, aclPath)
		if err != nil {
			return Mode{}, err
		}
		g, err := rdf.ParseTurtle(string(res.Body))
		if err != nil {
			continue
		}
		// Nearest existing ACL is authoritative, including an empty grant (deny).
		modes := evaluateACL(g, resourcePath, agentWebID, aclPath == candidates[0])
		return modes, nil
	}
	// Default: public read on root for bootstrap; otherwise deny
	if path == "" || path == "/" {
		return Mode{Read: true}, nil
	}
	// If no ACL anywhere, allow authenticated control for owner-like bootstrap
	if agentWebID != "" {
		return Mode{Read: true, Write: true, Append: true, Control: true}, nil
	}
	return Mode{Read: true}, nil
}

func aclCandidates(path string) []string {
	path = strings.TrimPrefix(path, "/")
	var out []string
	if path != "" {
		out = append(out, resourcestore.AuxPath(path, ".acl"))
	}
	// walk parents
	p := path
	if strings.HasSuffix(p, "/") {
		p = strings.TrimSuffix(p, "/")
	}
	for {
		dir := ""
		if idx := strings.LastIndex(p, "/"); idx >= 0 {
			dir = p[:idx]
			p = dir
		} else {
			dir = ""
			p = ""
		}
		if dir == "" {
			out = append(out, ".acl")
			break
		}
		out = append(out, dir+"/.acl")
		if dir == "" {
			break
		}
	}
	// dedupe
	seen := map[string]bool{}
	var uniq []string
	for _, a := range out {
		if !seen[a] {
			seen[a] = true
			uniq = append(uniq, a)
		}
	}
	return uniq
}

func evaluateACL(g *rdf.Graph, resourcePath, agentWebID string, isDirect bool) Mode {
	var mode Mode
	// Find authorization nodes
	authPred := ACL + "accessTo"
	defaultPred := ACL + "default"
	agentPred := ACL + "agent"
	agentClassPred := ACL + "agentClass"
	modePred := ACL + "mode"

	resourceIRI := strings.TrimPrefix(resourcePath, "/")
	auths := map[string]bool{}
	for _, t := range g.Triples {
		if t.Predicate == authPred || t.Predicate == defaultPred {
			// match resource or apply default from parent
			obj := strings.TrimPrefix(t.Object, "/")
			if t.Predicate == authPred {
				if obj == resourceIRI || strings.HasSuffix(t.Object, resourceIRI) || resourceIRI == "" {
					auths[t.Subject] = true
				}
			}
			if t.Predicate == defaultPred && !isDirect {
				auths[t.Subject] = true
			}
			if t.Predicate == defaultPred && isDirect {
				// defaults also apply to container children; for direct resource check accessTo
			}
			// always consider default rules for inheritance
			if t.Predicate == defaultPred {
				auths[t.Subject] = true
			}
		}
	}
	// If nothing matched via accessTo, still consider all auth nodes with agent (lenient bootstrap)
	if len(auths) == 0 {
		for _, t := range g.Triples {
			if t.Predicate == agentPred || t.Predicate == agentClassPred {
				auths[t.Subject] = true
			}
		}
	}

	for auth := range auths {
		applies := false
		for _, ag := range g.Objects(auth, agentPred) {
			if agentWebID != "" && (ag == agentWebID || strings.TrimSuffix(ag, "#me") == strings.TrimSuffix(agentWebID, "#me")) {
				applies = true
			}
		}
		for _, ac := range g.Objects(auth, agentClassPred) {
			if ac == AgentClassAgent {
				applies = true // public
			}
			if ac == AgentClassAuthenticated && agentWebID != "" {
				applies = true
			}
		}
		if !applies {
			continue
		}
		for _, m := range g.Objects(auth, modePred) {
			switch m {
			case ModeRead:
				mode.Read = true
			case ModeWrite:
				mode.Write = true
			case ModeAppend:
				mode.Append = true
			case ModeControl:
				mode.Control = true
			}
		}
	}
	return mode
}

// DefaultPublicACL returns Turtle granting public Read, authenticated Write, and owner Control.
func DefaultPublicACL(resourceURL, ownerWebID string) string {
	g := rdf.NewGraph()
	prefixes := map[string]string{
		"acl":  ACL,
		"foaf": FOAF,
	}
	g.AddIRI("#public", "http://www.w3.org/1999/02/22-rdf-syntax-ns#type", ACL+"Authorization")
	g.AddIRI("#public", ACL+"agentClass", FOAF+"Agent")
	g.AddIRI("#public", ACL+"accessTo", resourceURL)
	g.AddIRI("#public", ACL+"default", resourceURL)
	g.AddIRI("#public", ACL+"mode", ModeRead)

	g.AddIRI("#authenticated", "http://www.w3.org/1999/02/22-rdf-syntax-ns#type", ACL+"Authorization")
	g.AddIRI("#authenticated", ACL+"agentClass", AgentClassAuthenticated)
	g.AddIRI("#authenticated", ACL+"accessTo", resourceURL)
	g.AddIRI("#authenticated", ACL+"default", resourceURL)
	g.AddIRI("#authenticated", ACL+"mode", ModeRead)
	g.AddIRI("#authenticated", ACL+"mode", ModeWrite)
	g.AddIRI("#authenticated", ACL+"mode", ModeAppend)

	if ownerWebID != "" {
		addOwnerAuth(g, resourceURL, ownerWebID)
	}
	return rdf.SerializeTurtle(g, prefixes)
}

func addOwnerAuth(g *rdf.Graph, resourceURL, ownerWebID string) {
	g.AddIRI("#owner", "http://www.w3.org/1999/02/22-rdf-syntax-ns#type", ACL+"Authorization")
	g.AddIRI("#owner", ACL+"agent", ownerWebID)
	g.AddIRI("#owner", ACL+"accessTo", resourceURL)
	g.AddIRI("#owner", ACL+"default", resourceURL)
	g.AddIRI("#owner", ACL+"mode", ModeRead)
	g.AddIRI("#owner", ACL+"mode", ModeWrite)
	g.AddIRI("#owner", ACL+"mode", ModeAppend)
	g.AddIRI("#owner", ACL+"mode", ModeControl)
}

// OwnerOnlyACL grants the owner Read/Write/Append/Control and nobody else.
func OwnerOnlyACL(resourceURL, ownerWebID string) string {
	g := rdf.NewGraph()
	prefixes := map[string]string{
		"acl":  ACL,
		"foaf": FOAF,
	}
	addOwnerAuth(g, resourceURL, ownerWebID)
	return rdf.SerializeTurtle(g, prefixes)
}

// PublicReadACL grants foaf:Agent Read and the owner full control.
func PublicReadACL(resourceURL, ownerWebID string) string {
	g := rdf.NewGraph()
	prefixes := map[string]string{
		"acl":  ACL,
		"foaf": FOAF,
	}
	g.AddIRI("#public", "http://www.w3.org/1999/02/22-rdf-syntax-ns#type", ACL+"Authorization")
	g.AddIRI("#public", ACL+"agentClass", FOAF+"Agent")
	g.AddIRI("#public", ACL+"accessTo", resourceURL)
	g.AddIRI("#public", ACL+"default", resourceURL)
	g.AddIRI("#public", ACL+"mode", ModeRead)
	if ownerWebID != "" {
		addOwnerAuth(g, resourceURL, ownerWebID)
	}
	return rdf.SerializeTurtle(g, prefixes)
}
