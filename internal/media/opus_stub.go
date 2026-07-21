//go:build !cgo

package media

import "fmt"

type OpusEncoder struct{}

func NewOpusEncoder() (*OpusEncoder, error) {
	return nil, fmt.Errorf("opus encoder requires CGO (native compilation)")
}

func (e *OpusEncoder) Encode(pcm []float32) ([]byte, error) {
	return nil, fmt.Errorf("opus requires CGO")
}

type OpusDecoder struct{}

func NewOpusDecoder() (*OpusDecoder, error) {
	return nil, fmt.Errorf("opus decoder requires CGO (native compilation)")
}

func (d *OpusDecoder) Decode(opusData []byte, pcm []float32) (int, error) {
	return 0, fmt.Errorf("opus requires CGO")
}
