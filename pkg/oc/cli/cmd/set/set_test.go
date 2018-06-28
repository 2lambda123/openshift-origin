package set

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"

	kcmdtesting "k8s.io/kubernetes/pkg/kubectl/cmd/testing"
)

func TestLocalAndDryRunFlags(t *testing.T) {
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	errout := &bytes.Buffer{}
	tf := kcmdtesting.NewTestFactory().WithNamespace("test")
	defer tf.Cleanup()
	setCmd := NewCmdSet("", tf, in, out, errout)
	ensureLocalAndDryRunFlagsOnChildren(t, setCmd, "")
}

func ensureLocalAndDryRunFlagsOnChildren(t *testing.T, c *cobra.Command, prefix string) {
	for _, cmd := range c.Commands() {
		name := prefix + cmd.Name()
		if localFlag := cmd.Flag("local"); localFlag == nil {
			t.Errorf("Command %s does not implement the --local flag", name)
		}
		if dryRunFlag := cmd.Flag("dry-run"); dryRunFlag == nil {
			t.Errorf("Command %s does not implement the --dry-run flag", name)
		}
		ensureLocalAndDryRunFlagsOnChildren(t, cmd, name+".")
	}
}
