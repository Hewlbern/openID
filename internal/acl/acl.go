//go:build cssstub

package acl

// ACL represents an Access Control List
type ACL struct {
	Path       string
	AccessTo   []string
	Default    []string
	Access     []Access
	DefaultFor []Access
}

// Access represents an access control entry
type Access struct {
	Agent     string
	Group     string
	Origin    string
	Mode      []string
	Resource  string
	Inherited bool
}
