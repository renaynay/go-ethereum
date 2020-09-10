package ethtest

import (
	"compress/gzip"
	"crypto/ecdsa"
	"fmt"
	"github.com/ethereum/go-ethereum/core/forkid"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/internal/utesting"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/rlpx"
	"github.com/ethereum/go-ethereum/rlp"
	"io"
	"math/big"
	"net"
	"os"
	"strings"
)

// Suite represents a structure used to test the eth
// protocol of a node(s).
type Suite struct {
	Dest   *enode.Node
	OurKey *ecdsa.PrivateKey

	blocks []*types.Block
}

// NewSuite creates and returns a new eth-test suite that can
// be used to test the given node against the given blockchain
// data.
func NewSuite(dest *enode.Node, chainfile string) *Suite {
	blocks, err := loadChain(chainfile)
	if err != nil {
		panic(err)
	}
	return &Suite{
		Dest:   dest,
		blocks: blocks,
	}
}

// loadChain takes the given chain.rlp file, and decodes and returns
// the blocks from the file.
func loadChain(chainfile string) ([]*types.Block, error) {
	// Open the file handle and potentially unwrap the gzip stream
	fh, err := os.Open(chainfile)
	if err != nil {
		return nil, err
	}
	defer fh.Close()

	var reader io.Reader = fh
	if strings.HasSuffix(chainfile, ".gz") {
		if reader, err = gzip.NewReader(reader); err != nil {
			return nil, err
		}
	}
	stream := rlp.NewStream(reader, 0)
	var blocks []*types.Block
	for i := 0; ; i++ {
		var b types.Block
		if err := stream.Decode(&b); err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("at block %d: %v", i, err)
		}
		blocks = append(blocks, &b)
	}

	return blocks, nil
}

func (s *Suite) AllTests() []utesting.Test {
	return []utesting.Test{
		{"Ping", s.TestPing},
		{"Status", s.TestStatus},
		{"GetBlockHeaders", s.TestGetBlockHeaders},
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
	msg := s.handshake(conn, t)
	fmt.Printf("%+v\n", msg)
}

// handshake checks to make sure a `HELLO` is received.
func (s *Suite) handshake(conn *rlpx.Conn, t *utesting.T) Message {
	// write protoHandshake to client
	pub0 := crypto.FromECDSAPub(&s.OurKey.PublicKey)[1:]
	ourHandshake := &Hello{
		Version: 3,
		Caps:    []p2p.Cap{{"eth", 64}, {"eth", 65}},
		ID:      pub0,
	}
	if err := Write(conn, ourHandshake); err != nil {
		t.Fatalf("could not write to connection: %v", err)
	}
	// read protoHandshake from client
	switch msg := Read(conn).(type) {
	case *Hello:
		return msg
	default:
		t.Fatalf("bad handshake: %v", msg)
		return nil
	}
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
	s.handshake(conn, t)
	// get status
	msg := s.statusExchange(conn, t) // todo make this a switch
	fmt.Printf("%+v\n", msg)
}

// statusExchange performs a `Status` message exchange with the given
// node.
func (s *Suite) statusExchange(conn *rlpx.Conn, t *utesting.T) Message {
	// read status message from client
	var message Message
	switch msg := Read(conn).(type) {
	case *Status:
		if msg.Head != s.blocks[len(s.blocks)-1].Hash() {
			t.Fatalf("wrong head in status exchange: %v", msg.Head)
		}
		message = msg
	default:
		t.Fatalf("bad status message: %v", msg)
	}
	// write status message to client
	status := Status{
		ProtocolVersion: 65,
		NetworkID:       1,
		TD:              big.NewInt(262144016),
		Head:            s.blocks[len(s.blocks)-1].Hash(),
		Genesis:         s.blocks[0].Hash(),
		ForkID: forkid.ID{
			Hash: [4]byte{80, 147, 31, 15},
			Next: 0,
		},
	}
	if err := Write(conn, status)
		err != nil {
		t.Fatalf("could not write to connection: %v", err)
	}

	return message
}

// TestGetBlockHeaders tests whether the given node can respond to
// a `GetBlockHeaders` request and that the response is accurate.
func (s *Suite) TestGetBlockHeaders(t *utesting.T) {
	conn, err := s.dial()
	if err != nil {
		t.Fatalf("could not dial: %v", err)
	}

	s.handshake(conn, t)
	s.statusExchange(conn, t)

	// get block headers // TODO eventually make this customizable with CL args (take from a file)?
	req := &GetBlockHeaders{
		Origin:  hashOrNumber{
			Hash: s.blocks[1].Hash(),
		},
		Amount:  2,
		Skip: 1,
		Reverse: false,
	}

	if err := Write(conn, req); err != nil {
		t.Fatalf("could not write to connection: %v", err)
	}

	msg := Read(conn)
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

	s.handshake(conn, t)
	s.statusExchange(conn, t)
	// create block bodies request
	req := &GetBlockBodies{s.blocks[54].Hash(), s.blocks[75].Hash()}
	if err := Write(conn, req); err != nil {
		t.Fatalf("could not write to connection: %v", err)
	}

	msg := Read(conn)
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

func (s *Suite) TestBroadcast(t *utesting.T) {
	// create a connection to send a block announcement
	sendConn, err := s.dial()
	if err != nil {
		t.Fatalf("could not dial: %v", err)
	}
	// create a connection to receive a block announcement
	receiveConn, err := s.dial()
	if err != nil {
		t.Fatalf("could not dial: %v", err)
	}

	s.handshake(sendConn, t)
	s.handshake(receiveConn, t)

	s.statusExchange(sendConn, t)
	s.statusExchange(receiveConn, t)

	blockAnnouncement := &NewBlockHashes{}


}

// dial attempts to dial the given node and perform a handshake,
// returning the created Conn if successful.
func (s *Suite) dial() (*rlpx.Conn, error) {
	fd, err := net.Dial("tcp", fmt.Sprintf("%v:%d", s.Dest.IP(), s.Dest.TCP()))
	if err != nil {
		return nil, err
	}
	conn := rlpx.NewConn(fd, s.Dest.Pubkey())

	// do encHandshake
	s.OurKey, _ = crypto.GenerateKey()
	_, err = conn.Handshake(s.OurKey)
	if err != nil {
		return nil, err
	}

	return conn, nil
}
