// TODO add license

package ethtest

import (
	"fmt"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/internal/utesting"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/assert"
	"time"
)

// Eth66Tests returns eth protocol tests for the eth 66 protocol version
func (s *Suite) Eth66Tests() []utesting.Test {
	return []utesting.Test{
		{Name: "Status_66", Fn: s.TestStatus_66},
		{Name: "GetBlockHeaders_66", Fn: s.TestGetBlockHeaders_66},
		{Name: "Broadcast_66", Fn: s.TestBroadcast_66},
		//{Name: "GetBlockBodies_66", Fn: s.TestGetBlockBodies},
		//{Name: "TestLargeAnnounce_66", Fn: s.TestLargeAnnounce},
		//{Name: "TestMaliciousHandshake_66", Fn: s.TestMaliciousHandshake},
		//{Name: "TestMaliciousStatus_66", Fn: s.TestMaliciousStatus},
		//{Name: "TestTransactions_66", Fn: s.TestTransaction},
		//{Name: "TestMaliciousTransactions_66", Fn: s.TestMaliciousTx},
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

func (c *Conn) statusExchange_66(t *utesting.T, chain *Chain) Message {
	status := &Status{
		ProtocolVersion: uint32(66),
		NetworkID:       chain.chainConfig.ChainID.Uint64(),
		TD:              chain.TD(chain.Len()),
		Head:            chain.blocks[chain.Len()-1].Hash(),
		Genesis:         chain.blocks[0].Hash(),
		ForkID:          chain.ForkID(),
	}
	return c.statusExchange(t, chain, status)
}

func (c *Conn) write66(req eth.Packet, code int) error {
	payload, err := rlp.EncodeToBytes(req)
	if err != nil {
		return err
	}
	_, err = c.Conn.Write(uint64(code), payload)
	return err
}

func (c *Conn) read66() (uint64, Message) {
	code, rawData, _, err := c.Conn.Read()
	if err != nil {
		return 0, errorf("could not read from connection: %v", err)
	}

	var msg Message

	switch int(code) {
	case (Hello{}).Code():
		msg = new(Hello)

	case (Ping{}).Code():
		msg = new(Ping)
	case (Pong{}).Code():
		msg = new(Pong)
	case (Disconnect{}).Code():
		msg = new(Disconnect)
	case (Status{}).Code():
		msg = new(Status)
	case (GetBlockHeaders{}).Code():
		ethMsg := new(eth.GetBlockHeadersPacket66)
		if err := rlp.DecodeBytes(rawData, ethMsg); err != nil {
			return 0, errorf("could not rlp decode message: %v", err)
		}
		return ethMsg.RequestId, GetBlockHeaders(*ethMsg.GetBlockHeadersPacket)
	case (BlockHeaders{}).Code():
		ethMsg := new(eth.BlockHeadersPacket66)
		if err := rlp.DecodeBytes(rawData, ethMsg); err != nil {
			return 0, errorf("could not rlp decode message: %v", err)
		}
		return ethMsg.RequestId, BlockHeaders(ethMsg.BlockHeadersPacket)
	case (GetBlockBodies{}).Code():
		ethMsg := new(eth.GetBlockBodiesPacket66)
		if err := rlp.DecodeBytes(rawData, ethMsg); err != nil {
			return 0, errorf("could not rlp decode message: %v", err)
		}
		return ethMsg.RequestId, GetBlockBodies(ethMsg.GetBlockBodiesPacket)
	case (BlockBodies{}).Code():
		ethMsg := new(eth.BlockBodiesPacket66)
		if err := rlp.DecodeBytes(rawData, ethMsg); err != nil {
			return 0, errorf("could not rlp decode message: %v", err)
		}
		return ethMsg.RequestId, BlockBodies(ethMsg.BlockBodiesPacket)
	case (NewBlock{}).Code():
		msg = new(NewBlock)
	case (NewBlockHashes{}).Code():
		msg = new(NewBlockHashes)
	case (Transactions{}).Code():
		msg = new(Transactions)
	case (NewPooledTransactionHashes{}).Code():
		msg = new(NewPooledTransactionHashes)
	default:
		msg = errorf("invalid message code: %d", code)
	}

	if msg != nil {
		if err := rlp.DecodeBytes(rawData, msg); err != nil {
			return 0, errorf("could not rlp decode message: %v", err)
		}
		return 0, msg
	}
	return 0, errorf("invalid message: %s", string(rawData))
}

// ReadAndServe serves GetBlockHeaders requests while waiting
// on another message from the node.
func (c *Conn) readAndServe66(expectedID uint64, chain *Chain, timeout time.Duration) Message {
	start := time.Now()
	for time.Since(start) < timeout {
		timeout := time.Now().Add(10 * time.Second)
		c.SetReadDeadline(timeout)

		reqID, msg := c.read66()

		switch msg.(type) {
		case *Ping:
			c.Write(&Pong{})
		case *GetBlockHeaders:
			// check request IDg
			if reqID != expectedID {
				return errorf("request ID mismatch: wanted %d, got %d", expectedID, reqID)
			}
			req := *msg.(*GetBlockHeaders)
			headers, err := chain.GetHeaders(req)
			if err != nil {
				return errorf("could not get headers for inbound header request: %v", err)
			}

			if err := c.Write(headers); err != nil {
				return errorf("could not write to connection: %v", err)
			}
		default:
			return msg
		}
	}
	return errorf("no message received within %v", timeout)
}

func (s *Suite) setupConnection66(t *utesting.T) *Conn {
	// create conn
	sendConn := s.dial_66(t)
	sendConn.handshake(t)
	sendConn.statusExchange_66(t, s.chain)
	return sendConn
}

func (s *Suite) testAnnounce66(t *utesting.T, sendConn, receiveConn *Conn, blockAnnouncement *NewBlock) {
	// Announce the block.
	if err := sendConn.Write(blockAnnouncement); err != nil {
		t.Fatalf("could not write to connection: %v", err)
	}
	s.waitAnnounce66(t, receiveConn, blockAnnouncement)
}

func (s *Suite) waitAnnounce66(t *utesting.T, conn *Conn, blockAnnouncement *NewBlock) {
	timeout := 20 * time.Second
	switch msg := conn.readAndServe66(0, s.chain, timeout).(type) {
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
		message := *msg
		t.Logf("received NewBlockHashes message: %s", pretty.Sdump(message))
		assert.Equal(t, blockAnnouncement.Block.Hash(), message[0].Hash,
			"wrong block hash in announcement",
		)
	default:
		t.Fatalf("unexpected: %s", pretty.Sdump(msg))
	}
}

// waitForBlock_66 waits for confirmation from the client that it has
// imported the given block.
func (c *Conn) waitForBlock_66(block *types.Block) error {
	defer c.SetReadDeadline(time.Time{})

	timeout := time.Now().Add(20 * time.Second)
	c.SetReadDeadline(timeout)
	for {
		req := eth.GetBlockHeadersPacket66{
			RequestId:             54,
			GetBlockHeadersPacket: &eth.GetBlockHeadersPacket{
				Origin: eth.HashOrNumber{
					Hash: block.Hash(),
				},
				Amount: 1,
			},
		}
		if err := c.write66(req, GetBlockHeaders{}.Code()); err != nil {
			return err
		}

		reqID, msg := c.read66()
		// check message
		switch msg.(type) {
		case BlockHeaders:
			// check request ID
			if reqID != req.RequestId {
				return fmt.Errorf("request ID mismatch: wanted %d, got %d", req.RequestId, reqID)
			}
			if len(msg.(BlockHeaders)) > 0 {
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		default:
			return fmt.Errorf("invalid message: %s", pretty.Sdump(msg))
		}
	}
}
