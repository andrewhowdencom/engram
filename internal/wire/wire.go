//go:build wireinject
// +build wireinject

// Package wire provides compile-time dependency injection.
package wire

import (
	"github.com/google/wire"
)

// AppSet is the primary provider set for the engram application.
var AppSet = wire.NewSet()
