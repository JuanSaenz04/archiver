package worker

import (
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestGetWorkerName_ConsumerNameEnv(t *testing.T) {
	os.Setenv("CONSUMER_NAME", "my-worker")
	defer os.Unsetenv("CONSUMER_NAME")

	got := GetWorkerName()
	if got != "my-worker" {
		t.Fatalf("GetWorkerName() = %q, want %q", got, "my-worker")
	}
}

func TestGetWorkerName_ConsumerNameTakesPrecedence(t *testing.T) {
	os.Setenv("CONSUMER_NAME", "priority-worker")
	os.Setenv("HOSTNAME", "some-host")
	defer os.Unsetenv("CONSUMER_NAME")
	defer os.Unsetenv("HOSTNAME")

	got := GetWorkerName()
	if got != "priority-worker" {
		t.Fatalf("GetWorkerName() = %q, want %q", got, "priority-worker")
	}
}

func TestGetWorkerName_ConsumerNameWhitespace(t *testing.T) {
	os.Setenv("CONSUMER_NAME", "  spaced  ")
	defer os.Unsetenv("CONSUMER_NAME")

	got := GetWorkerName()
	if got != "spaced" {
		t.Fatalf("GetWorkerName() = %q, want %q", got, "spaced")
	}
}

func TestGetWorkerName_ConsumerNameEmptyFallsThrough(t *testing.T) {
	os.Setenv("CONSUMER_NAME", "   ")
	os.Setenv("HOSTNAME", "host123")
	defer os.Unsetenv("CONSUMER_NAME")
	defer os.Unsetenv("HOSTNAME")

	got := GetWorkerName()
	if got != "worker-host123" {
		t.Fatalf("GetWorkerName() = %q, want %q", got, "worker-host123")
	}
}

func TestGetWorkerName_HostnameEnv(t *testing.T) {
	os.Setenv("HOSTNAME", "pod-abc")
	defer os.Unsetenv("HOSTNAME")

	got := GetWorkerName()
	if got != "worker-pod-abc" {
		t.Fatalf("GetWorkerName() = %q, want %q", got, "worker-pod-abc")
	}
}

func TestGetWorkerName_HostnameEnvWhitespace(t *testing.T) {
	os.Setenv("HOSTNAME", "  pod-abc  ")
	defer os.Unsetenv("HOSTNAME")

	got := GetWorkerName()
	if got != "worker-pod-abc" {
		t.Fatalf("GetWorkerName() = %q, want %q", got, "worker-pod-abc")
	}
}

func TestGetWorkerName_FallbackOsHostname(t *testing.T) {
	os.Unsetenv("CONSUMER_NAME")
	os.Unsetenv("HOSTNAME")

	got := GetWorkerName()
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
