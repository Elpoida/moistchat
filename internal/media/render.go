package media

import (
	"strings"
)

var LumaThreshold byte = 76

var brailleBits = [4][2]byte{
	{0x01, 0x08},
	{0x02, 0x10},
	{0x04, 0x20},
	{0x40, 0x80},
}

func RenderFrame(frame WebcamFrame, outW, outH int) string {
	if outW <= 0 || outH <= 0 || frame.Width <= 0 || frame.Height <= 0 {
		return ""
	}

	scaleW := float64(frame.Width) / float64(outW*2)
	scaleH := float64(frame.Height) / float64(outH*4)

	var sb strings.Builder
	sb.Grow(outW * outH * 4)

	w := frame.Width

	for row := 0; row < outH; row++ {
		baseY := float64(row * 4)
		for col := 0; col < outW; col++ {
			baseX := float64(col * 2)

			var dotBits byte
			for sy := 0; sy < 4; sy++ {
				srcY := int((baseY + float64(sy)) * scaleH)
				if srcY >= frame.Height {
					srcY = frame.Height - 1
				}
				yBase := srcY * w
				for sx := 0; sx < 2; sx++ {
					srcX := int((baseX + float64(sx)) * scaleW)
					if srcX >= frame.Width {
						srcX = frame.Width - 1
					}
					if frame.Y[yBase+srcX] > LumaThreshold {
						dotBits |= brailleBits[sy][sx]
					}
				}
			}
			ch := rune(0x2800) + rune(dotBits)
			sb.WriteRune(ch)
		}
		if row < outH-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}
