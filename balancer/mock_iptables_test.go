package balancer

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type mockIPTables struct {
	t *testing.T

	// Map from "table-name chain-name" to lists of rules
	chains map[string][][]string
}

var builtinChains = []string{
	"nat PREROUTING",
	"nat OUTPUT",
	"filter FORWARD",
	"filter INPUT",
}

func newMockIPTables(t *testing.T) mockIPTables {
	m := mockIPTables{
		t:      t,
		chains: make(map[string][][]string),
	}

	for _, c := range builtinChains {
		m.chains[c] = make([][]string, 0)
	}

	return m
}

type mockExitError bool

func (e mockExitError) Success() bool { return bool(e) }
func (e mockExitError) Error() string { return "mockExitError" }

func (m mockIPTables) key(args []string) string {
	return fmt.Sprintf("%s %s", args[1], args[3])
}

func (m mockIPTables) error(msg ...interface{}) ([]byte, error) {
	return ([]byte)(fmt.Sprint(msg)), mockExitError(false)
}

func (m mockIPTables) cmd(args []string) ([]byte, error) {
	require.True(m.t, len(args) >= 4)
	require.Equal(m.t, "-t", args[0])

	if len(args[2]) != 2 || args[2][0] != '-' ||
		!strings.ContainsRune("NXFIAD", rune(args[2][1])) {
		m.t.Log("Unknown iptables option ", args[2])
		m.t.Fail()
		return m.error("Unknown option ", args[2])
	}

	k := m.key(args)
	if args[2] == "-N" {
		if _, present := m.chains[k]; present {
			return m.error("iptables: Chain already exists.")
		}

		if len(args) > 4 {
			return m.error("Bad argument '", args[4], "'")
		}

		m.chains[k] = make([][]string, 0)
		return nil, nil
	}

	if _, present := m.chains[k]; !present {
		return m.error("iptables: No chain/target/match by that name.")
	}

	switch args[2] {
	case "-X":
		if len(args) > 4 {
			return m.error("Bad argument '", args[4], "'")
		}

		delete(m.chains, k)

	case "-F":
		if len(args) > 4 {
			return m.error("Bad argument '", args[4], "'")
		}

		m.chains[k] = m.chains[k][:0]

	case "-I":
		// no rulenum support needed for now

		m.chains[k] = append([][]string{args[4:]}, m.chains[k]...)

	case "-A":
		// no rulenum support needed for now

		m.chains[k] = append(m.chains[k], args[4:])

	case "-D":
		for i, r := range m.chains[k] {
			if reflect.DeepEqual(args[4:], r) {
				m.chains[k] = append(m.chains[k][:i], m.chains[k][i+1:]...)
				return nil, nil
			}
		}

		return m.error("iptables: Bad rule (does a matching rule exist in that chain?).")
	}

	return nil, nil
}
