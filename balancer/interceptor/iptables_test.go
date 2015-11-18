package interceptor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizeIPTablesOutput(t *testing.T) {
	require.Equal(t, "x  y", sanitizeIPTablesOutput(([]byte)("x\n\ty")))

	as := make([]byte, 1000)
	for i := range as {
		as[i] = 'a'
	}
	require.Equal(t, string(as[:200]), sanitizeIPTablesOutput(as))
}
