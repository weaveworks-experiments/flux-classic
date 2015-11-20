package balancer

import (
	"fmt"
	"strings"
	"unicode"
)

type ipTablesError struct {
	cmd    string
	output string
}

func (err ipTablesError) Error() string {
	return fmt.Sprintf("'iptables %s' gave error: %s", err.cmd, err.output)
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

func (cf *config) doIPTables(args ...interface{}) error {
	flatArgs := flatten(args, nil)
	output, err := cf.iptables(flatArgs)
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

func (cf *config) chainRule() []interface{} {
	return []interface{}{"-i", cf.bridge, "-j", cf.chain}
}

func (cf *config) setupChain(table string, hookChains ...string) error {
	err := cf.deleteChain(table, hookChains...)
	if err != nil {
		return err
	}

	err = cf.doIPTables("-t", table, "-N", cf.chain)
	if err != nil {
		return err
	}

	for _, hookChain := range hookChains {
		err = cf.doIPTables("-t", table, "-I", hookChain, cf.chainRule())
		if err != nil {
			return err
		}
	}

	return nil
}

func (cf *config) deleteChain(table string, hookChains ...string) error {
	// First, remove any rules in the chain
	err := cf.doIPTables("-t", table, "-F", cf.chain)
	if err != nil {
		if _, ok := err.(ipTablesError); ok {
			// this probably means the chain doesn't exist
			return nil
		}
	}

	// Remove rules that reference our chain
	for _, hookChain := range hookChains {
		for {
			err := cf.doIPTables("-t", table, "-D", hookChain,
				cf.chainRule())
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
	return cf.doIPTables("-t", table, "-X", cf.chain)
}

func (cf *config) addRule(table string, args []interface{}) error {
	return cf.frobRule(table, "-A", args)
}

func (cf *config) deleteRule(table string, args []interface{}) error {
	return cf.frobRule(table, "-D", args)
}

func (cf *config) frobRule(table string, op string, args []interface{}) error {
	return cf.doIPTables("-t", table, op, cf.chain, args)
}
