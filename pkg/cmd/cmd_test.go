// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/go-cmd/pkg/cmd"
	"github.com/siderolabs/go-cmd/pkg/cmd/proc/reaper"
)

type CmdSuite struct {
	suite.Suite

	runReaper bool
}

func (suite *CmdSuite) SetupSuite() {
	if suite.runReaper {
		reaper.Run()
	}
}

func (suite *CmdSuite) TearDownSuite() {
	if suite.runReaper {
		reaper.Shutdown()
	}
}

func (suite *CmdSuite) TestRun() {
	type args struct {
		name string
		args []string
	}

	tests := []struct { //nolint:govet
		name      string
		args      args
		wantErr   bool
		errString string
	}{
		{
			"true",
			args{
				"true",
				[]string{},
			},
			false,
			"",
		},
		{
			"false",
			args{
				"false",
				[]string{},
			},
			true,
			"exit status 1: ",
		},
		{
			"false with output",
			args{
				"/bin/sh",
				[]string{
					"-c",
					"ls /not/found",
				},
			},
			true,
			"exit status 1: ls: /not/found: No such file or directory\n",
		},
		{
			"signal crash",
			args{
				"/bin/sh",
				[]string{
					"-c",
					"kill -2 $$",
				},
			},
			true,
			"signal: interrupt: ",
		},
		{
			"badexec",
			args{
				"badcommand",
				[]string{},
			},
			true,
			"exec: \"badcommand\": executable file not found in $PATH: ",
		},
	}

	for _, t := range tests {
		suite.Run(t.name, func() {
			// legacy API
			_, err := cmd.Run(t.args.name, t.args.args...)

			if t.wantErr {
				suite.Assert().Error(err)
				suite.Assert().Equal(t.errString, err.Error())
			} else {
				suite.Assert().NoError(err)
			}

			// modern API
			_, err = cmd.RunWithOptions(suite.T().Context(), t.args.name, t.args.args)
			if t.wantErr {
				suite.Assert().Error(err)
				suite.Assert().Equal(t.errString, err.Error())
			} else {
				suite.Assert().NoError(err)
			}
		})
	}

	// legacy API
	stdout, err := cmd.RunContext(cmd.WithStdin(context.Background(), strings.NewReader("hello")), "xargs", "echo")
	suite.Assert().NoError(err)
	suite.Assert().Equal("hello\n", stdout)

	// modern API
	stdout, err = cmd.RunWithOptions(suite.T().Context(), "xargs", []string{"echo"}, cmd.WithStandardInput(strings.NewReader("hello")))
	suite.Assert().NoError(err)
	suite.Assert().Equal("hello\n", stdout)
}

// TestLargeStdout verifies that stdout output exceeding the old 4096-byte limit
// (previously shared with MaxStderrLen) is not silently truncated.
func (suite *CmdSuite) TestLargeStdout() {
	// Generate output larger than the old 4096-byte stderr buffer.
	// printf '%6000s' pads an empty string to 6000 chars; tr converts spaces to 'x'.
	const wantLen = 6000

	// first, run with truncated output to verify that the test is valid
	stdout, err := cmd.RunWithOptions(suite.T().Context(), "/bin/sh", []string{"-c", fmt.Sprintf("printf '%%%ds' '' | tr ' ' 'x'", wantLen)})
	suite.Require().NoError(err)
	suite.Assert().Len(stdout, cmd.MaxStderrLen, "stdout should be truncated at the old 4096-byte limit")

	// now, run with full captured output
	stdout, err = cmd.RunWithOptions(suite.T().Context(), "/bin/sh", []string{"-c", fmt.Sprintf("printf '%%%ds' '' | tr ' ' 'x'", wantLen)}, cmd.WithFullStdoutCapture())
	suite.Require().NoError(err)
	suite.Assert().Len(stdout, wantLen, "stdout should not be truncated with full capture enabled")
}

func TestCmdSuite(t *testing.T) {
	for _, runReaper := range []bool{true, false} {
		func(runReaper bool) {
			t.Run(fmt.Sprintf("runReaper=%v", runReaper), func(t *testing.T) { suite.Run(t, &CmdSuite{runReaper: runReaper}) })
		}(runReaper)
	}
}
