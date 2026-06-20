package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetIPExtractorFromEnv_Empty(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "")

	extractor := GetIPExtractorFromEnv()
	assert.NotNil(t, extractor)

	// RemoteAddr has port; ExtractIPDirect should strip it.
	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "203.0.113.50:1234"
	req.Header.Set("X-Forwarded-For", "192.168.1.1, 10.0.0.1")

	ip := extractor(req)
	assert.Equal(t, "203.0.113.50", ip, "Should trust only RemoteAddr when TRUSTED_PROXIES is empty")
}

func TestGetIPExtractorFromEnv_SingleIP(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "192.168.1.1, 2001:db8::1")

	extractor := GetIPExtractorFromEnv()
	assert.NotNil(t, extractor)

	// Scenario 1: Request comes from an untrusted client directly (even if header contains trusted proxy)
	req1, _ := http.NewRequest("GET", "/", nil)
	req1.RemoteAddr = "203.0.113.50:1234"
	req1.Header.Set("X-Forwarded-For", "192.168.1.1")
	assert.Equal(t, "203.0.113.50", extractor(req1), "Should not trust XFF header if connection isn't from trusted proxy")

	// Scenario 2: Request comes from trusted IPv4 proxy
	req2, _ := http.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "192.168.1.1:9999"
	req2.Header.Set("X-Forwarded-For", "203.0.113.10, 192.168.1.1")
	assert.Equal(t, "203.0.113.10", extractor(req2), "Should traverse trusted proxies in XFF to find client IP")

	// Scenario 3: Request comes from trusted IPv6 proxy
	req3, _ := http.NewRequest("GET", "/", nil)
	req3.RemoteAddr = "[2001:db8::1]:1234"
	req3.Header.Set("X-Forwarded-For", "203.0.113.12")
	assert.Equal(t, "203.0.113.12", extractor(req3), "Should trust IPv6 proxy and extract client IP")
}

func TestGetIPExtractorFromEnv_CIDR(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "10.0.0.0/16")

	extractor := GetIPExtractorFromEnv()
	assert.NotNil(t, extractor)

	// Connection from within CIDR range
	req1, _ := http.NewRequest("GET", "/", nil)
	req1.RemoteAddr = "10.0.5.2:1234"
	req1.Header.Set("X-Forwarded-For", "198.51.100.5, 10.0.5.2")
	assert.Equal(t, "198.51.100.5", extractor(req1), "Should trust IP from CIDR range and extract client IP")

	// Connection from outside CIDR range
	req2, _ := http.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "10.1.5.2:1234" // Different subnet
	req2.Header.Set("X-Forwarded-For", "198.51.100.5")
	assert.Equal(t, "10.1.5.2", extractor(req2), "Should ignore XFF when connection is from outside CIDR range")
}

func TestGetIPExtractorFromEnv_MalformedAndWhitespace(t *testing.T) {
	// Mixed with extra spaces, empty elements, and invalid IP configurations
	t.Setenv("TRUSTED_PROXIES", " 192.168.1.10, , invalid_net, 10.0.0.0/8 ")

	extractor := GetIPExtractorFromEnv()
	assert.NotNil(t, extractor)

	// The valid elements (192.168.1.10 and 10.0.0.0/8) should still work.
	req1, _ := http.NewRequest("GET", "/", nil)
	req1.RemoteAddr = "192.168.1.10:4321"
	req1.Header.Set("X-Forwarded-For", "203.0.113.5")
	assert.Equal(t, "203.0.113.5", extractor(req1), "Should support parsed single IP proxy even with malformed entries present")

	req2, _ := http.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "10.5.5.5:80"
	req2.Header.Set("X-Forwarded-For", "203.0.113.6")
	assert.Equal(t, "203.0.113.6", extractor(req2), "Should support parsed CIDR range proxy even with malformed entries present")
}
