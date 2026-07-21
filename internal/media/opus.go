//go:build cgo && (linux || windows || darwin)

package media

import (
	"fmt"

	opusLib "github.com/hraban/opus"
)

type OpusEncoder struct {
	enc *opusLib.Encoder
}

type OpusDecoder struct {
	dec *opusLib.Decoder
}

func NewOpusEncoder() (*OpusEncoder, error) {
	enc, err := opusLib.NewEncoder(SampleRate, Channels, opusLib.AppVoIP)
	if err != nil {
		return nil, fmt.Errorf("opus encoder: %w", err)
	}
	return &OpusEncoder{enc: enc}, nil
}

func NewOpusDecoder() (*OpusDecoder, error) {
	dec, err := opusLib.NewDecoder(SampleRate, Channels)
	if err != nil {
		return nil, fmt.Errorf("opus decoder: %w", err)
	}
	return &OpusDecoder{dec: dec}, nil
}

func (e *OpusEncoder) Encode(pcm []float32) ([]byte, error) {
	if len(pcm) != FrameSize {
		return nil, fmt.Errorf("opus: expected %d PCM samples, got %d", FrameSize, len(pcm))
	}
	buf := make([]byte, 1500)
	n, err := e.enc.EncodeFloat32(pcm, buf)
	if err != nil {
		return nil, fmt.Errorf("opus encode: %w", err)
	}
	return buf[:n], nil
}

func (d *OpusDecoder) Decode(opusData []byte, pcm []float32) (int, error) {
	n, err := d.dec.DecodeFloat32(opusData, pcm)
	if err != nil {
		return 0, fmt.Errorf("opus decode: %w", err)
	}
	return n, nil
}
