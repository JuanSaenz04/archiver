package worker

import (
	"os"
	"strings"
	"testing"
	"uuid"
)

func TestGetWorkerName_ConsumerName(t *testing.T) {
	got := GetWorkerName("my-worker", "")
	if got != "my-worker" {
		t.Fatalf("GetWorkerName() = %q, want %q", got, "my-worker")
	}
}

func TestGetWorkerName_ConsumerNameTakesPrecedence(t *testing.T) {
	got := GetWorkerName("priority-worker", "some-host")
	if got != "priority-worker" {
		t.Fatalf("GetWorkerName() = %q, want %q", got, "priority-worker")
	}
}

func TestGetWorkerName_ConsumerNameWhitespace(t *testing.T) {
	got := GetWorkerName("  spaced  ", "")
	if got != "spaced" {
		t.Fatalf("GetWorkerName() = %q, want %q", got, "spaced")
	}
}

func TestGetWorkerName_ConsumerNameEmptyFallsThrough(t *testing.T) {
	got := GetWorkerName("   ", "host123")
	if got != "worker-host123" {
		t.Fatalf("GetWorkerName() = %q, want %q", got, "worker-host123")
	}
}

func TestGetWorkerName_Hostname(t *testing.T) {
	got := GetWorkerName("", "pod-abc")
	if got != "worker-pod-abc" {
		t.Fatalf("GetWorkerName() = %q, want %q", got, "worker-pod-abc")
	}
}

func TestGetWorkerName_HostnameWhitespace(t *testing.T) {
	got := GetWorkerName("", "  pod-abc  ")
	if got != "worker-pod-abc" {
		t.Fatalf("GetWorkerName() = %q, want %q", got, "worker-pod-abc")
	}
}

func TestGetWorkerName_FallbackOsHostname(t *testing.T) {
	got := GetWorkerName("", "")
	if !strings.HasPrefix(got, "worker-") {
		t.Fatalf("GetWorkerName() = %q, want prefix %q", got, "worker-")
	}

	if hostname, err := os.Hostname(); err == nil && strings.TrimSpace(hostname) != "" {
		want := "worker-" + strings.TrimSpace(hostname)
		if got != want {
			t.Fatalf("GetWorkerName() = %q, want %q", got, want)
		}
	} else {
		suffix := strings.TrimPrefix(got, "worker-")
		if _, err := uuid.Parse(suffix); err != nil {
			t.Fatalf("suffix %q is not a valid UUID: %v", suffix, err)
		}
	}
}
