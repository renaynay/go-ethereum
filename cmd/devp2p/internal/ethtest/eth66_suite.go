// Copyright 2021 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package ethtest

import (
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/internal/utesting"
	"github.com/ethereum/go-ethereum/p2p"
)

func (s *Suite) Eth66Tests() []utesting.Test {
	return []utesting.Test{
		{Name: "Status_66", Fn: s.TestStatus_66},
		//{Name: "GetBlockHeaders_66", Fn: s.TestGetBlockHeaders_66},
		//{Name: "Broadcast_66", Fn: s.TestBroadcast_66},
		//{Name: "GetBlockBodies_66", Fn: s.TestGetBlockBodies_66},
		//{Name: "TestLargeAnnounce_66", Fn: s.TestLargeAnnounce_66},
		//{Name: "TestMaliciousHandshake_66", Fn: s.TestMaliciousHandshake_66},
		//{Name: "TestMaliciousStatus_66", Fn: s.TestMaliciousStatus_66},
		//{Name: "TestTransactions_65", Fn: s.TestTransaction_65},
		//{Name: "TestMaliciousTransactions_65", Fn: s.TestMaliciousTx_65},
	}
}

// TestStatus_66 attempts to connect to the given node and perform a status
// exchange on the eth66 protocol, and then check to make sure the chain head
// is correct.
func (s *Suite) TestStatus_66(t *utesting.T) {
	conn, err := s.dial()
	if err != nil {
		t.Fatalf("could not dial: %v", err)
	}
	// get protoHandshake
	conn.handshake(t, s.caps)
	// get status
	switch msg := conn.statusExchange(t, s.chain, s.eth66StatusMessage()).(type) {
	case *Status:
		t.Logf("got status message: %s", pretty.Sdump(msg))
	default:
		t.Fatalf("unexpected: %s", pretty.Sdump(msg))
	}
}

func (s *Suite) eth66StatusMessage() *Status {
	return &Status{
		ProtocolVersion: 66,
		NetworkID:       s.chain.chainConfig.ChainID.Uint64(),
		TD:              s.chain.TD(s.chain.Len()),
		Head:            s.chain.blocks[s.chain.Len()-1].Hash(),
		Genesis:         s.chain.blocks[0].Hash(),
		ForkID:          s.chain.ForkID(),
	}
}

// TestGetBlockHeaders_66 tests whether the given node can respond to
// a `GetBlockHeaders` request on the eth66 protocol and that the response
// is accurate.
func (s *Suite) TestGetBlockHeaders_66(t *utesting.T) {
	s. getBlockHeaders(t, s.eth66StatusMessage())
}

// TestGetBlockBodies_66 tests whether the given node can respond to
// a `GetBlockBodies` request on the eth66 protocol and that the response
// is accurate.
func (s *Suite) TestGetBlockBodies_66(t *utesting.T) {
	s.getBlockBodies(t, s.eth66StatusMessage())
}

// TestBroadcast_66 tests whether a block announcement is correctly
// propagated to the given node's peer(s) on the eth66 protocol.
func (s *Suite) TestBroadcast_66(t *utesting.T) {
	s.broadcast(t, s.eth66StatusMessage())
}


func (s *Suite) TestMaliciousStatus_66(t *utesting.T) {
	conn, err := s.dial()
	if err != nil {
		t.Fatalf("could not dial: %v", err)
	}
	// get protoHandshake
	conn.handshake(t, s.caps)
	status := &Status{
		ProtocolVersion: 66,
		NetworkID:       s.chain.chainConfig.ChainID.Uint64(),
		TD:              largeNumber(2),
		Head:            s.chain.blocks[s.chain.Len()-1].Hash(),
		Genesis:         s.chain.blocks[0].Hash(),
		ForkID:          s.chain.ForkID(),
	}
	// get status
	switch msg := conn.statusExchange(t, s.chain, status).(type) {
	case *Status:
		t.Logf("%+v\n", msg)
	default:
		t.Fatalf("expected status, got: %#v ", msg)
	}
	// wait for disconnect
	switch msg := conn.ReadAndServe(s.chain, timeout).(type) {
	case *Disconnect:
	case *Error:
		return
	default:
		t.Fatalf("expected disconnect, got: %s", pretty.Sdump(msg))
	}
}

//
func (s *Suite) TestMaliciousHandshake_66(t *utesting.T) {
	conn, err := s.dial()
	if err != nil {
		t.Fatalf("could not dial: %v", err)
	}
	// write hello to client
	pub0 := crypto.FromECDSAPub(&conn.ourKey.PublicKey)[1:]
	handshakes := []*Hello{
		{
			Version: 5,
			Caps: []p2p.Cap{
				{Name: largeString(2), Version: 66},
			},
			ID: pub0,
		},
		{
			Version: 5,
			Caps: []p2p.Cap{
				{Name: "eth", Version: 66},
			},
			ID: append(pub0, byte(0)),
		},
		{
			Version: 5,
			Caps: []p2p.Cap{
				{Name: "eth", Version: 66},
			},
			ID: append(pub0, pub0...),
		},
		{
			Version: 5,
			Caps: []p2p.Cap{
				{Name: "eth", Version: 66},
			},
			ID: largeBuffer(2),
		},
		{
			Version: 5,
			Caps: []p2p.Cap{
				{Name: largeString(2), Version: 66},
			},
			ID: largeBuffer(2),
		},
	}
	for i, handshake := range handshakes {
		t.Logf("Testing malicious handshake %v\n", i)
		// Init the handshake
		if err := conn.Write(handshake); err != nil {
			t.Fatalf("could not write to connection: %v", err)
		}
		// check that the peer disconnected
		timeout := 20 * time.Second
		// Discard one hello
		for i := 0; i < 2; i++ {
			switch msg := conn.ReadAndServe(s.chain, timeout).(type) {
			case *Disconnect:
			case *Error:
			case *Hello:
				// Hello's are send concurrently, so ignore them
				continue
			default:
				t.Fatalf("unexpected: %s", pretty.Sdump(msg))
			}
		}
		// Dial for the next round
		conn, err = s.dial()
		if err != nil {
			t.Fatalf("could not dial: %v", err)
		}
	}
}

// TestLargeAnnounce tests the announcement mechanism with a large block.
func (s *Suite) TestLargeAnnounce_66(t *utesting.T) {
	s.largeAnnounce(t, s.eth66StatusMessage())
}

