//go:build amd64 || arm64

package hikrobot

import (
	"runtime"
	"testing"
	"unsafe"
)

// Expected values come from compiling CameraParams.h (MVS 4.8 for Windows,
// MVS 5.1 for Linux) with clang for x86_64-pc-windows-msvc and
// aarch64/x86_64-linux-gnu. The Linux layout was confirmed by calling
// MV_CC_ConvertPixelTypeEx in the real aarch64 runtime with each candidate.
func TestLayoutsMatchMVSHeaders(t *testing.T) {
	// The pixel-type enum is 4 bytes under MSVC and 8 under GCC/Clang, which
	// moves everything after it in these structs.
	frameInfoSize, frameOutSize, convertSize := uintptr(264), uintptr(336), uintptr(72)
	hostTime, length, lost, extWidth, extLength, dst := uintptr(32), uintptr(40), uintptr(104), uintptr(120), uintptr(128), uintptr(40)
	if runtime.GOOS == "windows" {
		frameInfoSize, frameOutSize, convertSize = 256, 328, 64
		hostTime, length, lost, extWidth, extLength, dst = 24, 32, 96, 112, 120, 32
	}
	checks := []struct {
		name      string
		got, want uintptr
	}{
		{"MV_CC_DEVICE_INFO", unsafe.Sizeof(deviceInfo{}), 636},
		{"MV_CC_DEVICE_INFO.SpecialInfo", unsafe.Offsetof(deviceInfo{}.Special), 32},
		{"MV_CC_DEVICE_INFO_LIST", unsafe.Sizeof(deviceList{}), 2056},
		{"MV_CC_DEVICE_INFO_LIST.pDeviceInfo", unsafe.Offsetof(deviceList{}.Devices), 8},
		{"MVCC_INTVALUE_EX", unsafe.Sizeof(intValue{}), 96},
		{"MVCC_FLOATVALUE", unsafe.Sizeof(floatValue{}), 28},
		{"MVCC_ENUMVALUE", unsafe.Sizeof(enumValue{}), 280},
		{"MVCC_ENUMENTRY", unsafe.Sizeof(enumEntry{}), 84},
		{"MVCC_STRINGVALUE", unsafe.Sizeof(stringValue{}), 272},
		{"MVCC_STRINGVALUE.nMaxLength", unsafe.Offsetof(stringValue{}.MaxLength), 256},
		{"MV_FRAME_OUT_INFO_EX", unsafe.Sizeof(frameInfo{}), frameInfoSize},
		{"MV_FRAME_OUT_INFO_EX.nHostTimeStamp", unsafe.Offsetof(frameInfo{}.HostTime), hostTime},
		{"MV_FRAME_OUT_INFO_EX.nFrameLen", unsafe.Offsetof(frameInfo{}.Length), length},
		{"MV_FRAME_OUT_INFO_EX.nLostPacket", unsafe.Offsetof(frameInfo{}.LostPackets), lost},
		{"MV_FRAME_OUT_INFO_EX.nExtendWidth", unsafe.Offsetof(frameInfo{}.ExtendedWidth), extWidth},
		{"MV_FRAME_OUT_INFO_EX.nFrameLenEx", unsafe.Offsetof(frameInfo{}.ExtendedLength), extLength},
		{"MV_FRAME_OUT", unsafe.Sizeof(frameOut{}), frameOutSize},
		{"MV_FRAME_OUT.stFrameInfo", unsafe.Offsetof(frameOut{}.Info), 8},
		{"MV_CC_PIXEL_CONVERT_PARAM_EX", unsafe.Sizeof(pixelConvert{}), convertSize},
		{"MV_CC_PIXEL_CONVERT_PARAM_EX.pSrcData", unsafe.Offsetof(pixelConvert{}.Source), 16},
		{"MV_CC_PIXEL_CONVERT_PARAM_EX.pDstBuffer", unsafe.Offsetof(pixelConvert{}.Destination), dst},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, c.got, c.want)
		}
	}
}
