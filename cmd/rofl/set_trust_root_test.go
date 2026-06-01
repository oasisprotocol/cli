package rofl

import "testing"

func TestSetTrustRootUse(t *testing.T) {
	if got, want := setTrustRootCmd.Name(), "set-trust-root"; got != want {
		t.Fatalf("unexpected command name: got %q, want %q", got, want)
	}
}

func TestSetTrustRootCommandRegistered(t *testing.T) {
	found := false
	for _, c := range Cmd.Commands() {
		if c == setTrustRootCmd || c.Name() == "set-trust-root" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("set-trust-root command not registered under rofl.Cmd")
	}
}

func TestSetTrustRootCommandFlags(t *testing.T) {
	// The command should expose selector flags (network/paratime) and deployment flags.
	names := []string{"network", "paratime", "no-paratime", "deployment"}
	for _, n := range names {
		if setTrustRootCmd.Flags().Lookup(n) == nil {
			t.Errorf("expected flag %q to be present", n)
		}
	}
}
