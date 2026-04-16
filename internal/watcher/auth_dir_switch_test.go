package watcher

import "testing"

func TestSwitchAuthDirUpdatesValueWithoutWatcher(t *testing.T) {
	w := &Watcher{authDir: "D:/auth/pool-a"}

	w.switchAuthDir("D:/auth/pool-b")

	if got := w.authDir; got != "D:/auth/pool-b" {
		t.Fatalf("authDir = %q, want %q", got, "D:/auth/pool-b")
	}
}

func TestSwitchAuthDirSkipsEmptyValue(t *testing.T) {
	w := &Watcher{authDir: "D:/auth/pool-a"}

	w.switchAuthDir("   ")

	if got := w.authDir; got != "D:/auth/pool-a" {
		t.Fatalf("authDir = %q, want unchanged", got)
	}
}
