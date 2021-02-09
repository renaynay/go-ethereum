// Copyright 2020 The go-ethereum Authors
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
	"fmt"
	"github.com/ethereum/go-ethereum/p2p"
	"net"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/internal/utesting"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/rlpx"
	"github.com/stretchr/testify/assert"
)

var pretty = spew.ConfigState{
	Indent:                  "  ",
	DisableCapacities:       true,
	DisablePointerAddresses: true,
	SortKeys:                true,
}

var timeout = 20 * time.Second

// Suite represents a structure used to test the eth
// protocol of a node(s).
type Suite struct {
	Dest *enode.Node

	caps []p2p.Cap

	chain     *Chain
	fullChain *Chain
}

// NewSuite creates and returns a new eth-test suite that can
// be used to test the given node against the given blockchain
// data.
func NewSuite(dest *enode.Node, chainfile string, genesisfile string, caps []p2p.Cap) *Suite {
	chain, err := loadChain(chainfile, genesisfile)
	if err != nil {
		panic(err)
	}
	return &Suite{
		Dest:      dest,
		caps: 	   caps,
		chain:     chain.Shorten(1000),
		fullChain: chain,
	}
}

func (s *Suite) EthTests() []utesting.Test {
	return []utesting.Test{
		{Name: "Status_65", Fn: s.TestStatus_65},
		{Name: "GetBlockHeaders_65", Fn: s.TestGetBlockHeaders_65},
		{Name: "Broadcast_65", Fn: s.TestBroadcast_65},
		{Name: "GetBlockBodies_65", Fn: s.TestGetBlockBodies_65},
		{Name: "TestLargeAnnounce_65", Fn: s.TestLargeAnnounce_65},
		{Name: "TestMaliciousHandshake", Fn: s.TestMaliciousHandshake},
		{Name: "TestMaliciousStatus_65", Fn: s.TestMaliciousStatus_65},
		{Name: "TestTransactions_65", Fn: s.TestTransaction_65},
		{Name: "TestMaliciousTransactions_65", Fn: s.TestMaliciousTx_65},
	}
}

func (s *Suite) AllTests() []utesting.Test {
	return append(s.EthTests(), s.Eth66Tests()...)
}

// TestStatus_65 attempts to connect to the given node and exchange
// a status message with it, and then check to make sure
// the chain head is correct.
func (s *Suite) TestStatus_65(t *utesting.T) {
	conn, err := s.dial()
	if err != nil {
		t.Fatalf("could not dial: %v", err)
	}
	// get protoHandshake
	conn.handshake(t, s.caps)
	// get status
	switch msg := conn.statusExchange(t, s.chain, nil).(type) {
	case *Status:
		t.Logf("got status message: %s", pretty.Sdump(msg))
	default:
		t.Fatalf("unexpected: %s", pretty.Sdump(msg))
	}
}

// TestMaliciousStatus sends a status package with a large total difficulty.
func (s *Suite) TestMaliciousStatus_65(t *utesting.T) {
	conn, err := s.dial()
	if err != nil {
		t.Fatalf("could not dial: %v", err)
	}
	// get protoHandshake
	conn.handshake(t, s.caps)
	status := &Status{
		ProtocolVersion: uint32(conn.ethProtocolVersion),
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

// TestGetBlockHeaders tests whether the given node can respond to
// a `GetBlockHeaders` request and that the response is accurate.
func (s *Suite) TestGetBlockHeaders_65(t *utesting.T) {
	s.getBlockHeaders(t, nil)
}

func (s *Suite) getBlockHeaders(t *utesting.T, status *Status) {
	conn, err := s.dial()
	if err != nil {
		t.Fatalf("could not dial: %v", err)
	}

	conn.handshake(t, s.caps)
	conn.statusExchange(t, s.chain, status)

	// get block headers
	req := &GetBlockHeaders{
		Origin: hashOrNumber{
			Hash: s.chain.blocks[1].Hash(),
		},
		Amount:  2,
		Skip:    1,
		Reverse: false,
	}

	if err := conn.Write(req); err != nil {
		t.Fatalf("could not write to connection: %v", err)
	}

	switch msg := conn.ReadAndServe(s.chain, timeout).(type) {
	case *BlockHeaders:
		headers := msg
		for _, header := range *headers {
			num := header.Number.Uint64()
			t.Logf("received header (%d): %s", num, pretty.Sdump(header))
			assert.Equal(t, s.chain.blocks[int(num)].Header(), header)
		}
	default:
		t.Fatalf("unexpected: %s", pretty.Sdump(msg))
	}
}

// TestGetBlockBodies_65 tests whether the given node can respond to
// a `GetBlockBodies` request and that the response is accurate.
func (s *Suite) TestGetBlockBodies_65(t *utesting.T) {
	s.getBlockBodies(t, nil)
}

func (s *Suite) getBlockBodies(t *utesting.T, status *Status) {
	conn, err := s.dial()
	if err != nil {
		t.Fatalf("could not dial: %v", err)
	}

	conn.handshake(t, s.caps)
	conn.statusExchange(t, s.chain, status)
	// create block bodies request
	req := &GetBlockBodies{s.chain.blocks[54].Hash(), s.chain.blocks[75].Hash()}
	if err := conn.Write(req); err != nil {
		t.Fatalf("could not write to connection: %v", err)
	}

	switch msg := conn.ReadAndServe(s.chain, timeout).(type) {
	case *BlockBodies:
		t.Logf("received %d block bodies", len(*msg))
	default:
		t.Fatalf("unexpected: %s", pretty.Sdump(msg))
	}
}

// TestBroadcast_65 tests whether a block announcement is correctly
// propagated to the given node's peer(s).
func (s *Suite) TestBroadcast_65(t *utesting.T) {
	s.broadcast(t, nil)
}

func (s *Suite) broadcast(t *utesting.T, status *Status) {
	sendConn, receiveConn := s.setupConnection(t, status), s.setupConnection(t, status)
	nextBlock := len(s.chain.blocks)
	blockAnnouncement := &NewBlock{
		Block: s.fullChain.blocks[nextBlock],
		TD:    s.fullChain.TD(nextBlock + 1),
	}
	s.testAnnounce(t, sendConn, receiveConn, blockAnnouncement)
	// update test suite chain
	s.chain.blocks = append(s.chain.blocks, s.fullChain.blocks[nextBlock])
	// wait for client to update its chain
	if err := receiveConn.waitForBlock(s.chain.Head()); err != nil {
		t.Fatal(err)
	}
}

// TestMaliciousHandshake tries to send malicious data during the handshake.
func (s *Suite) TestMaliciousHandshake(t *utesting.T) {
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
				{Name: largeString(2), Version: 64},
			},
			ID: pub0,
		},
		{
			Version: 5,
			Caps: []p2p.Cap{
				{Name: "eth", Version: 64},
				{Name: "eth", Version: 65},
			},
			ID: append(pub0, byte(0)),
		},
		{
			Version: 5,
			Caps: []p2p.Cap{
				{Name: "eth", Version: 64},
				{Name: "eth", Version: 65},
			},
			ID: append(pub0, pub0...),
		},
		{
			Version: 5,
			Caps: []p2p.Cap{
				{Name: "eth", Version: 64},
				{Name: "eth", Version: 65},
			},
			ID: largeBuffer(2),
		},
		{
			Version: 5,
			Caps: []p2p.Cap{
				{Name: largeString(2), Version: 64},
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

// TestLargeAnnounce_65 tests the announcement mechanism with a large block.
func (s *Suite) TestLargeAnnounce_65(t *utesting.T) {
	s.largeAnnounce(t, nil)
}

func (s *Suite) largeAnnounce(t *utesting.T, status *Status) {
	nextBlock := len(s.chain.blocks)
	blocks := []*NewBlock{
		{
			Block: largeBlock(),
			TD:    s.fullChain.TD(nextBlock + 1),
		},
		{
			Block: s.fullChain.blocks[nextBlock],
			TD:    largeNumber(2),
		},
		{
			Block: largeBlock(),
			TD:    largeNumber(2),
		},
		{
			Block: s.fullChain.blocks[nextBlock],
			TD:    s.fullChain.TD(nextBlock + 1),
		},
	}

	for i, blockAnnouncement := range blocks[0:3] {
		t.Logf("Testing malicious announcement: %v\n", i)
		sendConn := s.setupConnection(t, status)
		if err := sendConn.Write(blockAnnouncement); err != nil {
			t.Fatalf("could not write to connection: %v", err)
		}
		// Invalid announcement, check that peer disconnected
		switch msg := sendConn.ReadAndServe(s.chain, timeout).(type) {
		case *Disconnect:
		case *Error:
			break
		default:
			t.Fatalf("unexpected: %s wanted disconnect", pretty.Sdump(msg))
		}
	}
	// Test the last block as a valid block
	sendConn := s.setupConnection(t, nil)
	receiveConn := s.setupConnection(t, nil)
	s.testAnnounce(t, sendConn, receiveConn, blocks[3])
	// update test suite chain
	s.chain.blocks = append(s.chain.blocks, s.fullChain.blocks[nextBlock])
	// wait for client to update its chain
	if err := receiveConn.waitForBlock(s.fullChain.blocks[nextBlock]); err != nil {
		t.Fatal(err)
	}
}

func (s *Suite) testAnnounce(t *utesting.T, sendConn, receiveConn *Conn, blockAnnouncement *NewBlock) {
	// Announce the block.
	if err := sendConn.Write(blockAnnouncement); err != nil {
		t.Fatalf("could not write to connection: %v", err)
	}
	s.waitAnnounce(t, receiveConn, blockAnnouncement)
}

func (s *Suite) waitAnnounce(t *utesting.T, conn *Conn, blockAnnouncement *NewBlock) {
	timeout := 20 * time.Second
	switch msg := conn.ReadAndServe(s.chain, timeout).(type) {
	case *NewBlock:
		t.Logf("received NewBlock message: %s", pretty.Sdump(msg.Block))
		assert.Equal(t,
			blockAnnouncement.Block.Header(), msg.Block.Header(),
			"wrong block header in announcement",
		)
		assert.Equal(t,
			blockAnnouncement.TD, msg.TD,
			"wrong TD in announcement",
		)
	case *NewBlockHashes:
		hashes := *msg
		t.Logf("received NewBlockHashes message: %s", pretty.Sdump(hashes))
		assert.Equal(t,
			blockAnnouncement.Block.Hash(), hashes[0].Hash,
			"wrong block hash in announcement",
		)
	default:
		t.Fatalf("unexpected: %s", pretty.Sdump(msg))
	}
}

func (s *Suite) setupConnection(t *utesting.T, status *Status) *Conn {
	// create conn
	sendConn, err := s.dial()
	if err != nil {
		t.Fatalf("could not dial: %v", err)
	}
	sendConn.handshake(t, s.caps)
	sendConn.statusExchange(t, s.chain, status)
	return sendConn
}

// dial attempts to dial the given node and perform a handshake,
// returning the created Conn if successful.
func (s *Suite) dial() (*Conn, error) {
	var conn Conn

	fd, err := net.Dial("tcp", fmt.Sprintf("%v:%d", s.Dest.IP(), s.Dest.TCP()))
	if err != nil {
		return nil, err
	}
	conn.Conn = rlpx.NewConn(fd, s.Dest.Pubkey())

	// do encHandshake
	conn.ourKey, _ = crypto.GenerateKey()
	_, err = conn.Handshake(conn.ourKey)
	if err != nil {
		return nil, err
	}

	return &conn, nil
}

func (s *Suite) TestTransaction_65(t *utesting.T) {
	s.transaction(t, nil)
}

func (s *Suite) transaction(t *utesting.T, status *Status) {
	tests := []*types.Transaction{
		getNextTxFromChain(t, s),
		unknownTx(t, s),
	}
	for i, tx := range tests {
		t.Logf("Testing tx propagation: %v\n", i)
		sendSuccessfulTx(t, s, tx, status)
	}
}

func (s *Suite) TestMaliciousTx_65(t *utesting.T) {
	s.maliciousTx(t, nil)
}

func (s *Suite) maliciousTx(t *utesting.T, status *Status) {
	tests := []*types.Transaction{
		getOldTxFromChain(t, s),
		invalidNonceTx(t, s),
		hugeAmount(t, s),
		hugeGasPrice(t, s),
		hugeData(t, s),
	}
	for i, tx := range tests {
		t.Logf("Testing malicious tx propagation: %v\n", i)
		sendFailingTx(t, s, tx, status)
	}
}
