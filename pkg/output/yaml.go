package output

import (
	"io"

	"github.com/kubectl-k8i/pkg/model"
	"gopkg.in/yaml.v3"
)

// YAMLFormatter renders node data as a YAML document.
type YAMLFormatter struct{}

// Format writes nodes as a YAML list using gopkg.in/yaml.v3.
// An empty node list produces an empty YAML list "[]".
// No ANSI codes, headers, timestamps, or annotations are included.
func (f *YAMLFormatter) Format(w io.Writer, nodes []model.NodeInfo) error {
	out := ToNodeOutputList(nodes)
	encoder := yaml.NewEncoder(w)
	if err := encoder.Encode(out); err != nil {
		_ = encoder.Close()
		return err
	}
	return encoder.Close()
}
