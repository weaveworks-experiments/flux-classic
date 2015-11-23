package balancer

import (
	"fmt"
	"strings"
	"unicode"
)

type IPTablesCmd func([]string) ([]byte, error)

type ipTablesError struct {
	cmd    string
	output string
}

func (err ipTablesError) Error() string {
	return fmt.Sprintf("'iptables %s' gave error: %s", err.cmd, err.output)
}

type ipTables struct {
	netConfig
	cmd              IPTablesCmd
	natChainSetup    bool
	filterChainSetup bool
}

func newIPTables(nc netConfig, cmd IPTablesCmd) *ipTables {
	return &ipTables{netConfig: nc, cmd: cmd}
}

func (ipt *ipTables) start() error {
	err := ipt.setupChain("nat", "PREROUTING")
	if err != nil {
		return err
	}
	ipt.natChainSetup = true

	err = ipt.setupChain("filter", "FORWARD", "INPUT")
	if err != nil {
		return err
	}
	ipt.filterChainSetup = true

	return nil
}

func (ipt *ipTables) close() {
	if ipt.natChainSetup {
		ipt.natChainSetup = false
		logError(ipt.deleteChain("nat", "PREROUTING"))
	}

	if ipt.filterChainSetup {
		ipt.filterChainSetup = false
		logError(ipt.deleteChain("filter", "FORWARD", "INPUT"))
	}
}

func flatten(args []interface{}, onto []string) []string {
	for _, arg := range args {
		switch argt := arg.(type) {
		case []interface{}:
			onto = flatten(argt, onto)
		default:
			onto = append(onto, fmt.Sprint(arg))
		}
	}
	return onto
}

// exec.ExitError is opaque
type exitError interface {
	error
	Success() bool
}

func (ipt *ipTables) doIPTables(args ...interface{}) error {
	flatArgs := flatten(args, nil)
	output, err := ipt.cmd(flatArgs)
	switch errt := err.(type) {
	case nil:
	case exitError:
		if !errt.Success() {
			return ipTablesError{
				cmd:    strings.Join(flatArgs, " "),
				output: sanitizeIPTablesOutput(output),
			}
		}
	default:
		return err
	}

	return nil
}

func sanitizeIPTablesOutput(output []byte) string {
	limit := 200
	return strings.Map(func(ch rune) rune {
		if limit == 0 {
			return -1
		}
		limit--

		if unicode.IsControl(ch) {
			ch = ' '
		}
		return ch
	}, string(output))
}

func (ipt *ipTables) chainRule() []interface{} {
	return []interface{}{"-i", ipt.bridge, "-j", ipt.chain}
}

func (ipt *ipTables) setupChain(table string, hookChains ...string) error {
	err := ipt.deleteChain(table, hookChains...)
	if err != nil {
		return err
	}

	err = ipt.doIPTables("-t", table, "-N", ipt.chain)
	if err != nil {
		return err
	}

	for _, hookChain := range hookChains {
		err = ipt.doIPTables("-t", table, "-I", hookChain, ipt.chainRule())
		if err != nil {
			return err
		}
	}

	return nil
}

func (ipt *ipTables) deleteChain(table string, hookChains ...string) error {
	// First, remove any rules in the chain
	err := ipt.doIPTables("-t", table, "-F", ipt.chain)
	if err != nil {
		if _, ok := err.(ipTablesError); ok {
			// this probably means the chain doesn't exist
			return nil
		}
	}

	// Remove rules that reference our chain
	for _, hookChain := range hookChains {
		for {
			err := ipt.doIPTables("-t", table, "-D", hookChain,
				ipt.chainRule())
			if err != nil {
				if _, ok := err.(ipTablesError); !ok {
					return err
				}

				// a "no such rule" error
				break
			}
		}
	}

	// Actually delete the chain
	return ipt.doIPTables("-t", table, "-X", ipt.chain)
}

func (ipt *ipTables) addRule(table string, args []interface{}) error {
	return ipt.frobRule(table, "-A", args)
}

func (ipt *ipTables) deleteRule(table string, args []interface{}) error {
	return ipt.frobRule(table, "-D", args)
}

func (ipt *ipTables) frobRule(table string, op string, args []interface{}) error {
	return ipt.doIPTables("-t", table, op, ipt.chain, args)
}
