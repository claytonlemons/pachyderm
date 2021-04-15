package cmd

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/pachyderm/pachyderm/v2/src/internal/config"
	"github.com/pachyderm/pachyderm/v2/src/internal/grpcutil"
	"github.com/pachyderm/pachyderm/v2/src/internal/require"
	tu "github.com/pachyderm/pachyderm/v2/src/internal/testutil"
	uuid "github.com/satori/go.uuid"

	"github.com/spf13/cobra"
)

func TestPortForwardError(t *testing.T) {
	if os.Getenv("SKIP_TESTS_WITH_DEPS") == "TRUE" {
		t.Skip("Skipping because SKIP_TESTS_WITH_DEPS was true")
	}
	cfgFile := testConfig(t, "localhost:30650")
	defer os.Remove(cfgFile.Name())
	os.Setenv("PACH_CONFIG", cfgFile.Name())

	c := tu.Cmd("pachctl", "version", "--timeout=1ns")
	var errMsg bytes.Buffer
	c.Stdout = ioutil.Discard
	c.Stderr = &errMsg
	err := c.Run()
	require.YesError(t, err) // 1ns should prevent even local connections
	require.Matches(t, "context deadline exceeded", errMsg.String())
}

// Check that no commands have brackets in their names, which indicates that
// 'CreateAlias' was not used properly (or the command just needs to specify
// its name).
func TestCommandAliases(t *testing.T) {
	pachctlCmd := PachctlCmd()

	// Replace the first component with 'pachctl' because it uses os.Args[0] by default
	path := func(cmd *cobra.Command) string {
		return strings.Replace(cmd.CommandPath(), os.Args[0], "pachctl", 1)
	}

	paths := map[string]bool{}

	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		for _, subcmd := range cmd.Commands() {
			// This should only happen if there is a bug in MergeCommands, or some
			// code is bypassing it.
			require.False(
				t, paths[path(subcmd)],
				"Multiple commands found with the same invocation: '%s'",
				path(subcmd),
			)

			paths[path(subcmd)] = true

			require.True(
				t, subcmd.Short != "",
				"Command must provide a 'Short' description string: '%s'",
				path(subcmd),
			)
			require.True(
				t, subcmd.Long != "",
				"Command must provide a 'Long' description string: '%s' (%s)",
				path(subcmd), subcmd.Short,
			)
			require.False(
				t, strings.ContainsAny(subcmd.Name(), "[<({})>]"),
				"Command name contains invalid characters: '%s' (%s)",
				path(subcmd), subcmd.Short,
			)
			require.True(
				t, subcmd.Use != "",
				"Command must provide a 'Use' string: '%s' (%s)",
				path(subcmd), subcmd.Short,
			)

			walk(subcmd)
		}
	}

	walk(pachctlCmd)
}

func testConfig(t *testing.T, pachdAddressStr string) *os.File {
	t.Helper()

	cfgFile, err := ioutil.TempFile("", "")
	require.NoError(t, err)

	pachdAddress, err := grpcutil.ParsePachdAddress(pachdAddressStr)
	require.NoError(t, err)

	cfg := &config.Config{
		UserID: uuid.NewV4().String(),
		V2: &config.ConfigV2{
			ActiveContext: "test",
			Contexts: map[string]*config.Context{
				"test": &config.Context{
					PachdAddress: pachdAddress.Qualified(),
				},
			},
			Metrics: false,
		},
	}

	j, err := json.Marshal(&cfg)
	require.NoError(t, err)
	_, err = cfgFile.Write(j)
	require.NoError(t, err)
	require.NoError(t, cfgFile.Close())
	return cfgFile
}
