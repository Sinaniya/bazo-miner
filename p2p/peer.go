package p2p

import (
	"math/rand"
	"net"
	"strings"
	"sync"
)

const (
	PEERTYPE_MINER  = 1
	PEERTYPE_CLIENT = 2
)

//The reason we use an additional listener port is because the port the miner connected to this peer
//is not the same as the one it listens to for new connections. When we are queried for neighbors
//we send the IP address in p.conn.RemotAddr() with the listenerPort.
type peer struct {
	conn         net.Conn
	ch           chan []byte
	l            sync.Mutex
	listenerPort string
	time         int64
	peerType     uint
}

//Block constructor, argument is the previous block in the blockchain.
func newPeer(conn net.Conn, listenerPort string, peerType uint) *peer {
	p := new(peer)
	p.conn = conn
	p.ch = nil
	p.l = sync.Mutex{}
	p.listenerPort = listenerPort
	p.time = 0
	p.peerType = peerType

	return p
}

//PeerStruct is a thread-safe map that supports all necessary map operations needed by the server.
type peersStruct struct {
	minerConns  map[*peer]bool
	clientConns map[*peer]bool
	peerMutex   sync.Mutex
	closeChannelMutex sync.Mutex
}

func (peers peersStruct) contains(ipport string, peerType uint) bool {
	peers.closeChannelMutex.Lock()
	defer peers.closeChannelMutex.Unlock()

	var peerConns map[*peer]bool

	if peerType == PEERTYPE_MINER {
		peerConns = peers.minerConns
	}
	if peerType == PEERTYPE_CLIENT {
		peerConns = peers.clientConns
	}

	for peer := range peerConns {
		if peer.getIPPort() == ipport {
			return true
		}
	}

	return false
}

func (p *peer) getIPPort() string {
	ip := strings.Split(p.conn.RemoteAddr().String(), ":")
	//Cut off original port.
	port := p.listenerPort

	return ip[0] + ":" + port
}

func (peers peersStruct) add(p *peer) {
	peers.peerMutex.Lock()
	defer peers.peerMutex.Unlock()

	if p.peerType == PEERTYPE_MINER {
		for peer := range peers.minerConns {
			if p.getIPPort() == peer.getIPPort() {
				delete(peers.minerConns, peer)
				logger.Printf("Deleted Old Peer Entry for %v", peer.getIPPort())
			}
		}
		peers.minerConns[p] = true
	}
	if p.peerType == PEERTYPE_CLIENT {
		peers.clientConns[p] = true
	}

	for p := range peers.minerConns {
		//Check if a connection was already established once. If so, nothing happens.
		alreadyInSenderMap, needsUpdate := isConnectionAlreadyInSendingMap(p, sendingMap)
		//logger.Printf("Inside Validation for block --> Inside Broadcastservice (2)")
		if !alreadyInSenderMap && !needsUpdate {
			//logger.Printf("Inside Validation for block --> Inside Broadcastservice (3)")
			logger.Printf("Create sending map for %v", p.getIPPort())
			sendingMap[p.getIPPort()] = &delayedMessagesPerSender{p, nil}
		}
	}
}

func (peers peersStruct) delete(p *peer) {
	peers.peerMutex.Lock()
	defer peers.peerMutex.Unlock()

	if p.peerType == PEERTYPE_MINER {
		delete(peers.minerConns, p)
	}
	if p.peerType == PEERTYPE_CLIENT {
		delete(peers.clientConns, p)
	}
}

func (peers peersStruct) len(peerType uint) (length int) {
	peers.peerMutex.Lock()
	defer peers.peerMutex.Unlock()
	if peerType == PEERTYPE_MINER {
		length = len(peers.minerConns)
	}
	if peerType == PEERTYPE_CLIENT {
		length = len(peers.clientConns)
	}

	return length
}

func (peers peersStruct) getRandomPeer(peerType uint) (p *peer) {
	//Acquire list before locking, otherwise deadlock
	peerList := peers.getAllPeers(peerType)

	if len(peerList) == 0 {
		return nil
	} else {
		return peerList[int(rand.Uint32())%len(peerList)]
	}
}

func (peers peersStruct) getAllPeers(peerType uint) []*peer {
	peers.peerMutex.Lock()
	defer peers.peerMutex.Unlock()

	var peerList []*peer

	if peerType == PEERTYPE_MINER {
		for p := range peers.minerConns {
			peerList = append(peerList, p)
		}
	}
	if peerType == PEERTYPE_CLIENT {
		for p := range peers.clientConns {
			peerList = append(peerList, p)
		}
	}
	return peerList
}

func (peers peersStruct) getMinerTimes() (peerTimes []int64) {
	peers.peerMutex.Lock()
	defer peers.peerMutex.Unlock()

	for p := range peers.minerConns {
		p.l.Lock()
		peerTimes = append(peerTimes, p.time)
		//Concurrent writes need to protected. We set the time to 0 again as an indicator that the value has been consumed.
		p.time = 0
		p.l.Unlock()
	}

	return peerTimes
}
