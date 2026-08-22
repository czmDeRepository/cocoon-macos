package vm

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/spf13/cobra"
)

func TestStartAlreadyRunningIsIdempotent(t *testing.T) {
	stateDir := t.TempDir()
	vmDir := filepath.Join(stateDir, "vms", "macos-demo")
	if err := os.MkdirAll(vmDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Give isRunning a real process whose argv[0] and arguments match the
	// qemu identity check without requiring qemu or KVM in the unit test.
	fakeQEMU := filepath.Join(t.TempDir(), qemuBinary)
	if err := os.Symlink("/bin/sh", fakeQEMU); err != nil {
		t.Fatal(err)
	}
	disk := filepath.Join(vmDir, "disk.qcow2")
	process := exec.Command(fakeQEMU, "-c", "sleep 60", disk)
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = process.Process.Kill()
		_ = process.Wait()
	})

	rec := &record{Name: "macos-demo", Disk: disk, PID: process.Process.Pid, VNCDisp: 1}
	if err := saveRec(vmDir, rec); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	cmd.Flags().String("state-dir", stateDir, "")
	cmd.Flags().Int("vnc", -1, "")
	cmd.Flags().String("vnc-password", "", "")
	if err := cmd.Flags().Set("vnc", "2"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("vnc-password", "newpass"); err != nil {
		t.Fatal(err)
	}

	if err := NewHandler().Start(cmd, []string{"macos-demo"}); err != nil {
		t.Fatalf("duplicate start must adopt the live qemu: %v", err)
	}
	got, err := loadRec(vmDir)
	if err != nil {
		t.Fatal(err)
	}
	if got.PID != rec.PID || got.VNCDisp != 1 {
		t.Fatalf("live record changed: pid=%d vnc=%d, want pid=%d vnc=1", got.PID, got.VNCDisp, rec.PID)
 	}
}

func TestExitOnRebootEnabled(t *testing.T) {
	if exitOnRebootEnabled(&record{}) {
		t.Fatal("standalone VM must keep normal reboot behavior by default")
	}
	t.Setenv("COCOON_MACOS_EXIT_ON_REBOOT", "true")
	if !exitOnRebootEnabled(&record{}) {
		t.Fatal("external supervisor environment must enable cold relaunch behavior")
	}
	t.Setenv("COCOON_MACOS_EXIT_ON_REBOOT", "invalid")
	if exitOnRebootEnabled(&record{}) {
		t.Fatal("invalid environment value must not change reboot behavior")
	}
	if !exitOnRebootEnabled(&record{ExitOnReboot: true}) {
		t.Fatal("persisted VM setting must enable cold relaunch behavior")
	}
}

func TestCloneOpenCoreBase(t *testing.T) {
	tests := []struct {
		name string
		src  *record
		want string
	}{
		{"recorded base is inherited, not the source overlay", &record{OpenCore: "/vms/src/OpenCore.qcow2", OpenCoreBase: "/fw/OpenCore.qcow2"}, "/fw/OpenCore.qcow2"},
		{"no identity: OpenCore is itself the base", &record{OpenCore: "/fw/OpenCore.qcow2"}, "/fw/OpenCore.qcow2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cloneOpenCoreBase(&cobra.Command{}, tt.src)
			if err != nil {
				t.Fatalf("cloneOpenCoreBase: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestImagesToSnapshot(t *testing.T) {
	tests := []struct {
		name string
		rec  *record
		want []string
	}{
		// raw .fd NVRAM can't hold internal snapshots, so only the disk is captured
		{"raw nvram captures disk only", &record{Disk: "/v/disk.qcow2", OVMFVars: "/v/OVMF_VARS.fd"}, []string{"/v/disk.qcow2"}},
		// a qcow2 NVRAM rolls back too
		{"qcow2 nvram captures both", &record{Disk: "/v/disk.qcow2", OVMFVars: "/v/OVMF_VARS.qcow2"}, []string{"/v/disk.qcow2", "/v/OVMF_VARS.qcow2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := imagesToSnapshot(tt.rec); !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// auto-create/CNI/bridge need Linux + CAP_NET_ADMIN; they are smoke-tested on the testbed.
func TestPrepareNetNoProvision(t *testing.T) {
	tests := []struct {
		name                        string
		rec                         *record
		wantTap, wantNetns, wantMAC string
	}{
		{"user-mode", &record{NetMode: "user", MAC: "aa:bb:cc:dd:ee:ff"}, "", "", "aa:bb:cc:dd:ee:ff"},
		{"pre-created tap", &record{NetMode: "tap", Tap: "tap0", MAC: "aa:bb:cc:dd:ee:ff"}, "tap0", "", "aa:bb:cc:dd:ee:ff"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.SetContext(t.Context())
			tap, netns, mac, err := prepareNet(cmd, tt.rec)
			if err != nil {
				t.Fatalf("prepareNet: %v", err)
			}
			if tap != tt.wantTap || netns != tt.wantNetns || mac != tt.wantMAC {
				t.Errorf("got tap=%q netns=%q mac=%q; want %q %q %q", tap, netns, mac, tt.wantTap, tt.wantNetns, tt.wantMAC)
			}
		})
	}
}
