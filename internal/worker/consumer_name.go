package worker

import (
	"os"
	"strings"
	"uuid"
)

func GetWorkerName(configuredName, configuredHostname string) string {
	if name := strings.TrimSpace(configuredName); name != "" {
		return name
	}

	if hostname := strings.TrimSpace(configuredHostname); hostname != "" {
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
