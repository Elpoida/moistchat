//go:build cgo

package media

/*
#include <stdlib.h>
*/
import "C"

import (
	"encoding/binary"
	"fmt"
	"math"
	"sync"
	"unsafe"

	"github.com/gen2brain/malgo"
)

type Playback struct {
	ctx          *malgo.AllocatedContext
	device       *malgo.Device
	buf          []float32
	mu           sync.Mutex
	cond         *sync.Cond
	closed       bool
	savedID      malgo.DeviceID
	cDeviceIDPtr unsafe.Pointer
}

func f32ToBytes(v float32) []byte {
	bits := math.Float32bits(v)
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, bits)
	return b
}

func NewPlayback(deviceID string) (*Playback, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("malgo init: %w", err)
	}

	p := &Playback{
		ctx: ctx,
	}
	p.cond = sync.NewCond(&p.mu)

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Playback)
	deviceConfig.Playback.Format = malgo.FormatF32
	deviceConfig.Playback.Channels = 1
	deviceConfig.SampleRate = SampleRate

	if deviceID != "" {
		devs, _ := ctx.Devices(malgo.Playback)
		for _, d := range devs {
			if d.ID.String() == deviceID {
				p.savedID = d.ID
				idBytes := p.savedID[:]
				p.cDeviceIDPtr = unsafe.Pointer(C.CBytes(idBytes))
				deviceConfig.Playback.DeviceID = p.cDeviceIDPtr
				break
			}
		}
	}

	device, err := malgo.InitDevice(ctx.Context, deviceConfig, malgo.DeviceCallbacks{
		Data: func(output, input []byte, frameCount uint32) {
			p.mu.Lock()
			needed := int(frameCount)
			out := make([]byte, needed*4)
			for i := 0; i < needed && i < len(p.buf); i++ {
				copy(out[i*4:], f32ToBytes(p.buf[i]))
			}
			if needed < len(p.buf) {
				p.buf = p.buf[needed:]
			} else {
				p.buf = nil
			}
			if len(output) >= len(out) {
				copy(output, out)
			}
			p.cond.Broadcast()
			p.mu.Unlock()
		},
	})
	if err != nil {
		ctx.Uninit()
		return nil, fmt.Errorf("malgo playback device: %w", err)
	}
	p.device = device

	if err := device.Start(); err != nil {
		device.Uninit()
		ctx.Uninit()
		return nil, fmt.Errorf("playback start: %w", err)
	}

	return p, nil
}

func (p *Playback) WriteFrame(pcm []float32) error {
	if len(pcm) != FrameSize {
		return fmt.Errorf("playback: expected %d PCM samples, got %d", FrameSize, len(pcm))
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.buf) > FrameSize*16 {
		return nil
	}
	p.buf = append(p.buf, pcm...)
	return nil
}

func (p *Playback) Close() {
	p.mu.Lock()
	p.closed = true
	p.cond.Broadcast()
	p.mu.Unlock()
	if p.device != nil {
		p.device.Uninit()
	}
	if p.cDeviceIDPtr != nil {
		C.free(p.cDeviceIDPtr)
		p.cDeviceIDPtr = nil
	}
	if p.ctx != nil {
		p.ctx.Uninit()
	}
}
