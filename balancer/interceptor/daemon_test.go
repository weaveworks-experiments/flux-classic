package interceptor

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

type mockIPTables struct {
	t *testing.T

	// Map from "table-name chain-name" to lists of rules
	chains map[string][][]string
}

func newMockIPTables(t *testing.T) mockIPTables {
	return mockIPTables{
		t: t,
		chains: map[string][][]string{
			"nat PREROUTING": make([][]string, 0),
			"filter FORWARD": make([][]string, 0),
			"filter INPUT":   make([][]string, 0),
		},
	}
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

func (m mockIPTables) invoke(args []string) ([]byte, error) {
	require.Equal(m.t, "-t", args[0])

	switch args[2] {
	case "-N":
		k := m.key(args)
		if _, present := m.chains[k]; present {
			return m.error("iptables: Chain already exists.")
		}

		if len(args) > 4 {
			return m.error("Bad argument '", args[4], "'")
		}

		m.chains[k] = make([][]string, 0)

	case "-X":
		k := m.key(args)
		if _, present := m.chains[k]; !present {
			return m.error("iptables: No chain/target/match by that name.")
		}

		if len(args) > 4 {
			return m.error("Bad argument '", args[4], "'")
		}

		delete(m.chains, k)

	case "-F":
		k := m.key(args)
		if _, present := m.chains[k]; !present {
			return m.error("iptables: No chain/target/match by that name.")
		}

		if len(args) > 4 {
			return m.error("Bad argument '", args[4], "'")
		}

		m.chains[k] = m.chains[k][:0]

	case "-I":
		k := m.key(args)
		if _, present := m.chains[k]; !present {
			return m.error("iptables: No chain/target/match by that name.")
		}

		// no rulenum support needed for now

		m.chains[k] = append([][]string{args[4:]}, m.chains[k]...)

	case "-D":
		k := m.key(args)
		if _, present := m.chains[k]; !present {
			return m.error("iptables: No chain/target/match by that name.")
		}

		for i, r := range m.chains[k] {
			if reflect.DeepEqual(args[4:], r) {
				m.chains[k] = append(m.chains[k][:i], m.chains[k][i+1:]...)
				return nil, nil
			}
		}

		return m.error("iptables: Bad rule (does a matching rule exist in that chain?).")

	default:
		m.t.Log("Unknown iptables option ", args[2])
		m.t.Fail()
		return m.error("Unknown option ", args[2])
	}

	return nil, nil
}

func TestDaemon(t *testing.T) {
	i := Start([]string{"interceptor"}, newMockIPTables(t).invoke)

	select {
	case err := <-i.Fatal:
		t.Fatal(err)
	default:
	}

	i.Stop()
}
