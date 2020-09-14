package ethtest

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/forkid"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/rlpx"
	"github.com/ethereum/go-ethereum/rlp"
	"io"
	"math/big"
)

type Message interface {
	Code() int
	//Protocol() int // TODO
}

func Read(conn *rlpx.Conn) Message {
	code, rawData, err := conn.Read()
	if err != nil {
		return &Error{fmt.Errorf("could not read from connection: %v", err)}
	}

	var msg Message
	switch int(code) {
	case (Hello{}).Code():
		msg = new(Hello)
	case (Disc{}).Code():
		msg = new(Disc)
	case (Status{}).Code():
		msg = new(Status)
	case (GetBlockHeaders{}).Code():
		msg = new(GetBlockHeaders)
	case (BlockHeaders{}).Code():
		msg = new(BlockHeaders)
	case (GetBlockBodies{}).Code():
		msg = new(GetBlockBodies)
	case (BlockBodies{}).Code():
		msg = new(BlockBodies)
	case (NewBlock{}).Code():
		msg = new(NewBlock)
	case (NewBlockHashes{}).Code():
		msg = new(NewBlockHashes)
	default:
		return &Error{fmt.Errorf("invalid message code: %d", code)}
	}

	if err := rlp.DecodeBytes(rawData, msg); err != nil {
		return &Error{fmt.Errorf("could not rlp decode message: %v", err)}
	}

	return msg
}

func Write(conn *rlpx.Conn, msg Message) error { // TODO eventually put this method on the Conn
	size, payload, err := rlp.EncodeToReader(msg)
	if err != nil {
		return err
	}
	_, err = conn.WriteMsg(uint64(msg.Code()), uint32(size), payload)
	return err
}

type Error struct {
	err error
}

func (e *Error) Unwrap() error { return e.err }
func (e *Error) Error() string { return e.err.Error() }
func (e *Error) Code() int     { return -1 }

// Hello is the RLP structure of the protocol handshake.
type Hello struct {
	Version    uint64
	Name       string
	Caps       []p2p.Cap
	ListenPort uint64
	ID         []byte // secp256k1 public key

	// Ignore additional fields (for forward compatibility).
	Rest []rlp.RawValue `rlp:"tail"`
}

func (h Hello) Code() int { return 0x00 }

type Disc struct {
	Reason p2p.DiscReason
}

func (d Disc) Code() int { return 0x01 }

// Status is the network packet for the status message for eth/64 and later.
type Status struct {
	ProtocolVersion uint32
	NetworkID       uint64
	TD              *big.Int
	Head            common.Hash
	Genesis         common.Hash
	ForkID          forkid.ID
}

func (s Status) Code() int { return 16 }

//func (sd *statusData) Protocol() int {
//
//}

// NewBlockHashes is the network packet for the block announcements.
type NewBlockHashes []struct {
	Hash   common.Hash // Hash of one particular block being announced
	Number uint64      // Number of one particular block being announced
}
func (nbh NewBlockHashes) Code() int { return 17 }

// NewBlock is the network packet for the block propagation message.
type NewBlock struct {
	Block *types.Block
	TD    *big.Int
}
func (nb NewBlock) Code() int { return 23 }

// GetBlockHeaders represents a block header query.
type GetBlockHeaders struct {
	Origin  hashOrNumber // Block from which to retrieve headers
	Amount  uint64       // Maximum number of headers to retrieve
	Skip    uint64       // Blocks to skip between consecutive headers
	Reverse bool         // Query direction (false = rising towards latest, true = falling towards genesis)
}
func (g GetBlockHeaders) Code() int { return 19 }

type BlockHeaders []*types.Header
func (bh BlockHeaders) Code() int { return 20 }

// HashOrNumber is a combined field for specifying an origin block.
type hashOrNumber struct {
	Hash   common.Hash // Block hash from which to retrieve headers (excludes Number)
	Number uint64      // Block hash from which to retrieve headers (excludes Hash)
}

// EncodeRLP is a specialized encoder for hashOrNumber to encode only one of the
// two contained union fields.
func (hn *hashOrNumber) EncodeRLP(w io.Writer) error {
	if hn.Hash == (common.Hash{}) {
		return rlp.Encode(w, hn.Number)
	}
	if hn.Number != 0 {
		return fmt.Errorf("both origin hash (%x) and number (%d) provided", hn.Hash, hn.Number)
	}
	return rlp.Encode(w, hn.Hash)
}

// DecodeRLP is a specialized decoder for hashOrNumber to decode the contents
// into either a block hash or a block number.
func (hn *hashOrNumber) DecodeRLP(s *rlp.Stream) error {
	_, size, _ := s.Kind()
	origin, err := s.Raw()
	if err == nil {
		switch {
		case size == 32:
			err = rlp.DecodeBytes(origin, &hn.Hash)
		case size <= 8:
			err = rlp.DecodeBytes(origin, &hn.Number)
		default:
			err = fmt.Errorf("invalid input size %d for origin", size)
		}
	}
	return err
}

// GetBlockBodies represents a GetBlockBodies request
type GetBlockBodies []common.Hash
func (gbb GetBlockBodies) Code() int { return 21 }

// BlockBodies is the network packet for block content distribution.
type BlockBodies []*types.Body
func (bb BlockBodies) Code() int { return 22 }

