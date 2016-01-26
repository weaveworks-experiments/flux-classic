package version

import "fmt"

var (
	revision = "unknown revision"
	version  = "head"
)

func Version() string {
	return fmt.Sprintf("%s (%s)", version, revision)
}
