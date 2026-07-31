//go:build cssstub

package ldp

// Container represents a Linked Data Platform container
type Container struct {
	Path       string
	Resources  []string
	Containers []string
}

// NewContainer creates a new container
func NewContainer(path string) *Container {
	return &Container{
		Path:       path,
		Resources:  make([]string, 0),
		Containers: make([]string, 0),
	}
}

// AddResource adds a resource to the container
func (c *Container) AddResource(path string) {
	c.Resources = append(c.Resources, path)
}

// AddContainer adds a sub-container to the container
func (c *Container) AddContainer(path string) {
	c.Containers = append(c.Containers, path)
}

// RemoveResource removes a resource from the container
func (c *Container) RemoveResource(path string) {
	for i, r := range c.Resources {
		if r == path {
			c.Resources = append(c.Resources[:i], c.Resources[i+1:]...)
			break
		}
	}
}

// RemoveContainer removes a sub-container from the container
func (c *Container) RemoveContainer(path string) {
	for i, sub := range c.Containers {
		if sub == path {
			c.Containers = append(c.Containers[:i], c.Containers[i+1:]...)
			break
		}
	}
}
