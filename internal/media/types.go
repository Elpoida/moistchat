package media

type DeviceInfo struct {
	ID   string
	Name string
}

type AudioFrame struct {
	From string
	Data []byte
}

const (
	SampleRate = 48000
	Channels   = 1
	FrameSize  = 960
	FrameMs    = 20
)
