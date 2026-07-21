//go:build cgo

package media

import (
	"log"
	"strings"

	"github.com/gen2brain/malgo"
	"github.com/pion/mediadevices"
)

func ListMicrophones() ([]DeviceInfo, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		log.Printf("[media] malgo init failed: %v", err)
		return nil, err
	}
	defer ctx.Uninit()

	devs, err := ctx.Devices(malgo.Capture)
	if err != nil {
		log.Printf("[media] failed to list capture devices: %v", err)
		return nil, err
	}

	var result []DeviceInfo
	for _, d := range devs {
		name := strings.TrimSpace(d.Name())
		if name == "" {
			continue
		}
		if strings.Contains(strings.ToLower(name), "monitor") {
			continue
		}
		result = append(result, DeviceInfo{
			ID:   d.ID.String(),
			Name: name,
		})
	}
	return result, nil
}

func ListSpeakers() ([]DeviceInfo, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		log.Printf("[media] malgo init failed: %v", err)
		return nil, err
	}
	defer ctx.Uninit()

	devs, err := ctx.Devices(malgo.Playback)
	if err != nil {
		log.Printf("[media] failed to list playback devices: %v", err)
		return nil, err
	}

	var result []DeviceInfo
	for _, d := range devs {
		name := strings.TrimSpace(d.Name())
		if name == "" {
			continue
		}
		result = append(result, DeviceInfo{
			ID:   d.ID.String(),
			Name: name,
		})
	}
	return result, nil
}

func cleanWebcamName(id string) string {
	if strings.Contains(id, "usb-") {
		parts := strings.SplitN(id, "-video-", 2)
		prefix := parts[0]
		if idx := strings.Index(prefix, "_"); idx > 0 {
			name := prefix[idx+1:]
			if lastIdx := strings.LastIndex(name, "_"); lastIdx > 0 {
				suffix := name[lastIdx+1:]
				if len(suffix) >= 8 && !strings.ContainsAny(suffix, " -") {
					name = name[:lastIdx]
				}
			}
			return strings.ReplaceAll(name, "_", " ")
		}
	}
	return "Camera"
}

func ListWebcams() ([]DeviceInfo, error) {
	devs := mediadevices.EnumerateDevices()
	var result []DeviceInfo
	for _, d := range devs {
		if d.Kind != mediadevices.VideoInput {
			continue
		}
		label := d.Label
		if label == "" {
			label = d.DeviceID
		}
		result = append(result, DeviceInfo{
			ID:   d.DeviceID,
			Name: label,
		})
	}
	return result, nil
}
