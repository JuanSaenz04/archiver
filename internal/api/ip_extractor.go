package api

import (
	"log/slog"
	"net"
	"os"
	"strings"

	"github.com/labstack/echo/v5"
)

func GetIPExtractorFromEnv() echo.IPExtractor {
	trustedProxiesEnv := os.Getenv("TRUSTED_PROXIES")
	if trustedProxiesEnv == "" {
		// No proxies defined: trust only the direct connection.
		return echo.ExtractIPDirect()
	}

	var trustOptions []echo.TrustOption
	trustOptions = append(trustOptions,
		echo.TrustLoopback(false),
		echo.TrustLinkLocal(false),
		echo.TrustPrivateNet(false),
	)

	for proxy := range strings.SplitSeq(trustedProxiesEnv, ",") {
		proxy = strings.TrimSpace(proxy)
		if proxy == "" {
			continue
		}

		// Try to parse as CIDR
		_, ipnet, err := net.ParseCIDR(proxy)
		if err == nil {
			trustOptions = append(trustOptions, echo.TrustIPRange(ipnet))
			continue
		}

		// Try to parse as single IP
		ip := net.ParseIP(proxy)
		if ip != nil {
			var mask net.IPMask
			if ip.To4() != nil {
				mask = net.CIDRMask(32, 32)
			} else {
				mask = net.CIDRMask(128, 128)
			}
			trustOptions = append(trustOptions, echo.TrustIPRange(&net.IPNet{IP: ip, Mask: mask}))
			continue
		}

		slog.Warn("invalid trusted proxy", "proxy", proxy)
	}

	slog.Info("configured secure IP extractor", "trusted_proxies", trustedProxiesEnv)
	return echo.ExtractIPFromXFFHeader(trustOptions...)
}
