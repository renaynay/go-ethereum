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
	"math/big"
	"net"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/forkid"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/rlpx"
	"github.com/ethereum/go-ethereum/rlp"
	"gopkg.in/urfave/cli.v1"
)

var (
	rlpxCommand = cli.Command{
		Name:  "rlpx",
		Usage: "RLPx Commands",
		Subcommands: []cli.Command{
			rlpxPingCommand,
			rlpxStatusCommand,
		},
	}
	rlpxPingCommand = cli.Command{
		Name:      "ping",
		Usage:     "Perform a RLPx handshake",
		ArgsUsage: "<node>",
		Action:    rlpxPing,
	}
	rlpxStatusCommand = cli.Command{
		Name:      "status",
		Usage:     "Get the given node's status",
		ArgsUsage: "<node>",
		Action:    getStatus,
	}
)

// devp2pHandshake is the RLP structure of the devp2p protocol handshake.
type devp2pHandshake struct {
	Version    uint64
	Name       string
	Caps       []p2p.Cap
	ListenPort uint64
	ID         hexutil.Bytes // secp256k1 public key
	// Ignore additional fields (for forward compatibility).
	Rest []rlp.RawValue `rlp:"tail"`
}

func rlpxPing(ctx *cli.Context) error {
	conn, err := createConn(ctx)
	if err != nil {
		exit(fmt.Sprintf("could not connect to node: %v", err))
	}
	// do enc handshake
	ourKey, _ := crypto.GenerateKey()
	_, err = conn.Handshake(ourKey)
	if err != nil {
		exit(fmt.Sprintf("could not handshake with node: %v", err))
	}

	code, data, err := conn.Read()
	if err != nil {
		return err
	}
	switch code {
	case 0:
		var h devp2pHandshake
		if err := rlp.DecodeBytes(data, &h); err != nil {
			return fmt.Errorf("invalid handshake: %v", err)
		}
		fmt.Printf("%+v\n", h)
	case 1:
		var msg []p2p.DiscReason
		if rlp.DecodeBytes(data, &msg); len(msg) == 0 {
			return fmt.Errorf("invalid disconnect message")
		}
		return fmt.Errorf("received disconnect message: %v", msg[0])
	default:
		return fmt.Errorf("invalid message code %d, expected handshake (code zero)", code)
	}
	return nil
}

func createConn(ctx *cli.Context) (*rlpx.Conn, error) {
	n := getNodeArg(ctx)

	fd, err := net.Dial("tcp", fmt.Sprintf("%v:%d", n.IP(), n.TCP()))
	if err != nil {
		return nil, err
	}
	conn := rlpx.NewConn(fd, n.Pubkey())

	return conn, nil
}

// statusData is the network packet for the status message for eth/64 and later.
type statusData struct {
	ProtocolVersion uint32
	NetworkID       uint64
	TD              *big.Int
	Head            common.Hash
	Genesis         common.Hash
	ForkID          forkid.ID
}

// protoHandshake is the RLP structure of the protocol handshake.
type protoHandshake struct {
	Version    uint64
	Name       string
	Caps       []p2p.Cap
	ListenPort uint64
	ID         []byte // secp256k1 public key

	// Ignore additional fields (for forward compatibility).
	Rest []rlp.RawValue `rlp:"tail"`
}

func getStatus(ctx *cli.Context) error {
	conn, err := createConn(ctx)
	if err != nil {
		exit(fmt.Sprintf("could not connect to node: %v", err))
	}

	// do enc handshake
	ourKey, _ := crypto.GenerateKey()
	_, err = conn.Handshake(ourKey)
	if err != nil {
		exit(fmt.Sprintf("could not handshake with node: %v", err))
	}

	// create and write our protoHandshake
	pub0    := crypto.FromECDSAPub(&ourKey.PublicKey)[1:]
	ourHandshake := &protoHandshake{
		Version:    3,
		Caps:       []p2p.Cap{{"eth", 63}, {"eth", 64}},
		ID:         pub0,
	}

	size, payload, err := rlp.EncodeToReader(ourHandshake)
	if err != nil {
		exit(fmt.Sprintf("could not encode protoHandshake to reader: %v", err))
	}
	handshakeMsgCode := 0x00

	if _, err := conn.WriteMsg(uint64(handshakeMsgCode), uint32(size), payload); err != nil {
		exit(fmt.Sprintf("could not write protoHandshake to connection: %v", err))
	}

	wg := sync.WaitGroup{}

	// get protoHandshake
	wg.Add(1)
	go func() {
		code, rawData, err := conn.Read()
		if err != nil {
			exit(fmt.Sprintf("could not read from connection: %v", err))
		}
		var h devp2pHandshake
		if err := rlp.DecodeBytes(rawData, &h); err != nil {
			exit(fmt.Sprintf("could not decode payload: %v", err))
		}
		fmt.Println("code: ", code, "\nhandshakeData: ", h)
		wg.Done()
	}()
	wg.Wait()

	// get status
	wg.Add(1)
	go func() {
		code, rawData, err := conn.Read()
		if err != nil {
			exit(fmt.Sprintf("could not read from connection: %v", err))
		}

		switch code {
		case 1:
			var reason [1]p2p.DiscReason
			if err := rlp.DecodeBytes(rawData, &reason); err != nil {
				exit(fmt.Sprintf("could not decode payload: %v", err))
			}

			fmt.Println("code: ", code, "\nstatus: ", reason)
		case 16:
			var status statusData
			if err := rlp.DecodeBytes(rawData, &status); err != nil {
				exit(fmt.Sprintf("could not decode payload: %v", err))
			}

			fmt.Println("code: ", code, "\nstatus: ", status)
		}
		wg.Done()
	}()
	wg.Wait()

	return err
}
