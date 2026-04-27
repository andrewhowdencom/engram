// Package embedder provides a factory for creating embedders from application
// configuration. It bridges the core Embedder interface with concrete
// implementations (ONNX, no-op).
package embedder

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/andrewhowdencom/engram/internal/embedder/noop"
	"github.com/andrewhowdencom/engram/internal/embedder/onnx"
	"github.com/andrewhowdencom/engram/pkg/engram"
	"github.com/spf13/viper"
)

// NewFromConfig creates an embedder based on the current Viper configuration.
// Supported types: "noop", "onnx".
func NewFromConfig() (engram.Embedder, error) {
	typ := viper.GetString("embedder.type")
	switch typ {
	case "", "noop":
		return noop.New(), nil

	case "onnx":
		return newONNXFromConfig()

	default:
		return nil, fmt.Errorf("unknown embedder type %q", typ)
	}
}

func newONNXFromConfig() (engram.Embedder, error) {
	modelPath := viper.GetString("embedder.onnx.model_path")
	vocabPath := viper.GetString("embedder.onnx.vocab_path")

	// If paths are empty, use the default model directory.
	if modelPath == "" {
		dir := onnx.DefaultModelDir()
		modelPath = filepath.Join(dir, "model.onnx")
		vocabPath = filepath.Join(dir, "vocab.txt")
	}

	// If the model is missing and auto-download is enabled, fetch it.
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		if viper.GetBool("embedder.onnx.auto_download") {
			modelURL := viper.GetString("embedder.onnx.model_url")
			vocabURL := viper.GetString("embedder.onnx.vocab_url")
			if modelURL == "" {
				modelURL = onnx.DefaultModelURL()
			}
			if vocabURL == "" {
				vocabURL = onnx.DefaultVocabURL()
			}
			if err := onnx.DownloadModel(onnx.DefaultModelDir(), modelURL, vocabURL); err != nil {
				return nil, fmt.Errorf("auto-download model failed: %w", err)
			}
		} else {
			return nil, fmt.Errorf("onnx model not found at %q (set embedder.onnx.auto_download=true or provide model_path)", modelPath)
		}
	}

	return onnx.NewEmbedder(modelPath, vocabPath)
}
