package onnx

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	stdlibhttp "github.com/andrewhowdencom/stdlib/http"
)

// DownloadModel fetches a remote model and vocab file into dir.
// It skips existing files. Progress is silent; callers can wrap with logging.
func DownloadModel(dir, modelURL, vocabURL string) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create model dir: %w", err)
	}

	// Use the stdlib HTTP client with longer timeouts for large model downloads.
	client, err := stdlibhttp.NewClient(
		stdlibhttp.WithTimeout(5*time.Minute),
		stdlibhttp.WithConnectTimeout(10*time.Second),
		stdlibhttp.WithResponseHeaderTimeout(30*time.Second),
	)
	if err != nil {
		return fmt.Errorf("create http client: %w", err)
	}

	modelPath := filepath.Join(dir, "model.onnx")
	vocabPath := filepath.Join(dir, "vocab.txt")

	if err := downloadFile(client, modelPath, modelURL); err != nil {
		return fmt.Errorf("download model: %w", err)
	}
	if err := downloadFile(client, vocabPath, vocabURL); err != nil {
		return fmt.Errorf("download vocab: %w", err)
	}

	return nil
}

func downloadFile(client *http.Client, path, url string) error {
	if _, err := os.Stat(path); err == nil {
		return nil // already exists
	}

	tmpPath := path + ".tmp"
	//nolint:gosec // path is derived from a configurable model directory, not user input.
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmpPath) }()

	//nolint:gosec // url is a known model download endpoint, validated by caller.
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d for %s", resp.StatusCode, url)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

// DefaultModelURL is a known-good ONNX model URL for all-MiniLM-L6-v2.
// This points to a community ONNX export on HuggingFace.
const DefaultModelURL = "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/onnx/model.onnx"

// DefaultVocabURL is the vocab.txt URL for all-MiniLM-L6-v2.
const DefaultVocabURL = "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/vocab.txt"
