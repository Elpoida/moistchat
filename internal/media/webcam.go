package media

import (
	"fmt"
	"image"
	"log"

	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/frame"
	"github.com/pion/mediadevices/pkg/prop"
)

type WebcamFrame struct {
	Y      []byte
	Width  int
	Height int
}

type WebcamCapture struct {
	closer func()
	frames chan WebcamFrame
	width  int
	height int
}

func StartWebcam(devicePath string) (*WebcamCapture, error) {
	stream, err := mediadevices.GetUserMedia(mediadevices.MediaStreamConstraints{
		Video: func(c *mediadevices.MediaTrackConstraints) {
			c.FrameFormat = prop.FrameFormat(frame.FormatYUYV)
			c.Width = prop.Int(640)
			c.Height = prop.Int(480)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get camera: %w", err)
	}

	tracks := stream.GetVideoTracks()
	if len(tracks) == 0 {
		return nil, fmt.Errorf("no video track found")
	}

	videoTrack, ok := tracks[0].(*mediadevices.VideoTrack)
	if !ok {
		tracks[0].Close()
		return nil, fmt.Errorf("unexpected track type")
	}

	reader := videoTrack.NewReader(true)

	var imgW, imgH int
	firstFrame := true

	wc := &WebcamCapture{
		frames: make(chan WebcamFrame, 8),
		closer: func() { tracks[0].Close() },
	}

	go func() {
		defer close(wc.frames)
		defer tracks[0].Close()

		for {
			img, release, err := reader.Read()
			if err != nil {
				log.Printf("[video] read frame: %v", err)
				return
			}

			if firstFrame {
				bounds := img.Bounds()
				imgW = bounds.Dx()
				imgH = bounds.Dy()
				wc.width = imgW
				wc.height = imgH
				log.Printf("[video] cam format=%dx%d", imgW, imgH)
				firstFrame = false
			}

			yData := extractY(img, imgW, imgH)
			release()

			select {
			case wc.frames <- WebcamFrame{Y: yData, Width: imgW, Height: imgH}:
			default:
			}
		}
	}()

	return wc, nil
}

func extractY(img image.Image, width, height int) []byte {
	switch i := img.(type) {
	case *image.YCbCr:
		size := i.Rect.Dx() * i.Rect.Dy()
		y := make([]byte, size)
		copy(y, i.Y[:size])
		return y
	default:
		y := make([]byte, width*height)
		for py := 0; py < height; py++ {
			for px := 0; px < width; px++ {
				r, g, b, _ := i.At(px, py).RGBA()
				y[py*width+px] = byte((r+g+b)/3 >> 8)
			}
		}
		return y
	}
}

func (wc *WebcamCapture) Frames() <-chan WebcamFrame {
	return wc.frames
}

func (wc *WebcamCapture) Close() {
	if wc.closer != nil {
		wc.closer()
	}
}
