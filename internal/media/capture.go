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

type Capture struct {
	ctx          *malgo.AllocatedContext
	device       *malgo.Device
	buf          []float32
	mu           sync.Mutex
	cond         *sync.Cond
	closed       bool
	savedID      malgo.DeviceID
	cDeviceIDPtr unsafe.Pointer
}

func bytesToF32LE(data []byte) float32 {
	bits := binary.LittleEndian.Uint32(data)
	return math.Float32frombits(bits)
}

func NewCapture(deviceID string) (*Capture, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("malgo init: %w", err)
	}

	c := &Capture{
		ctx: ctx,
		buf: make([]float32, 0, FrameSize*8),
	}
	c.cond = sync.NewCond(&c.mu)

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatF32
	deviceConfig.Capture.Channels = 1
	deviceConfig.SampleRate = SampleRate

	if deviceID != "" {
		devs, _ := ctx.Devices(malgo.Capture)
		for _, d := range devs {
			if d.ID.String() == deviceID {
				c.savedID = d.ID
				idBytes := c.savedID[:]
				c.cDeviceIDPtr = unsafe.Pointer(C.CBytes(idBytes))
				deviceConfig.Capture.DeviceID = c.cDeviceIDPtr
				break
			}
		}
	}

	device, err := malgo.InitDevice(ctx.Context, deviceConfig, malgo.DeviceCallbacks{
		Data: func(output, input []byte, frameCount uint32) {
			if len(input) == 0 {
				return
			}
			samples := len(input) / 4
			data := make([]float32, samples)
			for i := 0; i < samples; i++ {
				data[i] = bytesToF32LE(input[i*4 : i*4+4])
			}
			c.mu.Lock()
			if !c.closed {
				c.buf = append(c.buf, data...)
			}
			c.cond.Broadcast()
			c.mu.Unlock()
		},
	})
	if err != nil {
		ctx.Uninit()
		return nil, fmt.Errorf("malgo capture device: %w", err)
	}
	c.device = device

	if err := device.Start(); err != nil {
		device.Uninit()
		ctx.Uninit()
		return nil, fmt.Errorf("capture start: %w", err)
	}

	return c, nil
}

func (c *Capture) ReadFrame(pcm []float32) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for len(c.buf) < FrameSize {
		if c.closed {
			return 0, fmt.Errorf("capture closed")
		}
		c.cond.Wait()
	}
	n := copy(pcm, c.buf[:FrameSize])
	c.buf = c.buf[FrameSize:]
	return n, nil
}

func (c *Capture) Close() {
	c.mu.Lock()
	c.closed = true
	c.cond.Broadcast()
	c.mu.Unlock()
	if c.device != nil {
		c.device.Uninit()
	}
	if c.cDeviceIDPtr != nil {
		C.free(c.cDeviceIDPtr)
		c.cDeviceIDPtr = nil
	}
	if c.ctx != nil {
		c.ctx.Uninit()
	}
}
