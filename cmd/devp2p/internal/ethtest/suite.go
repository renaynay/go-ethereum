package ethtest

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/stretchr/testify/assert"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/internal/utesting"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/rlpx"
	"net"
	"reflect"
)

// TODO REPLACE PRINT WITH T.LOGF
// todo use assert for comparisons

// Suite represents a structure used to test the eth
// protocol of a node(s).
type Suite struct {
	Dest *enode.Node

	// TODO FULL CHAIN vs CHAIN (SO THAT YOU CAN RUN TESTS IN ANY ORDER BC BLOCK PROP TEST WILL INCREMENT CHAIN)
	chain     *Chain
	fullChain *Chain
}

type Conn struct {
	*rlpx.Conn
	ourKey *ecdsa.PrivateKey
}

// handshake checks to make sure a `HELLO` is received.
func (c *Conn) handshake(t *utesting.T) Message {
	// write protoHandshake to client
	pub0 := crypto.FromECDSAPub(&c.ourKey.PublicKey)[1:]
	ourHandshake := &Hello{
		Version: 3,
		Caps:    []p2p.Cap{{"eth", 64}, {"eth", 65}},
		ID:      pub0,
	}
	if err := Write(c.Conn, ourHandshake); err != nil {
		t.Fatalf("could not write to connection: %v", err)
	}
	// read protoHandshake from client
	switch msg := Read(c.Conn).(type) {
	case *Hello:
		return msg
	default:
		t.Fatalf("bad handshake: %v", msg)
		return nil
	}
}

// statusExchange performs a `Status` message exchange with the given
// node.
func (c *Conn) statusExchange(t *utesting.T, chain *Chain) Message {
	// read status message from client
	var message Message
	switch msg := Read(c.Conn).(type) {
	case *Status: // TODO you can impl more checks here (TD for ex.)
		if msg.Head != chain.blocks[chain.Len()-1].Hash() {
			t.Fatalf("wrong head in status: %v", msg.Head)
		}
		if msg.TD.Cmp(chain.TD(chain.Len())) != 0 {
			t.Fatalf("wrong TD in status: %v", msg.TD)
		}
		if !reflect.DeepEqual(msg.ForkID, chain.ForkID()) {
			t.Fatalf("wrong fork ID in status: %v", msg.ForkID)
		}
		message = msg
	default:
		t.Fatalf("bad status message: %v", msg)
	}
	// write status message to client
	status := Status{
		ProtocolVersion: 65,
		NetworkID:       1,
		TD:              chain.TD(chain.Len()),
		Head:            chain.blocks[chain.Len()-1].Hash(),
		Genesis:         chain.blocks[0].Hash(),
		ForkID:          chain.ForkID(),
	}
	if err := Write(c.Conn, status)
		err != nil {
		t.Fatalf("could not write to connection: %v", err)
	}

	return message
}

// NewSuite creates and returns a new eth-test suite that can
// be used to test the given node against the given blockchain
// data.
func NewSuite(dest *enode.Node, chainfile string, genesisfile string) *Suite {
	chain, err := loadChain(chainfile, genesisfile)
	if err != nil {
		panic(err)
	}
	return &Suite{
		Dest: dest,
		chain: chain.Shorten(1000),
		fullChain: chain,
	}
}

func (s *Suite) AllTests() []utesting.Test {
	return []utesting.Test{
		{"Ping", s.TestPing},
		{"Status", s.TestStatus},
		{"GetBlockHeaders", s.TestGetBlockHeaders},
		{"Broadcast", s.TestBroadcast},
		{"GetBlockBodies", s.TestGetBlockBodies},
	}
}

// TestPing attempts to exchange `HELLO` messages
// with the given node.
func (s *Suite) TestPing(t *utesting.T) {
	conn, err := s.dial()
	if err != nil {
		t.Fatalf("could not dial: %v", err)
	}
	// get protoHandshake
	msg := conn.handshake(t)
	fmt.Printf("%+v\n", msg)
}

// TestStatus attempts to connect to the given node and exchange
// a status message with it, and then check to make sure
// the chain head is correct.
func (s *Suite) TestStatus(t *utesting.T) {
	conn, err := s.dial()
	if err != nil {
		t.Fatalf("could not dial: %v", err)
	}
	// get protoHandshake
	conn.handshake(t)
	// get status
	msg := conn.statusExchange(t, s.chain) // todo make this a switch
	fmt.Printf("%+v\n", msg)
}

// TestGetBlockHeaders tests whether the given node can respond to
// a `GetBlockHeaders` request and that the response is accurate.
func (s *Suite) TestGetBlockHeaders(t *utesting.T) {
	conn, err := s.dial()
	if err != nil {
		t.Fatalf("could not dial: %v", err)
	}

	conn.handshake(t)
	conn.statusExchange(t, s.chain)

	// get block headers // TODO eventually make this customizable with CL args (take from a file)?
	req := &GetBlockHeaders{
		Origin: hashOrNumber{
			Hash: s.chain.blocks[1].Hash(),
		},
		Amount:  2,
		Skip:    1,
		Reverse: false,
	}

	if err := Write(conn.Conn, req); err != nil {
		t.Fatalf("could not write to connection: %v", err)
	}

	msg := Read(conn.Conn)
	switch msg.Code() {
	case 20:
		headers, ok := msg.(*BlockHeaders)
		if !ok {
			t.Fatalf("message %v does not match code %d", msg, msg.Code())
		}
		for _, header := range *headers {
			fmt.Printf("\nHEADER FOR BLOCK NUMBER %d: %+v\n", header.Number, header) // TODO eventually check against our own data
		}
	default:
		t.Fatalf("error reading message: %v", msg)
	}
}

// TestGetBlockBodies tests whether the given node can respond to
// a `GetBlockBodies` request and that the response is accurate.
func (s *Suite) TestGetBlockBodies(t *utesting.T) {
	conn, err := s.dial()
	if err != nil {
		t.Fatalf("could not dial: %v", err)
	}

	conn.handshake(t)
	conn.statusExchange(t, s.chain)
	// create block bodies request
	req := &GetBlockBodies{s.chain.blocks[54].Hash(), s.chain.blocks[75].Hash()}
	if err := Write(conn.Conn, req); err != nil {
		t.Fatalf("could not write to connection: %v", err)
	}

	msg := Read(conn.Conn)
	switch msg.Code() {
	case 22:
		bodies, ok := msg.(*BlockBodies)
		if !ok {
			t.Fatalf("message %v does not match code %d", msg, msg.Code()) // TODO eventually check against our own data
		}
		for _, body := range *bodies {
			fmt.Printf("\nBODY: %+v\n", body)
		}
	default:
		t.Fatalf("error reading message: %v", msg)
	}
}

// TestBroadcast // TODO how to make sure this is compatible with the imported blockchain of the node
func (s *Suite) TestBroadcast(t *utesting.T) {
	// create conn to send block announcement
	sendConn, err := s.dial()
	if err != nil {
		t.Fatalf("could not dial: %v", err)
	}
	// create conn to receive block announcement
	receiveConn, err := s.dial()
	if err != nil {
		t.Fatalf("could not dial: %v", err)
	}

	sendConn.handshake(t)
	receiveConn.handshake(t)

	sendConn.statusExchange(t, s.chain)
	receiveConn.statusExchange(t, s.chain)

	// sendConn sends the block announcement
	blockAnnouncement := &NewBlock{
		Block: s.fullChain.blocks[1000],
		TD: s.fullChain.TD(1001),
	}
	if err := Write(sendConn.Conn, blockAnnouncement); err != nil {
		t.Fatalf("could not write to connection: %v", err)
	}

	switch msg := Read(receiveConn.Conn).(type) {
	case *NewBlock:
		assert.Equal(t, blockAnnouncement.Block.Header(), msg.Block.Header(),
			"wrong block header in announcement")
		assert.Equal(t, blockAnnouncement.TD, msg.TD,
			"wrong TD in announcement")
	case *NewBlockHashes:
		t.Log("NEW BLOCK HASHES: ", "%+v\n", msg) // TODO impl some check here
	default:
		t.Fatal(msg)
	}

	newChain := s.chain.blocks
	newChain = append(newChain, s.fullChain.blocks[1000])

	config := *s.chain.chainConfig

	s.chain = &Chain{
		blocks: newChain,
		chainConfig: &config,
	}
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
