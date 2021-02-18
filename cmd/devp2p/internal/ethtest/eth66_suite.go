
package ethtest

import (
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/internal/utesting"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/stretchr/testify/assert"
	"time"
)

// Eth66Tests returns all eth 66 protocol version tests
func (s *Suite) Eth66Tests() []utesting.Test {
	return []utesting.Test{
		{Name: "Status_66", Fn: s.TestStatus_66},
		{Name: "GetBlockHeaders_66", Fn: s.TestGetBlockHeaders_66},
		{Name: "Broadcast_66", Fn: s.TestBroadcast_66},
		{Name: "GetBlockBodies_66", Fn: s.TestGetBlockBodies_66},
		{Name: "TestLargeAnnounce_66", Fn: s.TestLargeAnnounce_66},
		{Name: "TestMaliciousHandshake_66", Fn: s.TestMaliciousHandshake_66},
		{Name: "TestMaliciousStatus_66", Fn: s.TestMaliciousStatus},
		{Name: "TestTransactions_66", Fn: s.TestTransaction_66},
		{Name: "TestMaliciousTransactions_66", Fn: s.TestMaliciousTx},
	}
}

// TestStatus_66 attempts to connect to the given node and exchange
// a status message with it, and then check to make sure
// the chain head is correct.
func (s *Suite) TestStatus_66(t *utesting.T) {
	conn := s.dial_66(t)
	// get protoHandshake
	conn.handshake(t)
	// get status
	switch msg := conn.statusExchange_66(t, s.chain).(type) {
	case *Status:
		status := *msg
		if status.ProtocolVersion != uint32(66) {
			t.Fatalf("mismatch in version: wanted 66, got %d", status.ProtocolVersion)
		}
		t.Logf("got status message: %s", pretty.Sdump(msg))
	default:
		t.Fatalf("unexpected: %s", pretty.Sdump(msg))
	}
}

// TestGetBlockHeaders_66 tests whether the given node can respond to
// an eth66 `GetBlockHeaders` request and that the response is accurate.
func (s *Suite) TestGetBlockHeaders_66(t *utesting.T) {
	conn := s.setupConnection66(t)
	// get block headers
	req := &eth.GetBlockHeadersPacket66{
		RequestId:             3,
		GetBlockHeadersPacket: &eth.GetBlockHeadersPacket{
			Origin: eth.HashOrNumber{
				Hash: s.chain.blocks[1].Hash(),
			},
			Amount:  2,
			Skip:    1,
			Reverse: false,
		},
	}
	// write message
	if err := conn.write66(req, GetBlockHeaders{}.Code()); err != nil {
		t.Fatalf("could not write to connection: %v", err)
	}
	// check block headers response
	switch msg := conn.readAndServe66(req.RequestId, s.chain, timeout).(type) {
	case BlockHeaders:
		for _, header := range msg {
			num := header.Number.Uint64()
			t.Logf("received header (%d): %s", num, pretty.Sdump(header))
			assert.Equal(t, s.chain.blocks[int(num)].Header(), header)
		}
	default:
		t.Fatalf("unexpected: %s", pretty.Sdump(msg))
	}
}

// TestBroadcast_66 tests whether a block announcement is correctly
// propagated to the given node's peer(s).
func (s *Suite) TestBroadcast_66(t *utesting.T) {
	sendConn, receiveConn := s.setupConnection66(t), s.setupConnection66(t)
	nextBlock := len(s.chain.blocks)
	blockAnnouncement := &NewBlock{
		Block: s.fullChain.blocks[nextBlock],
		TD:    s.fullChain.TD(nextBlock + 1),
	}
	s.testAnnounce66(t, sendConn, receiveConn, blockAnnouncement)
	// update test suite chain
	s.chain.blocks = append(s.chain.blocks, s.fullChain.blocks[nextBlock])
	// wait for client to update its chain
	if err := receiveConn.waitForBlock_66(s.chain.Head()); err != nil {
		t.Fatal(err)
	}
}

func (s *Suite) dial_66(t *utesting.T) *Conn {
	conn, err := s.dial()
	if err != nil {
		t.Fatalf("could not dial: %v", err)
	}
	conn.caps = append(conn.caps, p2p.Cap{"eth", 66})
	return conn
}

// TestGetBlockBodies_66 tests whether the given node can respond to
// a `GetBlockBodies` request and that the response is accurate.
func (s *Suite) TestGetBlockBodies_66(t *utesting.T) {
	conn := s.setupConnection66(t)
	// create block bodies request
	id := uint64(55)
	req := &eth.GetBlockBodiesPacket66{
		RequestId:            id,
		GetBlockBodiesPacket: eth.GetBlockBodiesPacket{
			s.chain.blocks[54].Hash(),
			s.chain.blocks[75].Hash(),
		},
	}
	if err := conn.write66(req, GetBlockBodies{}.Code()); err != nil {
		t.Fatalf("could not write to connection: %v", err)
	}

	switch msg := conn.readAndServe66(id, s.chain, timeout).(type) {
	case BlockBodies:
		t.Logf("received %d block bodies", len(msg))
	default:
		t.Fatalf("unexpected: %s", pretty.Sdump(msg))
	}
}

// TestLargeAnnounce_66 tests the announcement mechanism with a large block.
func (s *Suite) TestLargeAnnounce_66(t *utesting.T) {
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
		sendConn := s.setupConnection66(t)
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
	sendConn := s.setupConnection66(t)
	receiveConn := s.setupConnection66(t)
	s.testAnnounce66(t, sendConn, receiveConn, blocks[3])
	// update test suite chain
	s.chain.blocks = append(s.chain.blocks, s.fullChain.blocks[nextBlock])
	// wait for client to update its chain
	if err := receiveConn.waitForBlock_66(s.fullChain.blocks[nextBlock]); err != nil {
		t.Fatal(err)
	}
}

// TestMaliciousHandshake_66 tries to send malicious data during the handshake.
func (s *Suite) TestMaliciousHandshake_66(t *utesting.T) {
	conn := s.dial_66(t)
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
				{Name: "eth", Version: 64},
				{Name: "eth", Version: 65},
				{Name: "eth", Version: 66},
			},
			ID: append(pub0, byte(0)),
		},
		{
			Version: 5,
			Caps: []p2p.Cap{
				{Name: "eth", Version: 64},
				{Name: "eth", Version: 65},
				{Name: "eth", Version: 66},
			},
			ID: append(pub0, pub0...),
		},
		{
			Version: 5,
			Caps: []p2p.Cap{
				{Name: "eth", Version: 64},
				{Name: "eth", Version: 65},
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
		conn = s.dial_66(t)
	}
}

// TestMaliciousStatus_66 sends a status package with a large total difficulty.
func (s *Suite) TestMaliciousStatus_66(t *utesting.T) {
	conn := s.dial_66(t)
	// get protoHandshake
	conn.handshake(t)
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

func (s *Suite) TestTransaction_66(t *utesting.T) {
	tests := []*types.Transaction{
		getNextTxFromChain(t, s),
		unknownTx(t, s),
	}
	for i, tx := range tests {
		t.Logf("Testing tx propagation: %v\n", i)
		sendSuccessfulTx66(t, s, tx)
	}
}

func (s *Suite) TestMaliciousTx_66(t *utesting.T) {
	tests := []*types.Transaction{
		getOldTxFromChain(t, s),
		invalidNonceTx(t, s),
		hugeAmount(t, s),
		hugeGasPrice(t, s),
		hugeData(t, s),
	}
	for i, tx := range tests {
		t.Logf("Testing malicious tx propagation: %v\n", i)
		sendFailingTx66(t, s, tx)
	}
}
