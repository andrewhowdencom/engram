//go:build cgo

package onnx

import (
	"context"
	"fmt"
	"math"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

var envOnce sync.Once

// sessionHolder keeps the ONNX session and its bound tensors together.
type sessionHolder struct {
	session       *ort.AdvancedSession
	inputIDs      *ort.Tensor[int64]
	attentionMask *ort.Tensor[int64]
	output        *ort.Tensor[float32]
}

func (e *Embedder) loadModel() error {
	var initErr error
	envOnce.Do(func() {
		initErr = ort.InitializeEnvironment()
	})
	if initErr != nil {
		return fmt.Errorf("init onnx environment: %w", initErr)
	}

	// We need pre-created tensors to build the session.
	// Shapes: batch=1, seq_len=maxLen, hidden_size=hiddenSize.
	inputShape := ort.NewShape(1, int64(e.maxLen))
	outputShape := ort.NewShape(1, int64(e.maxLen), int64(e.hiddenSize))

	inputIDs, err := ort.NewEmptyTensor[int64](inputShape)
	if err != nil {
		return fmt.Errorf("create input_ids tensor: %w", err)
	}
	attentionMask, err := ort.NewEmptyTensor[int64](inputShape)
	if err != nil {
		return fmt.Errorf("create attention_mask tensor: %w", err)
	}
	outputTensor, err := ort.NewEmptyTensor[float32](outputShape)
	if err != nil {
		return fmt.Errorf("create output tensor: %w", err)
	}

	session, err := ort.NewAdvancedSession(e.modelPath,
		[]string{"input_ids", "attention_mask"},
		[]string{"last_hidden_state"},
		[]ort.Value{inputIDs, attentionMask},
		[]ort.Value{outputTensor},
		nil)
	if err != nil {
		return fmt.Errorf("create onnx session: %w", err)
	}

	e.session = &sessionHolder{
		session:       session,
		inputIDs:      inputIDs,
		attentionMask: attentionMask,
		output:        outputTensor,
	}
	e.modelLoaded = true
	return nil
}

func (e *Embedder) embed(_ context.Context, text string) ([]float32, error) {
	inputIDs, attentionMask, length := e.tokenizer.encode(text)

	// Get the session holder.
	holder, ok := e.session.(*sessionHolder)
	if !ok {
		return nil, fmt.Errorf("session not initialised")
	}

	// Populate input tensors.
	idData := holder.inputIDs.GetData()
	maskData := holder.attentionMask.GetData()
	for i := 0; i < e.maxLen; i++ {
		idData[i] = inputIDs[i]
		maskData[i] = attentionMask[i]
	}

	// Run inference.
	if err := holder.session.Run(); err != nil {
		return nil, fmt.Errorf("onnx run: %w", err)
	}

	// Extract output and mean-pool.
	outData := holder.output.GetData()

	// Mean pooling: average all token embeddings where attention_mask == 1.
	embedding := make([]float32, e.hiddenSize)
	var maskSum float64
	for i := 0; i < length; i++ {
		if attentionMask[i] == 0 {
			continue
		}
		maskSum++
		for j := 0; j < e.hiddenSize; j++ {
			embedding[j] += outData[i*e.hiddenSize+j]
		}
	}
	if maskSum > 0 {
		inv := float32(1.0 / maskSum)
		for j := 0; j < e.hiddenSize; j++ {
			embedding[j] *= inv
		}
	}

	// L2 normalise.
	var norm float64
	for _, v := range embedding {
		norm += float64(v) * float64(v)
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		invNorm := float32(1.0 / norm)
		for j := 0; j < e.hiddenSize; j++ {
			embedding[j] *= invNorm
		}
	}

	return embedding, nil
}
