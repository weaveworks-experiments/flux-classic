package balancer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizeIPTablesOutput(t *testing.T) {
	require.Equal(t, "x  y", sanitizeIPTablesOutput(([]byte)("x\n\ty")))

	as := strings.Repeat("a", 1000)
	require.Equal(t, as[:200], sanitizeIPTablesOutput(([]byte)(as)))
}
