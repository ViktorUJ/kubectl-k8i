//go:build tools

package tools

// This file ensures tool and test dependencies are tracked in go.mod.
// It is never compiled into the binary.
import (
	_ "github.com/spf13/cobra"
	_ "github.com/stretchr/testify/assert"
	_ "golang.org/x/sync/errgroup"
	_ "golang.org/x/term"
	_ "gopkg.in/yaml.v3"
	_ "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/kubernetes"
	_ "k8s.io/metrics/pkg/client/clientset/versioned"
	_ "pgregory.net/rapid"
)
