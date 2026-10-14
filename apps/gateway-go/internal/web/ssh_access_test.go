package web

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFindSSHInitScriptAndServiceName(t *testing.T) {
	directory := t.TempDir()
	sshd := filepath.Join(directory, "S50sshd")
	dropbear := filepath.Join(directory, "S50dropbear")
	if err := os.WriteFile(sshd, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dropbear, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if got := findSSHInitScript([]string{dropbear, sshd}); got != dropbear {
		t.Fatalf("findSSHInitScript() = %q, want %q", got, dropbear)
	}
	if got := sshServiceName(dropbear); got != "Dropbear SSH" {
		t.Fatalf("sshServiceName() = %q", got)
	}
	if got := sshProcessName(sshd); got != "sshd" {
		t.Fatalf("sshProcessName() = %q", got)
	}
}

func TestApplySSHAccessInvokesRuntimeAction(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not preserve Unix executable mode bits")
	}
	script := filepath.Join(t.TempDir(), "S50dropbear")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0644); err != nil {
		t.Fatal(err)
	}
	var action string
	runner := func(_ context.Context, gotScript, gotAction string) (string, error) {
		if gotScript != script {
			t.Fatalf("script = %q, want %q", gotScript, script)
		}
		action = gotAction
		return "", nil
	}
	status := sshAccessStatus{Supported: true, InitScript: script, Running: false}
	if err := applySSHAccess(context.Background(), status, true, runner); err != nil {
		t.Fatal(err)
	}
	if action != "start" {
		t.Fatalf("action = %q, want start", action)
	}
	info, err := os.Stat(script)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0111 == 0 {
		t.Fatal("SSH init script should be executable after enabling")
	}
	status.Running = true
	if err := applySSHAccess(context.Background(), status, false, runner); err != nil {
		t.Fatal(err)
	}
	if action != "stop" {
		t.Fatalf("action = %q, want stop", action)
	}
}

func TestReadSSHAccessStatusUnsupportedOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific unsupported state")
	}
	status := readSSHAccessStatus(nil, func(string) bool { return false })
	if status.Supported || status.Port != 22 {
		t.Fatalf("unexpected status: %#v", status)
	}
}
