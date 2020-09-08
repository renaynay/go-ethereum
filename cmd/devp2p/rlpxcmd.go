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


func rlpxEthTest(ctx *cli.Context) error {
	suite := ethtest.NewSuite(getNodeArg(ctx), parseFileName(ctx))

	// Filter and run test cases.
	tests := suite.AllTests()
	if ctx.IsSet(testPatternFlag.Name) {
		tests = utesting.MatchTests(tests, ctx.String(testPatternFlag.Name))
	}
	results := utesting.RunTests(tests, os.Stdout)
	if fails := utesting.CountFailures(results); fails > 0 {
		return fmt.Errorf("%v/%v tests passed.", len(tests)-fails, len(tests))
	}
	fmt.Printf("%v/%v passed\n", len(tests), len(tests))
	return nil
}

func parseFileName(ctx *cli.Context) string {
	if ctx.NArg() < 2 {
		exit("missing path to chain.rlp as command-line argument")
	}
	return ctx.Args()[1]
}

// TODO make a test chain, long enough for tests to work, 2000 blocks max

//
//func rlpxGetBlockHeaders(ctx *cli.Context) error {
//	// TODO Duplicate code, put in separate func later
//	conn, err := createConn(ctx)
//	if err != nil {
//		exit(fmt.Sprintf("could not connect to node: %v", err))
//	}
//
//	// do enc handshake
//	ourKey, _ := crypto.GenerateKey()
//	_, err = conn.Handshake(ourKey)
//	if err != nil {
//		exit(fmt.Sprintf("could not handshake with node: %v", err))
//	}
//
//	// create and write our protoHandshake
//	pub0    := crypto.FromECDSAPub(&ourKey.PublicKey)[1:]
//	ourHandshake := &protoHandshake{
//		Version:    3,
//		Caps:       []p2p.Cap{{"eth", 65}},
//		ID:         pub0,
//	}
//
//	size, payload, err := rlp.EncodeToReader(ourHandshake)
//	if err != nil {
//		exit(fmt.Sprintf("could not encode protoHandshake to reader: %v", err))
//	}
//	handshakeMsgCode := 0x00
//
//	if _, err := conn.WriteMsg(uint64(handshakeMsgCode), uint32(size), payload); err != nil {
//		exit(fmt.Sprintf("could not write protoHandshake to connection: %v", err))
//	}
//
//	wg := sync.WaitGroup{}
//
//	// get protoHandshake
//	wg.Add(1)
//	go func() {
//		code, rawData, err := conn.Read()
//		if err != nil {
//			exit(fmt.Sprintf("could not read from connection: %v", err))
//		}
//		var h devp2pHandshake
//		if err := rlp.DecodeBytes(rawData, &h); err != nil {
//			exit(fmt.Sprintf("could not decode payload: %v", err))
//		}
//		fmt.Println("code: ", code, "\nhandshakeData: ", h)
//		wg.Done()
//	}()
//	wg.Wait()
//
//	// write status message
//	status := statusData{
//		ProtocolVersion: 65,
//		NetworkID:       1,
//		TD:              big.NewInt(1098883023422514),
//		Head:            common.BytesToHash([]byte{161, 12, 247, 201, 173, 67, 40, 69, 72, 119, 106, 82, 125, 205, 96, 232, 21, 217, 61, 77, 106, 8, 235, 236, 133, 222, 92, 88, 244, 110, 46, 220}),
//		Genesis:         common.BytesToHash([]byte{212, 229, 103, 64, 248, 118, 174, 248, 192, 16, 184, 106, 64, 213, 245, 103, 69, 161, 24, 208, 144, 106, 52, 230, 154, 236, 140, 13, 177, 203, 143, 163}),
//		ForkID:          forkid.ID{
//			Hash: [4]byte{252, 100, 236, 4},
//			Next: uint64(1150000),
//		},
//	}
//
//	size, payload, err = rlp.EncodeToReader(status)
//	if err != nil {
//		exit(fmt.Sprintf("cannot encode payload to reader: %v", err))
//	}
//
//	_, err = conn.WriteMsg(uint64(16), uint32(size), payload)
//	if err != nil {
//		exit(fmt.Sprintf("cannot write to connection: %v", err))
//	}
//
//	// get status
//	wg.Add(1)
//	go func() {
//		code, rawData, err := conn.Read()
//		if err != nil {
//			exit(fmt.Sprintf("could not read from connection: %v", err))
//		}
//
//		switch code {
//		case 1:
//			var reason [1]p2p.DiscReason
//			if err := rlp.DecodeBytes(rawData, &reason); err != nil {
//				exit(fmt.Sprintf("could not decode payload: %v", err))
//			}
//
//			fmt.Println("code: ", code, "\nstatus: ", reason)
//		case 16:
//			var status statusData
//			if err := rlp.DecodeBytes(rawData, &status); err != nil {
//				exit(fmt.Sprintf("could not decode payload: %v", err))
//			}
//
//			fmt.Println("code: ", code, "\nstatus: ", status)
//		}
//		wg.Done()
//	}()
//	wg.Wait()
//
//	// TODO Not sure?
//	wg.Add(1)
//	go func() {
//		code, rawData, err := conn.Read()
//		if err != nil {
//			exit(fmt.Sprintf("could not read from connection: %v", err))
//		}
//
//		switch code {
//		case 1:
//			var reason [1]p2p.DiscReason
//			if err := rlp.DecodeBytes(rawData, &reason); err != nil {
//				exit(fmt.Sprintf("could not decode payload: %v", err))
//			}
//
//			fmt.Println("code: ", code, "\nreason: ", reason)
//		//case 19:
//		//	var number hashOrNumber
//		//	if err := rlp.DecodeBytes(rawData, &number); err != nil {
//		//		exit(fmt.Sprintf("could not decode payload: %v", err))
//		//	}
//		//	fmt.Println("code: ", code, "\nhashes: ", number)
//		default:
//
//			fmt.Println("code: ", code, "\nrawData: ", rawData)
//		}
//
//		wg.Done()
//	}()
//	wg.Wait()
//
//	// get block hashes
//	//[block: {P, B_32}, maxHeaders: P, skip: P, reverse: P in {0, 1}]
//
//	req := getBlockHeadersData{
//		Origin:  hashOrNumber{
//			Hash: common.HexToHash("0x88e96d4537bea4d9c05d12549907b32561d3bf31f45aae734cdc119f13406cb6"),
//		},
//		Amount:  1,
//		Skip:    0,
//		Reverse: false,
//	}
//
//	size, payload, err = rlp.EncodeToReader(req)
//	if err != nil {
//		exit(fmt.Sprintf("could not rlp-encode block header data to reader: %v", err))
//	}
//
//	if _, err := conn.WriteMsg(uint64(3), uint32(size), payload); err != nil {
//		exit(fmt.Sprintf("could not write message to connection: %v", err))
//	}
//
//	// TODO Not sure?
//	wg.Add(1)
//	go func() {
//
//		for {
//			code, rawData, err := conn.Read()
//			if err != nil {
//				exit(fmt.Sprintf("could not read from connection: %v", err))
//			}
//
//			switch code {
//			case 1:
//				var reason [1]p2p.DiscReason
//				if err := rlp.DecodeBytes(rawData, &reason); err != nil {
//					exit(fmt.Sprintf("could not decode payload: %v", err))
//				}
//
//				fmt.Println("\n GOT DISCONNECT \n")
//				fmt.Println("code: ", code, "\nreason: ", reason)
//				wg.Done()
//				return
//			case 19:
//				var req [1]getBlockHeadersData
//				if err := rlp.DecodeBytes(rawData, &req); err != nil {
//					exit(fmt.Sprintf("could not decode payload: %v", err))
//				}
//				fmt.Println("code: ", code, "rawData: ", req)
//			default:
//
//				fmt.Println("code: ", code, "\nrawData: ", rawData)
//			}
//		}
//	}()
//	wg.Wait()
//
//
//	return nil
//}
