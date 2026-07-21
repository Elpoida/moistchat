//go:build !cgo

package media

import "fmt"

type Pipeline struct{}

func NewPipeline() *Pipeline {
	return &Pipeline{}
}

func (p *Pipeline) Start(micID, spkID string) error {
	return fmt.Errorf("audio requires CGO (native compilation)")
}

func (p *Pipeline) Stop() {}

func (p *Pipeline) Outgoing() <-chan AudioFrame {
	ch := make(chan AudioFrame)
	close(ch)
	return ch
}

func (p *Pipeline) EnqueueIncoming(frame AudioFrame) {}

func (p *Pipeline) Mute() bool { return false }

func (p *Pipeline) IsMuted() bool { return false }

func (p *Pipeline) IsRunning() bool { return false }

func ListMicrophones() ([]DeviceInfo, error) {
	return []DeviceInfo{}, nil
}

func ListSpeakers() ([]DeviceInfo, error) {
	return []DeviceInfo{}, nil
}

func ListWebcams() ([]DeviceInfo, error) {
	return []DeviceInfo{}, nil
}
