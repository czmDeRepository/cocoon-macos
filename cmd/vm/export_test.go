package vm

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestExportRestartVNCPreservesCurrentDisplay(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Int("vnc", -1, "")
	cmd.Flags().String("vnc-password", "", "")

	vnc, password := exportRestartVNC(cmd, &record{VNCDisp: 3, VNCPass: "secret"})
	if vnc != 3 || password != "secret" {
		t.Fatalf("restart VNC = (%d, %q), want (3, secret)", vnc, password)
	}
}

func TestExportCommandAcceptsLocalOnlyInvocation(t *testing.T) {
	root := Command(NewHandler())
	exportCmd, _, err := root.Find([]string{"export"})
	if err != nil {
		t.Fatal(err)
	}
	if err := exportCmd.Args(exportCmd, []string{"vm-1"}); err != nil {
		t.Fatalf("one-argument local export rejected: %v", err)
	}
	if err := exportCmd.Args(exportCmd, []string{"vm-1", "registry/repo:tag"}); err != nil {
		t.Fatalf("two-argument OCI export rejected: %v", err)
	}
	if err := exportCmd.Args(exportCmd, nil); err == nil {
		t.Fatal("zero-argument export unexpectedly accepted")
	}
	if err := exportCmd.Args(exportCmd, []string{"vm-1", "registry/repo:tag", "extra"}); err == nil {
		t.Fatal("three-argument export unexpectedly accepted")
	}
}

func TestExportLocalErrorWithoutOCIDestination(t *testing.T) {
	err := exportLocalError("", "", "retaining local cloud image", "custom-macos-run-aa:test", errors.New("disk full"))
	if got := err.Error(); got != "retaining local cloud image custom-macos-run-aa:test failed: disk full" {
		t.Fatalf("error = %q", got)
	}
}

func TestExportLocalErrorAfterOCIPush(t *testing.T) {
	err := exportLocalError("registry/repo:tag", "sha256:abc", "retaining local cloud image", "custom-macos-run-aa:test", errors.New("disk full"))
	for _, want := range []string{"pushed registry/repo:tag as sha256:abc", "retaining local cloud image custom-macos-run-aa:test failed: disk full"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not contain %q", err, want)
		}
	}
}

func TestExportRestartVNCUsesExplicitOverrides(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Int("vnc", -1, "")
	cmd.Flags().String("vnc-password", "", "")
	if err := cmd.Flags().Set("vnc", "1"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("vnc-password", "newpass"); err != nil {
		t.Fatal(err)
	}

	vnc, password := exportRestartVNC(cmd, &record{VNCDisp: 3, VNCPass: "oldpass"})
	if vnc != 1 || password != "newpass" {
		t.Fatalf("restart VNC = (%d, %q), want (1, newpass)", vnc, password)
	}
}
