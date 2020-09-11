// Copyright 2020 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"fmt"
	"github.com/ethereum/go-ethereum/cmd/devp2p/internal/ethtest"
	"github.com/ethereum/go-ethereum/internal/utesting"
	"gopkg.in/urfave/cli.v1"
	"os"
)

// TODO TO TEST:
// TODO: - GETBLOCKHEADERS, GETBLOCKBODIES, 2 CONN BLOCK PROPAGATION TEST

var (
	rlpxCommand = cli.Command{
		Name:  "rlpx",
		Usage: "RLPx Commands",
		Subcommands: []cli.Command{
			rlpxEthTestCommand,
		},
	}
	rlpxEthTestCommand = cli.Command{
		Name:   "eth-test",
		Usage:  "Runs tests against a node",
		ArgsUsage: "<node> <path_to_chain.rlp_file>", // TODO maybe better?
		Action: rlpxEthTest,
		Flags:  []cli.Flag{testPatternFlag},
	}
)

// TODO restore the ping cmd

func rlpxEthTest(ctx *cli.Context) error {
	if ctx.NArg() < 3 {
		exit("missing path to chain.rlp as command-line argument")
	}

	suite := ethtest.NewSuite(getNodeArg(ctx), ctx.Args()[1], ctx.Args()[2])

	// Filter and run test cases.
	tests := suite.AllTests()
	if ctx.IsSet(testPatternFlag.Name) {
		tests = utesting.MatchTests(tests, ctx.String(testPatternFlag.Name))
	}
	results := utesting.RunTests(tests, os.Stdout)
	if fails := utesting.CountFailures(results); fails > 0 {
		return fmt.Errorf("%v of %v tests passed.", len(tests)-fails, len(tests))
	}
	fmt.Printf("all tests passed\n")
	return nil
}
