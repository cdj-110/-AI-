//go:build linux

package storage

import (
	"strings"
	"testing"
)

func TestParseMountInfo(t *testing.T) {
	input := "28 15 8:4 / /mnt/my\\040disk rw,nodev - ext4 /dev/sda4 rw\n" +
		"29 15 8:2 / /media/udisk1 rw - vfat /dev/sda2 ro\n" +
		"30 15 0:1 / /tmp rw - tmpfs tmpfs rw\nmalformed\n"
	d := parseMountInfo(strings.NewReader(input))
	if len(d) != 2 || d[0].MountID != "28" || d[0].MountPath != "/mnt/my disk" || d[0].ReadOnly || !d[1].ReadOnly {
		t.Fatalf("bad mounts: %+v", d)
	}
}
