package hikrobot

import "fmt"

// PixelType is a GigE Vision pixel format code (PixelType_Gvsp_* in the SDK's
// PixelType.h). Bits 16-23 hold the bits per pixel.
type PixelType uint32

// Common pixel types. Any other code the camera reports works too; it simply
// prints as hex.
const (
	Mono8       PixelType = 0x01080001
	Mono10      PixelType = 0x01100003
	Mono12      PixelType = 0x01100005
	Mono16      PixelType = 0x01100007
	BayerGR8    PixelType = 0x01080008
	BayerRG8    PixelType = 0x01080009
	BayerGB8    PixelType = 0x0108000A
	BayerBG8    PixelType = 0x0108000B
	BayerGR10   PixelType = 0x0110000C
	BayerRG10   PixelType = 0x0110000D
	BayerGB10   PixelType = 0x0110000E
	BayerBG10   PixelType = 0x0110000F
	BayerGR12   PixelType = 0x01100010
	BayerRG12   PixelType = 0x01100011
	BayerGB12   PixelType = 0x01100012
	BayerBG12   PixelType = 0x01100013
	RGB8Packed  PixelType = 0x02180014
	BGR8Packed  PixelType = 0x02180015
	RGBA8Packed PixelType = 0x02200016
	BGRA8Packed PixelType = 0x02200017
	YUV422      PixelType = 0x0210001F
	YUV422YUYV  PixelType = 0x02100032
)

var pixelNames = map[PixelType]string{
	Mono8: "Mono8", Mono10: "Mono10", Mono12: "Mono12", Mono16: "Mono16",
	BayerGR8: "BayerGR8", BayerRG8: "BayerRG8", BayerGB8: "BayerGB8", BayerBG8: "BayerBG8",
	BayerGR10: "BayerGR10", BayerRG10: "BayerRG10", BayerGB10: "BayerGB10", BayerBG10: "BayerBG10",
	BayerGR12: "BayerGR12", BayerRG12: "BayerRG12", BayerGB12: "BayerGB12", BayerBG12: "BayerBG12",
	RGB8Packed: "RGB8Packed", BGR8Packed: "BGR8Packed", RGBA8Packed: "RGBA8Packed", BGRA8Packed: "BGRA8Packed",
	YUV422: "YUV422_Packed", YUV422YUYV: "YUV422_YUYV_Packed",
}

// String returns the GenICam name, e.g. "BayerGB8", or the hex code.
func (p PixelType) String() string {
	if name, ok := pixelNames[p]; ok {
		return name
	}
	return fmt.Sprintf("0x%08X", uint32(p))
}

// BitsPerPixel is the storage size of one pixel.
func (p PixelType) BitsPerPixel() int { return int(p>>16) & 0xff }

// Is8BitRaw reports Mono8 and the four 8-bit Bayer layouts: one byte per pixel,
// which Camera.Preview can downsample before conversion.
func (p PixelType) Is8BitRaw() bool { return p == Mono8 || (p >= BayerGR8 && p <= BayerBG8) }
