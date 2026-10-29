//go:build amd64 || arm64

package hikrobot

import (
	"testing"
	"unsafe"
)

// Expected values come from compiling the MVS 4.8 CameraParams.h for
// x86_64-pc-windows-msvc. Other 64-bit Go targets share the alignment, so the
// check runs on any CI machine.
func TestLayoutsMatchMVSHeaders(t *testing.T) {
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
		{"MV_FRAME_OUT_INFO_EX", unsafe.Sizeof(frameInfo{}), 256},
		{"MV_FRAME_OUT_INFO_EX.nHostTimeStamp", unsafe.Offsetof(frameInfo{}.HostTime), 24},
		{"MV_FRAME_OUT_INFO_EX.nFrameLen", unsafe.Offsetof(frameInfo{}.Length), 32},
		{"MV_FRAME_OUT_INFO_EX.nLostPacket", unsafe.Offsetof(frameInfo{}.LostPackets), 96},
		{"MV_FRAME_OUT_INFO_EX.nExtendWidth", unsafe.Offsetof(frameInfo{}.ExtendedWidth), 112},
		{"MV_FRAME_OUT_INFO_EX.nFrameLenEx", unsafe.Offsetof(frameInfo{}.ExtendedLength), 120},
		{"MV_FRAME_OUT", unsafe.Sizeof(frameOut{}), 328},
		{"MV_FRAME_OUT.stFrameInfo", unsafe.Offsetof(frameOut{}.Info), 8},
		{"MV_CC_PIXEL_CONVERT_PARAM_EX", unsafe.Sizeof(pixelConvert{}), 64},
		{"MV_CC_PIXEL_CONVERT_PARAM_EX.pSrcData", unsafe.Offsetof(pixelConvert{}.Source), 16},
		{"MV_CC_PIXEL_CONVERT_PARAM_EX.pDstBuffer", unsafe.Offsetof(pixelConvert{}.Destination), 32},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, c.got, c.want)
		}
	}
}
