package output

import (
	"encoding/json"
	"io"

	"github.com/kubectl-k8i/pkg/model"
)

// JSONFormatter renders node data as a JSON array.
type JSONFormatter struct{}

// Format writes nodes as an indented JSON array using encoding/json.
// An empty node list produces an empty JSON array "[]".
// No ANSI codes, headers, timestamps, or annotations are included.
func (f *JSONFormatter) Format(w io.Writer, nodes []model.NodeInfo) error {
	out := ToNodeOutputList(nodes)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(out)
}
