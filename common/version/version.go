package version

import (
	"fmt"
	"os"
	"path"
	"strings"
)

var (
	revision = "unknown revision"
	version  = "head"
)

func Banner() string {
	name := path.Base(os.Args[0])
	if !strings.HasPrefix(name, "flux") {
		name = "flux " + name
	}

	return fmt.Sprintf("%s version %s (%s)", name, version, revision)
}
