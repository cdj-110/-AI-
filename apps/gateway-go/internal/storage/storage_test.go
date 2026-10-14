package storage

import "testing"

func TestResolveUUIDDoesNotFollowReassignedMountPath(t *testing.T) {
	devices := []Device{
		{Device: "/dev/sda2", UUID: "boot", MountPath: "/media/udisk1"},
		{Device: "/dev/sda4", UUID: "history", MountPath: "/mnt/udisk"},
	}
	got, ok := resolveDevice(devices, "/media/udisk1", "history")
	if !ok || got.Device != "/dev/sda4" {
		t.Fatalf("wrong volume: %+v, %v", got, ok)
	}
	if _, ok := resolveDevice(devices[:1], "/media/udisk1", "history"); ok {
		t.Fatal("fell back to wrong partition")
	}
	if got, ok := resolveDevice(devices, "/media/udisk1", ""); !ok || got.UUID != "boot" {
		t.Fatal("legacy path resolution changed")
	}
}

func TestMountGuardRejectsSameDeviceAfterRemount(t *testing.T) {
	original := Device{Device: "/dev/sda4", MountPath: "/mnt/udisk", MountID: "28", UUID: "data"}
	current := original
	guard := mountGuard(original, func() ([]Device, error) { return []Device{current}, nil })
	if !guard() {
		t.Fatal("original mount unavailable")
	}
	current.MountID = "31"
	if guard() {
		t.Fatal("reused stale database after remount")
	}
	current = original
	if guard() {
		t.Fatal("lost mount was not latched")
	}
}

func TestMountGuardRejectsReadOnly(t *testing.T) {
	original := Device{Device: "/dev/sda4", MountPath: "/mnt/udisk", MountID: "28"}
	current := original
	current.ReadOnly = true
	if mountGuard(original, func() ([]Device, error) { return []Device{current}, nil })() {
		t.Fatal("accepted read-only mount")
	}
}
