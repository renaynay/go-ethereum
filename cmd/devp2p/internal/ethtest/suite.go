package ethtest

import (
	"compress/gzip"
	"crypto/ecdsa"
	"fmt"
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

type Suite struct {
	Dest   *enode.Node
	OurKey *ecdsa.PrivateKey

	blocks []*types.Block
}

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

// TestStatus attempts to connect to the given node and exchange
// a status message with it, and then check to make sure
// the chain head is correct.
func (s *Suite) TestStatus(t *utesting.T) {
	conn, err := s.dial()
	if err != nil {
		t.Fatalf("could not dial: %v", err)
	}
	// create and write our protoHandshake
	pub0 := crypto.FromECDSAPub(&s.OurKey.PublicKey)[1:]
	ourHandshake := &Hello{
		Version: 3,
		Caps:    []p2p.Cap{{"eth", 63}, {"eth", 64}},
		ID:      pub0,
	}
	if err := Write(conn, ourHandshake); err != nil {
		t.Fatalf("could not write to connection: %v", err)
	}
	// get protoHandshake
	s.handshake(conn, t)
	// get status
	msg := s.statusExchange(conn, t)
	fmt.Printf("%+v\n", msg)
}

// TestGetBlockHeaders // TODO THIS IS BROKEN
func (s *Suite) TestGetBlockHeaders(t *utesting.T) {
	conn, err := s.dial()
	if err != nil {
		t.Fatalf("could not dial: %v", err)
	}

	s.handshake(conn, t)
	s.statusExchange(conn, t)

	// write status message
	status := &Status{
		ProtocolVersion: 64,
		NetworkID:       1,
		TD:              big.NewInt(262144016),
		Head:            s.blocks[len(s.blocks)-1].Hash(),
		Genesis:         s.blocks[0].Hash(),
	}
	if err := Write(conn, status)
		err != nil {
		t.Fatalf("could not write to connection: %v", err)
	}

	// TODO loop to wait for msgs from the node?

	// get block headers // TODO eventually make this customizable with CL args (take from a file)?
	req := &GetBlockHeaders{
		Origin:  HashOrNumber{
			Hash: s.blocks[0].Hash(),
		},
		Amount:  1,
	}

	fmt.Println("HERE")
	if err := Write(conn, req); err != nil {
		t.Fatalf("could not write to connection: %v", err)
	}

	msg := Read(conn)
	fmt.Println(msg)
}

func (s *Suite) statusExchange(conn *rlpx.Conn, t *utesting.T) Message {
	switch msg := Read(conn).(type) {
	case *Status:
		if msg.Head != s.blocks[len(s.blocks)-1].Hash() {
			t.Fatalf("wrong head in status exchange: %v", msg.Head)
		}
		return msg
	default:
		t.Fatalf("bad status message: %v", msg)
		return nil
	}
}

// handshake checks to make sure a `HELLO` is received.
func (s *Suite) handshake(conn *rlpx.Conn, t *utesting.T) Message {
	switch msg := Read(conn).(type) {
	case *Hello:
		return msg
	default:
		t.Fatalf("bad handshake: %v", msg)
		return nil
	}
}

// dial attempts to dial the given node and perform a handshake,
// returning the created Conn if successful.
func (s *Suite) dial() (*rlpx.Conn, error) {
	fd, err := net.Dial("tcp", fmt.Sprintf("%v:%d", s.Dest.IP(), s.Dest.TCP()))
	if err != nil {
		return nil, err
	}
	conn := rlpx.NewConn(fd, s.Dest.Pubkey())

	s.OurKey, _ = crypto.GenerateKey()
	_, err = conn.Handshake(s.OurKey)
	if err != nil {
		return nil, err
	}

	return conn, nil
}
