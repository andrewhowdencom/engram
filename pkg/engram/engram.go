// Package engram provides the core library for managing agent memory.
package engram

// Engram is the primary abstraction for the memory agent store.
type Engram struct{}

// New creates a new Engram instance.
func New() *Engram {
	return &Engram{}
}
