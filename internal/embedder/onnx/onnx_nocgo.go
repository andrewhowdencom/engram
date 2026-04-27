//go:build !cgo

package onnx

import (
	"context"
	"errors"
)

var errNoCGO = errors.New("onnx embedder requires CGO. build with CGO_ENABLED=1")

func (e *Embedder) loadModel() error {
	return errNoCGO
}

func (e *Embedder) embed(_ context.Context, _ string) ([]float32, error) {
	return nil, errNoCGO
}
