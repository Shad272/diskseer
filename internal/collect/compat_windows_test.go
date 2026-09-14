//go:build windows

package collect

import (
	"encoding/binary"
	"syscall"
	"testing"
	"unsafe"
)

func TestInvalidDriveIDsNeverReachDeviceAPI(t *testing.T) {
	for _, id := range []string{"", "../file", "0\\path", "-1", "4294967296"} {
		if err := suDispositivo(id, func(syscall.Handle) error { t.Fatal("invalid device opened"); return nil }); err == nil {
			t.Fatal("invalid id accepted", id)
		}
	}
}

func TestSATLayoutMatchesArchitecture(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) == 4 {
		if sizeScsiPassThrough != 44 || satBufferField != 20 || satSenseField != 24 || satCDBField != 28 {
			t.Fatal("invalid x86 SAT layout")
		}
	} else if sizeScsiPassThrough != 56 || satBufferField != 24 || satSenseField != 32 || satCDBField != 36 {
		t.Fatal("invalid 64-bit SAT layout")
	}
}

func TestNVMeRejectsShortAndOverflowingDriverResponses(t *testing.T) {
	buf := make([]byte, 560)
	binary.LittleEndian.PutUint32(buf[24:], 40)
	binary.LittleEndian.PutUint32(buf[28:], 512)
	if _, err := parseNVMeResponse(buf, 560); err != nil {
		t.Fatal(err)
	}
	for _, size := range []uint32{0, 24, 48, 559, 1000} {
		if _, err := parseNVMeResponse(buf, size); err == nil {
			t.Fatalf("accepted short response %d", size)
		}
	}
	binary.LittleEndian.PutUint32(buf[24:], 0xfffffff0)
	if _, err := parseNVMeResponse(buf, 560); err == nil {
		t.Fatal("accepted overflowing offset")
	}
}
