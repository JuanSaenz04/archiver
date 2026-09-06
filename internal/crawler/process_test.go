package crawler

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRunProcess_Cancellation(t *testing.T) {
	for _, tc := range []struct {
		name, script string
		graceful     bool
	}{
		{"sigterm", `trap 'echo terminated > "$2"; exit 0' TERM; echo ready > "$1"; while :; do sleep 0.01; done`, true},
		{"sigkill fallback", `trap '' TERM; echo ready > "$1"; while :; do sleep 0.01; done`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			ready, terminated := filepath.Join(dir, "ready"), filepath.Join(dir, "terminated")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cmd := exec.CommandContext(ctx, "sh", "-c", tc.script, "sh", ready, terminated)
			done := make(chan error, 1)
			go func() { done <- runProcess(cmd, 100*time.Millisecond) }()
			require.Eventually(t, func() bool { _, err := os.Stat(ready); return err == nil }, 2*time.Second, 10*time.Millisecond)
			cancel()
			select {
			case err := <-done:
				require.Error(t, err)
			case <-time.After(2 * time.Second):
				t.Fatal("process did not stop")
			}
			if tc.graceful {
				require.FileExists(t, terminated)
			}
		})
	}
}

func TestRunProcess_KillsDescendantAfterWrapperExits(t *testing.T) {
	dir := t.TempDir()
	ready, survived := filepath.Join(dir, "ready"), filepath.Join(dir, "survived")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// The child ignores TERM while the wrapper exits. Without group-wide KILL,
	// the child would survive long enough to write its marker.
	cmd := exec.CommandContext(ctx, "sh", "-c", `trap 'exit 0' TERM; sh -c 'trap "" TERM; echo ready > "$1"; sleep 0.5; echo survived > "$2"' sh "$1" "$2" & wait`, "sh", ready, survived)
	done := make(chan error, 1)
	go func() { done <- runProcess(cmd, 100*time.Millisecond) }()
	require.Eventually(t, func() bool { _, err := os.Stat(ready); return err == nil }, 2*time.Second, 10*time.Millisecond)
	cancel()
	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("process group did not stop")
	}
	time.Sleep(600 * time.Millisecond)
	require.NoFileExists(t, survived)
}
