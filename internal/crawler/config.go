package crawler

import (
	"fmt"
	"strings"
)

type AnubisMode string

const (
	AnubisModeAuto   AnubisMode = "auto"
	AnubisModeAlways AnubisMode = "always"
	AnubisModeOff    AnubisMode = "off"
)

type Config struct {
	TimeoutInSeconds int
	AnubisMode       AnubisMode
}

func ParseAnubisMode(value string) (AnubisMode, error) {
	mode := AnubisMode(strings.ToLower(strings.TrimSpace(value)))
	if mode == "" {
		return AnubisModeAuto, nil
	}

	switch mode {
	case AnubisModeAuto, AnubisModeAlways, AnubisModeOff:
		return mode, nil
	default:
		return "", fmt.Errorf("invalid ANUBIS_MODE %q: expected auto, always, or off", value)
	}
}
