package output

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/kubectl-k8i/pkg/model"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

// sampleNode returns a NodeInfo with representative values.
func sampleNode(name string) model.NodeInfo {
	return model.NodeInfo{
		Name:             name,
		PodsUsed:         5,
		PodsMax:          110,
		CPURequestCores:  1.5,
		CPULimitCores:    3.0,
		CPUUsageCores:    2.0,
		CPUCapacityCores: 4.0,
		CPULoadPercent:   50,
		MemRequestGB:     4.0,
		MemLimitGB:       8.0,
		MemUsageGB:       6.0,
		MemCapacityGB:    16.0,
		MemLoadPercent:   38,
		EC2InstanceID:    "i-0abcdef1234567890",
		InstanceType:     "m5.xlarge",
		CapacityType:     "spot",
		Architecture:     "amd64",
		Zone:             "1a",
		Nodepool:         "pool-a",
		Nodeclaim:        "claim-1",
		Autoscaler:       "karpenter",
		Age:              "5d12h",
		TaintStr:         "none",
	}
}

func TestJSONFormatter_ValidJSON(t *testing.T) {
	nodes := []model.NodeInfo{sampleNode("node-1"), sampleNode("node-2")}
	var buf bytes.Buffer
	f := &JSONFormatter{}
	err := f.Format(&buf, nodes)
	assert.NoError(t, err)

	// Verify output is valid JSON.
	var parsed []NodeOutput
	err = json.Unmarshal(buf.Bytes(), &parsed)
	assert.NoError(t, err)
	assert.Len(t, parsed, 2)
}

func TestYAMLFormatter_ValidYAML(t *testing.T) {
	nodes := []model.NodeInfo{sampleNode("node-1"), sampleNode("node-2")}
	var buf bytes.Buffer
	f := &YAMLFormatter{}
	err := f.Format(&buf, nodes)
	assert.NoError(t, err)

	// Verify output is valid YAML.
	var parsed []NodeOutput
	err = yaml.Unmarshal(buf.Bytes(), &parsed)
	assert.NoError(t, err)
	assert.Len(t, parsed, 2)
}

func TestJSONFormatter_EmptyList(t *testing.T) {
	var buf bytes.Buffer
	f := &JSONFormatter{}
	err := f.Format(&buf, []model.NodeInfo{})
	assert.NoError(t, err)

	var parsed []NodeOutput
	err = json.Unmarshal(buf.Bytes(), &parsed)
	assert.NoError(t, err)
	assert.Empty(t, parsed)
}

func TestYAMLFormatter_EmptyList(t *testing.T) {
	var buf bytes.Buffer
	f := &YAMLFormatter{}
	err := f.Format(&buf, []model.NodeInfo{})
	assert.NoError(t, err)

	var parsed []NodeOutput
	err = yaml.Unmarshal(buf.Bytes(), &parsed)
	assert.NoError(t, err)
	assert.Empty(t, parsed)
}

func TestNodeOutput_StructTags(t *testing.T) {
	// Verify that NodeOutput struct tags match expected field names.
	expectedJSONFields := []string{
		"name", "pods_used", "pods_max",
		"cpu_request_cores", "cpu_limit_cores", "cpu_usage_cores", "cpu_capacity_cores", "cpu_load_percent",
		"mem_request_gb", "mem_limit_gb", "mem_usage_gb", "mem_capacity_gb", "mem_load_percent",
		"ec2_instance_id", "instance_type", "capacity_type", "architecture", "zone",
		"nodepool", "nodeclaim", "autoscaler", "age", "taints",
	}

	typ := reflect.TypeOf(NodeOutput{})
	var actualJSONFields []string
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("json")
		if tag != "" {
			actualJSONFields = append(actualJSONFields, tag)
		}
	}

	assert.Equal(t, expectedJSONFields, actualJSONFields)
}

func TestJSONFormatter_NoANSICodes(t *testing.T) {
	// Even with high load percentages, JSON output should have no ANSI codes.
	node := sampleNode("node-1")
	node.CPULoadPercent = 95
	node.MemLoadPercent = 85

	var buf bytes.Buffer
	f := &JSONFormatter{}
	err := f.Format(&buf, []model.NodeInfo{node})
	assert.NoError(t, err)

	assert.NotContains(t, buf.String(), "\033[")
}

func TestYAMLFormatter_NoANSICodes(t *testing.T) {
	node := sampleNode("node-1")
	node.CPULoadPercent = 95
	node.MemLoadPercent = 85

	var buf bytes.Buffer
	f := &YAMLFormatter{}
	err := f.Format(&buf, []model.NodeInfo{node})
	assert.NoError(t, err)

	assert.NotContains(t, buf.String(), "\033[")
}

func TestNewFormatter_JSON(t *testing.T) {
	f, err := NewFormatter(FormatJSON)
	assert.NoError(t, err)
	assert.IsType(t, &JSONFormatter{}, f)
}

func TestNewFormatter_YAML(t *testing.T) {
	f, err := NewFormatter(FormatYAML)
	assert.NoError(t, err)
	assert.IsType(t, &YAMLFormatter{}, f)
}

func TestNewFormatter_Unsupported(t *testing.T) {
	_, err := NewFormatter("xml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported output format")
}

func TestToNodeOutput_FieldMapping(t *testing.T) {
	node := sampleNode("test-node")
	out := ToNodeOutput(node)

	assert.Equal(t, node.Name, out.Name)
	assert.Equal(t, node.PodsUsed, out.PodsUsed)
	assert.Equal(t, node.PodsMax, out.PodsMax)
	assert.Equal(t, node.CPURequestCores, out.CPURequestCores)
	assert.Equal(t, node.CPULimitCores, out.CPULimitCores)
	assert.Equal(t, node.CPUUsageCores, out.CPUUsageCores)
	assert.Equal(t, node.CPUCapacityCores, out.CPUCapacityCores)
	assert.Equal(t, node.CPULoadPercent, out.CPULoadPercent)
	assert.Equal(t, node.MemRequestGB, out.MemRequestGB)
	assert.Equal(t, node.MemLimitGB, out.MemLimitGB)
	assert.Equal(t, node.MemUsageGB, out.MemUsageGB)
	assert.Equal(t, node.MemCapacityGB, out.MemCapacityGB)
	assert.Equal(t, node.MemLoadPercent, out.MemLoadPercent)
	assert.Equal(t, node.EC2InstanceID, out.EC2InstanceID)
	assert.Equal(t, node.InstanceType, out.InstanceType)
	assert.Equal(t, node.CapacityType, out.CapacityType)
	assert.Equal(t, node.Architecture, out.Architecture)
	assert.Equal(t, node.Zone, out.Zone)
	assert.Equal(t, node.Nodepool, out.Nodepool)
	assert.Equal(t, node.Nodeclaim, out.Nodeclaim)
	assert.Equal(t, node.Autoscaler, out.Autoscaler)
	assert.Equal(t, node.Age, out.Age)
	assert.Equal(t, node.TaintStr, out.Taints)
}

func TestJSONFormatter_FieldNamesInOutput(t *testing.T) {
	nodes := []model.NodeInfo{sampleNode("node-1")}
	var buf bytes.Buffer
	f := &JSONFormatter{}
	err := f.Format(&buf, nodes)
	assert.NoError(t, err)

	output := buf.String()
	// Verify key field names appear in JSON output.
	for _, field := range []string{"name", "pods_used", "cpu_request_cores", "mem_load_percent", "autoscaler", "taints"} {
		assert.True(t, strings.Contains(output, field), "JSON output should contain field %q", field)
	}
}
