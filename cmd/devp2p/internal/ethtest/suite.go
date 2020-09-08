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
		 Dest: dest,
		 blocks: blocks,
	}
}

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
	for i := 0;; i++ {
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
		{
			"Status",
			s.TestStatus,
		},
	}
}

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
	switch msg := Read(conn).(type) {
	case *Hello:
		fmt.Printf("%+v\n", msg)
	default:
		t.Fatalf("bad message: %v", msg)
	}

	// get status msg
	switch msg := Read(conn).(type) {
	case *Status:
		fmt.Printf("%+v\n", msg)
		if msg.Head != s.blocks[len(s.blocks)-1].Hash() {
			t.Fatalf("wrong head in status exchange: %v", msg.Head)
		}
	default:
		t.Fatalf("bad message: %v", msg)
	}
}

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
