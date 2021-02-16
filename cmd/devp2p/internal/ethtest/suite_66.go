// TODO add license

package ethtest

import (
	"github.com/ethereum/go-ethereum/internal/utesting"
	"github.com/ethereum/go-ethereum/p2p"
)

func (s *Suite) Eth66Tests() []utesting.Test {
	return []utesting.Test{
		{Name: "Status_66", Fn: s.TestStatus_66},
		//{Name: "GetBlockHeaders_66", Fn: s.TestGetBlockHeaders_66},
		//{Name: "Broadcast_66", Fn: s.TestBroadcast},
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
	switch msg := conn.statusExchange_66(t, s).(type) {
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

func (s *Suite) dial_66(t *utesting.T) *Conn {
	conn, err := s.dial()
	if err != nil {
		t.Fatalf("could not dial: %v", err)
	}
	conn.caps = append(conn.caps, p2p.Cap{"eth", 66})
	return conn
}

func (c *Conn) statusExchange_66(t *utesting.T, s *Suite) Message {
	status := &Status{
		ProtocolVersion: uint32(66),
		NetworkID:       s.chain.chainConfig.ChainID.Uint64(),
		TD:              s.chain.TD(s.chain.Len()),
		Head:            s.chain.blocks[s.chain.Len()-1].Hash(),
		Genesis:         s.chain.blocks[0].Hash(),
		ForkID:          s.chain.ForkID(),
	}
	return c.statusExchange(t, s.chain, status)
}

