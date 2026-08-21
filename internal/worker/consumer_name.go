package worker

import (
	"os"
	"strings"
	"uuid"
)

func GetWorkerName() string {
	if name := strings.TrimSpace(os.Getenv("CONSUMER_NAME")); name != "" {
		return name
	}

	if hostname := strings.TrimSpace(os.Getenv("HOSTNAME")); hostname != "" {
		return "worker-" + hostname
	}

	if hostname, err := os.Hostname(); err == nil {
		hostname = strings.TrimSpace(hostname)
		if hostname != "" {
			return "worker-" + hostname
		}
	}

	return "worker-" + uuid.New().String()
}
