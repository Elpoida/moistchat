//go:build cgo

package media

import (
	"context"
	"log"
)

type Pipeline struct {
	capture    *Capture
	playback   *Playback
	encoder    *OpusEncoder
	decoder    *OpusDecoder

	outgoing   chan AudioFrame
	incoming   chan AudioFrame
	cancel     context.CancelFunc
	ctx        context.Context
	muted      bool
	running    bool
}

func NewPipeline() *Pipeline {
	return &Pipeline{
		outgoing: make(chan AudioFrame, 64),
		incoming: make(chan AudioFrame, 64),
	}
}

func (p *Pipeline) Start(micID, spkID string) error {
	if p.running {
		return nil
	}

	var err error
	p.encoder, err = NewOpusEncoder()
	if err != nil {
		return err
	}
	p.decoder, err = NewOpusDecoder()
	if err != nil {
		return err
	}

	p.capture, err = NewCapture(micID)
	if err != nil {
		log.Printf("[audio] capture init: %v", err)
		return err
	}

	p.playback, err = NewPlayback(spkID)
	if err != nil {
		p.capture.Close()
		log.Printf("[audio] playback init: %v", err)
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	p.ctx = ctx
	p.cancel = cancel
	p.running = true
	p.muted = false

	// Capture goroutine: read PCM frames, encode to Opus, push to outgoing
	go func() {
		pcm := make([]float32, FrameSize)
		for {
			select {
			case <-p.ctx.Done():
				return
			default:
			}
			_, err := p.capture.ReadFrame(pcm)
			if err != nil {
				return
			}
			if p.muted {
				continue
			}
			data, err := p.encoder.Encode(pcm)
			if err != nil {
				log.Printf("[audio] encode error: %v", err)
				continue
			}
			select {
			case p.outgoing <- AudioFrame{Data: data}:
			default:
			}
		}
	}()

	// Playback goroutine: read incoming frames, decode to PCM, write to playback
	go func() {
		pcm := make([]float32, FrameSize)
		for {
			select {
			case <-p.ctx.Done():
				return
			case frame := <-p.incoming:
				_, err := p.decoder.Decode(frame.Data, pcm)
				if err != nil {
					log.Printf("[audio] decode error: %v", err)
					continue
				}
				if err := p.playback.WriteFrame(pcm); err != nil {
					log.Printf("[audio] playback write error: %v", err)
				}
			}
		}
	}()

	log.Printf("[audio] pipeline started")
	return nil
}

func (p *Pipeline) Stop() {
	if !p.running {
		return
	}
	p.running = false
	if p.cancel != nil {
		p.cancel()
	}
	if p.capture != nil {
		p.capture.Close()
	}
	if p.playback != nil {
		p.playback.Close()
	}
	p.encoder = nil
	p.decoder = nil
	log.Printf("[audio] pipeline stopped")
}

func (p *Pipeline) Outgoing() <-chan AudioFrame {
	return p.outgoing
}

func (p *Pipeline) EnqueueIncoming(frame AudioFrame) {
	select {
	case p.incoming <- frame:
	default:
	}
}

func (p *Pipeline) Mute() bool {
	p.muted = !p.muted
	return p.muted
}

func (p *Pipeline) IsMuted() bool {
	return p.muted
}

func (p *Pipeline) IsRunning() bool {
	return p.running
}
