package archiveutil

import (
	"path/filepath"
	"strings"
)

func NormalizeArchiveName(name string) (string, bool) {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." {
		return "", false
	}

	name = strings.ReplaceAll(name, " ", "-")
	if !strings.EqualFold(filepath.Ext(name), ".wacz") {
		name += ".wacz"
	}

	return name, true
}
