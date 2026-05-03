package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- ParseFilter tests ---

func TestParseFilter_Valid(t *testing.T) {
	tests := []struct {
		input     string
		wantAttr  string
		wantValue string
	}{
		{"arch=amd64", "arch", "amd64"},
		{"zone=1a", "zone", "1a"},
		{"pool=pool-a", "pool", "pool-a"},
		{"ec2_type=spot", "ec2_type", "spot"},
		{"instance_type=m5.xlarge", "instance_type", "m5.xlarge"},
		{"nodeclaim=claim-1", "nodeclaim", "claim-1"},
		{"taint=dedicated", "taint", "dedicated"},
		{"taint=dedicated=gpu", "taint", "dedicated=gpu"},
		{"autoscaler=karpenter", "autoscaler", "karpenter"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			attr, val, err := ParseFilter(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantAttr, attr)
			assert.Equal(t, tt.wantValue, val)
		})
	}
}

func TestParseFilter_MissingEquals(t *testing.T) {
	_, _, err := ParseFilter("archvalue")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected attribute=value")
}

func TestParseFilter_EmptyAttribute(t *testing.T) {
	_, _, err := ParseFilter("=value")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "attribute cannot be empty")
}

func TestParseFilter_EmptyValue(t *testing.T) {
	_, _, err := ParseFilter("arch=")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "value cannot be empty")
}

func TestParseFilter_UnsupportedAttribute(t *testing.T) {
	_, _, err := ParseFilter("unknown=value")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported filter attribute")
}

// --- ParseSort tests ---

func TestParseSort_Valid(t *testing.T) {
	tests := []struct {
		input   string
		wantCol string
		wantDir string
	}{
		{"pool=asc", "pool", "asc"},
		{"cpu_load=desc", "cpu_load", "desc"},
		{"name=asc", "name", "asc"},
		{"mem_use=desc", "mem_use", "desc"},
		{"autoscaler=asc", "autoscaler", "asc"},
		{"age=desc", "age", "desc"},
		{"taint=asc", "taint", "asc"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			col, dir, err := ParseSort(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantCol, col)
			assert.Equal(t, tt.wantDir, dir)
		})
	}
}

func TestParseSort_MissingEquals(t *testing.T) {
	_, _, err := ParseSort("poolasc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected column=direction")
}

func TestParseSort_EmptyColumn(t *testing.T) {
	_, _, err := ParseSort("=asc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "column cannot be empty")
}

func TestParseSort_EmptyDirection(t *testing.T) {
	_, _, err := ParseSort("pool=")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "direction cannot be empty")
}

func TestParseSort_UnsupportedColumn(t *testing.T) {
	_, _, err := ParseSort("unknown=asc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported sort column")
}

func TestParseSort_InvalidDirection(t *testing.T) {
	_, _, err := ParseSort("pool=up")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid sort direction")
}

// --- ValidateOutputFormat tests ---

func TestValidateOutputFormat_Valid(t *testing.T) {
	for _, format := range []string{"table", "json", "yaml"} {
		t.Run(format, func(t *testing.T) {
			err := ValidateOutputFormat(format)
			assert.NoError(t, err)
		})
	}
}

func TestValidateOutputFormat_Unsupported(t *testing.T) {
	err := ValidateOutputFormat("xml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported output format")
}

func TestValidateOutputFormat_Empty(t *testing.T) {
	err := ValidateOutputFormat("")
	assert.Error(t, err)
}

// --- NewRootCommand tests ---

func TestNewRootCommand_DefaultFlags(t *testing.T) {
	cmd := NewRootCommand()

	// Default output format is "table".
	outputFlag := cmd.Flags().Lookup("output")
	assert.NotNil(t, outputFlag)
	assert.Equal(t, "table", outputFlag.DefValue)

	// Default sort is "pool=asc".
	sortFlag := cmd.Flags().Lookup("sort")
	assert.NotNil(t, sortFlag)
	assert.Equal(t, "pool=asc", sortFlag.DefValue)

	// Default no-headers is false.
	noHeadersFlag := cmd.Flags().Lookup("no-headers")
	assert.NotNil(t, noHeadersFlag)
	assert.Equal(t, "false", noHeadersFlag.DefValue)
}

func TestNewRootCommand_OutputShortAlias(t *testing.T) {
	cmd := NewRootCommand()

	// -o should be the short alias for --output.
	outputFlag := cmd.Flags().ShorthandLookup("o")
	assert.NotNil(t, outputFlag)
	assert.Equal(t, "output", outputFlag.Name)
}

func TestNewRootCommand_AllFlagsExist(t *testing.T) {
	cmd := NewRootCommand()

	expectedFlags := []string{
		"context", "labels", "filter", "sort", "fargate",
		"color", "debug", "group-by", "output", "no-headers",
	}

	for _, name := range expectedFlags {
		flag := cmd.Flags().Lookup(name)
		assert.NotNil(t, flag, "flag --%s should exist", name)
	}
}

func TestNewRootCommand_HelpOutput(t *testing.T) {
	cmd := NewRootCommand()
	// SetArgs to --help and capture output.
	cmd.SetArgs([]string{"--help"})
	var buf []byte
	cmd.SetOut(nil) // use default
	// Just verify the command doesn't error on --help.
	err := cmd.Execute()
	_ = buf
	assert.NoError(t, err)
}

func TestNewRootCommand_UnknownFlag(t *testing.T) {
	cmd := NewRootCommand()
	cmd.SetArgs([]string{"--unknown-flag"})
	err := cmd.Execute()
	assert.Error(t, err)
}

func TestParseFilter_TaintWithEquals(t *testing.T) {
	// taint=dedicated=gpu should parse as attribute=taint, value=dedicated=gpu.
	attr, val, err := ParseFilter("taint=dedicated=gpu")
	assert.NoError(t, err)
	assert.Equal(t, "taint", attr)
	assert.Equal(t, "dedicated=gpu", val)
}

func TestParseSort_AllSupportedColumns(t *testing.T) {
	columns := []string{
		"name", "pods", "cpu_req", "cpu_lim", "cpu_use", "cpu_cap", "cpu_load",
		"mem_req", "mem_lim", "mem_use", "mem_cap", "mem_load",
		"ec2_type", "instance_type", "arch", "zone", "pool", "age", "taint", "autoscaler",
	}

	for _, col := range columns {
		t.Run(col+"=asc", func(t *testing.T) {
			c, d, err := ParseSort(col + "=asc")
			assert.NoError(t, err)
			assert.Equal(t, col, c)
			assert.Equal(t, "asc", d)
		})
		t.Run(col+"=desc", func(t *testing.T) {
			c, d, err := ParseSort(col + "=desc")
			assert.NoError(t, err)
			assert.Equal(t, col, c)
			assert.Equal(t, "desc", d)
		})
	}
}

func TestParseFilter_AllSupportedAttributes(t *testing.T) {
	attributes := []string{
		"ec2_type", "instance_type", "arch", "zone", "pool", "nodeclaim", "taint", "autoscaler",
	}

	for _, attr := range attributes {
		t.Run(attr, func(t *testing.T) {
			a, v, err := ParseFilter(attr + "=test")
			assert.NoError(t, err)
			assert.Equal(t, attr, a)
			assert.Equal(t, "test", v)
		})
	}
}
