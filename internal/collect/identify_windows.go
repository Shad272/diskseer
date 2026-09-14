//go:build windows

package collect

import (
	"encoding/binary"
	"fmt"
	"github.com/shad272/diskseer/internal/model"
	"syscall"
)

// StorageDeviceProperty predates Windows 7. Seek-penalty support is queried
// separately: unsupported properties produce unknown values, never a guess.
func identifyDisk(d *model.Disk) {
	_ = suDispositivo(d.DeviceID, func(h syscall.Handle) error {
		query := make([]byte, 12)
		buf := make([]byte, 1024)
		var returned uint32
		if err := syscall.DeviceIoControl(h, ioctlStorageQueryProperty, &query[0], uint32(len(query)), &buf[0], uint32(len(buf)), &returned, nil); err != nil {
			return err
		}
		if returned < 32 {
			return fmt.Errorf("short device descriptor")
		}
		switch binary.LittleEndian.Uint32(buf[28:]) {
		case 1:
			d.BusType = "SCSI"
		case 3:
			d.BusType = "ATA"
		case 7:
			d.BusType = "USB"
		case 8:
			d.BusType = "RAID"
		case 11:
			d.BusType = "SATA"
		case 17:
			d.BusType = "NVMe"
			d.MediaType = "SSD"
		}
		if d.MediaType != "" && d.MediaType != "Unspecified" {
			return nil
		}
		if d.BusType != "ATA" && d.BusType != "SATA" {
			return nil
		}
		binary.LittleEndian.PutUint32(query, 7) // StorageDeviceSeekPenaltyProperty
		if err := syscall.DeviceIoControl(h, ioctlStorageQueryProperty, &query[0], uint32(len(query)), &buf[0], uint32(len(buf)), &returned, nil); err != nil {
			return err
		}
		if returned >= 9 && binary.LittleEndian.Uint32(buf[4:]) >= 9 {
			if buf[8] != 0 {
				d.MediaType = "HDD"
			} else {
				d.MediaType = "SSD"
			}
		}
		return nil
	})
}
