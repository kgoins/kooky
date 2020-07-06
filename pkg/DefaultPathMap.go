package kooky

// DefaultPathMap stores an os dependant file path
type DefaultPathMap struct {
	pathMap map[string]string
}

// NewDefaultPathMap constructs a new DefaultPathMap
func NewDefaultPathMap() DefaultPathMap {
	return DefaultPathMap{
		pathMap: make(map[string]string),
	}
}

// Add adds a path to the map for a specific os
func (c *DefaultPathMap) Add(operatingSystem string, path string) {
	c.pathMap[operatingSystem] = path
}

// Get returns the path for a specific os
func (c *DefaultPathMap) Get(operatingSystem string) (string, bool) {
	val, found := c.pathMap[operatingSystem]
	return val, found
}
