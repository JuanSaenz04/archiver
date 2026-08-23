package archiveutil

import (
	"path/filepath"
	"strings"
)

var archiveNameReplacer = strings.NewReplacer(
	"/", "-",
	"\\", "-",
	" ", "-",
)

func NormalizeArchiveName(name string) (string, bool) {
	name = archiveNameReplacer.Replace(strings.TrimSpace(name))
	name = filepath.Base(name)
	if name == "" || name == "." {
		return "", false
	}

	if !strings.EqualFold(filepath.Ext(name), ".wacz") {
		name += ".wacz"
	}

	return name, true
}
