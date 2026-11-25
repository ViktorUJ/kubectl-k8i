package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// toggleValue is a pflag.Value for on/off switches written as
// --flag=enable / --flag=disable. A bare --flag means enable.
// true/false, on/off, yes/no and 1/0 are accepted as aliases.
type toggleValue bool

func newToggleValue(def bool, p *bool) *toggleValue {
	*p = def
	return (*toggleValue)(p)
}

func (t *toggleValue) Set(s string) error {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "enable", "enabled", "true", "on", "yes", "1":
		*t = true
	case "disable", "disabled", "false", "off", "no", "0":
		*t = false
	default:
		return fmt.Errorf("invalid value %q: expected enable or disable", s)
	}
	return nil
}

func (t *toggleValue) String() string {
	if *t {
		return "enable"
	}
	return "disable"
}

func (t *toggleValue) Type() string { return "enable|disable" }

// addToggleFlag registers an enable/disable flag with shell completion.
func addToggleFlag(cmd *cobra.Command, p *bool, name string, def bool, usage string) {
	f := cmd.Flags().VarPF(newToggleValue(def, p), name, "", usage)
	f.NoOptDefVal = "enable" // bare --name enables
	_ = cmd.RegisterFlagCompletionFunc(name, func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"enable", "disable"}, cobra.ShellCompDirectiveNoFileComp
	})
}

// toggleArgsHint rejects positional args and explains the "--flag=value" form
// when a user writes "--nodes disable" (a space-separated value is parsed as an
// argument because a bare --nodes already means enable).
func toggleArgsHint(flagNames ...string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return nil
		}
		var tv toggleValue
		if tv.Set(args[0]) == nil {
			examples := make([]string, 0, len(flagNames))
			for _, n := range flagNames {
				examples = append(examples, "--"+n+"="+args[0])
			}
			return fmt.Errorf("unexpected argument %q: pass the value with '=', e.g. %s", args[0], strings.Join(examples, " or "))
		}
		return fmt.Errorf("unknown argument %q for %q", args[0], cmd.CommandPath())
	}
}

var _ pflag.Value = (*toggleValue)(nil)
