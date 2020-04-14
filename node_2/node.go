package node_2

import (
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/internal/debug"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/prometheus/tsdb/fileutil"
	"reflect"
	"sync"
)

type Node struct {
	eventmux *event.TypeMux // Event multiplexer used between the services of a stack
	config   *Config
	accman   *accounts.Manager

	ephemeralKeystore string            // if non-empty, the key directory that will be removed by Stop
	instanceDirLock   fileutil.Releaser // prevents concurrent use of instance directory

	p2pServer    *p2p.Server // Currently running P2P networking layer

	services map[reflect.Type]Service // Currently running services
	auxServices map[reflect.Type]AuxiliaryService // Currently running auxiliary services

	rpcAPIs []rpc.API // TODO still need to figure out whether this is necessary

	inprocHandler *rpc.Server

	ipcHandler *httpHandler
	httpHandler *httpHandler
	wsHandler *httpHandler

	stop chan struct{}
	lock sync.RWMutex

	log log.Logger
}

func (n *Node) Start() error {
	n.lock.Lock()

	//TODO

	return nil
}

func (n *Node) defaultAPIs() []rpc.API {
	return []rpc.API{
		{
			Namespace: "admin",
			Version:   "1.0",
			Service:   NewPrivateAdminAPI(n),
		}, {
			Namespace: "admin",
			Version:   "1.0",
			Service:   NewPublicAdminAPI(n),
			Public:    true,
		}, {
			Namespace: "debug",
			Version:   "1.0",
			Service:   debug.Handler,
		}, {
			Namespace: "web3",
			Version:   "1.0",
			Service:   NewPublicWeb3API(n),
			Public:    true,
		},
	}
}
