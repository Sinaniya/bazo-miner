package p2p

import (
	"github.com/bazo-blockchain/bazo-miner/storage"
	"time"
)


var (
	sendingMap  map[string]*delayedMessagesPerSender
)

type delayedMessagesPerSender struct {
	peer *peer
	delayedMessages [][]byte
}

//This is not accessed concurrently, one single goroutine. However, the "peers" are accessed concurrently, therefore the
//Thread-safe implementation.
func peerService() {
	for {
		select {
		case p := <-register:
			peers.add(p)
		case p := <-disconnect:
			peers.closeChannelMutex.Lock()
			peers.delete(p)
			close(p.ch)
			peers.closeChannelMutex.Unlock()
		}
	}
}

func minerBroadcastService() {
	sendingMap = map[string]*delayedMessagesPerSender{}

	//For miner connections a map is created where all connections are stored based on the IP and Port of the peer.
	// In the first iteration this map is initalized.


	//Broadcasting all messages.
//	for msg := range minerBrdcstMsg {
//		logger.Printf("Inside minerBroadcastservice (1) len(minerBrdcstMsg) %v", len(minerBrdcstMsg))
//	//	for p := range peers.minerConns {
//	//		//Check if a connection was already established once. If so, nothing happens.
//	//		alreadyInSenderMap, needsUpdate := isConnectionAlreadyInSendingMap(p, sendingMap)
//	//		//logger.Printf("Inside Validation for block --> Inside Broadcastservice (2)")
//	//		if !alreadyInSenderMap && !needsUpdate {
//	//			//logger.Printf("Inside Validation for block --> Inside Broadcastservice (3)")
//	//			logger.Printf("create sending map for %v", p.getIPPort())
//	//			sendingMap[p.getIPPort()] = &delayedMessagesPerSender{p, nil}
//	//		}
//	//	}
//		//logger.Printf("Inside Validation for block --> Inside Broadcastservice (3)")
//		sendAndSearchMessages(msg)
//	}
//	logger.Printf("Inside minerBroadcastservice (2) len(minerBrdcstMsg) %v", len(minerBrdcstMsg))

	for {
		select {
		case msg := <-minerBrdcstMsg:
			if len(minerBrdcstMsg) > 0 {
				logger.Printf("Inside MinerBrdCst: len(minerBrdcstMsg) = %v", len(minerBrdcstMsg))
			}
			go sendAndSearchMessages(msg)
		}
	}

}

func clientBroadcastService() {

	for {
		select {
		case msg := <-clientBrdcstMsg:
			for p := range peers.clientConns {
				if peers.contains(p.getIPPort(),PEERTYPE_CLIENT) {
					p.ch <- msg
				} else {
					logger.Printf("CHANNEL_CLIENT: Wanted to send to %v, but %v is not in the peers.minerConns anymore", p.getIPPort(), p.getIPPort())
				}
			}
		}
	}
}

//This function does send the current and possible previous not send messages
func sendAndSearchMessages(msg []byte) {
	//logger.Printf("Inside Validation for block --> Inside SendAndSearchMessages (1)")
	for _, p := range sendingMap {
		//Check if there is a valid connection to peer p, if not, store message
		//if peers.minerConns[p.peer] {
		if peers.contains(p.peer.getIPPort(), PEERTYPE_MINER) {

			//If connection is valid, send message.
			//This is used to get the newest channel for given IP+Port. In case of an update in the background
			//logger.Printf("Inside Validation for block --> Inside SendAndSearchMessages (2)")
			peers.closeChannelMutex.Lock()
			//logger.Printf("Inside Validation for block --> Inside SendAndSearchMessages (3)")
			_, _ = isConnectionAlreadyInSendingMap(p.peer, sendingMap)
			receiver := sendingMap[p.peer.getIPPort()].peer
			if len(receiver.ch) > 0 {
				logger.Printf("Inside Sendand Search to %v -->  len(receiver.ch) = %v", receiver.getIPPort(), len(receiver.ch))
			}
			receiver.ch <- msg
			//logger.Printf("Inside Validation for block --> Inside SendAndSearchMessages (4) --> Sent")
			//logger.Printf("Inside Validation for block --> Inside SendAndSearchMessages (5)")

			//Send previously stored messages for this miner as well.
			for _, hMsg := range p.delayedMessages {
					//Send historic not yet sent transaction and remove it.

					//This is used to get the newest channel for given IP+Port. In case of an update in the background
					//logger.Printf("Inside Validation for block --> Inside SendAndSearchMessages (6)")
					//logger.Printf("Inside Validation for block --> Inside SendAndSearchMessages (7)")
					if len(receiver.ch) > 0 {
						logger.Printf("Inside Sendand Search With delay to %v -->  len(receiver.ch) = %v", receiver.getIPPort(), len(receiver.ch))
					}
					receiver.ch <- hMsg

					//logger.Printf("Inside Validation for block --> Inside SendAndSearchMessages (8) len(receiver.ch) %v", len(receiver.ch))

				p.delayedMessages = p.delayedMessages[1:]
			}
			peers.closeChannelMutex.Unlock()
		} else {
			//Store messages which are not sent du to connectivity issues.
			//logger.Printf("Inside Validation for block --> Inside SendAndSearchMessages (9)")
			messages := p.delayedMessages
			////Check that not too many delayed messages are stored.
			if len(messages) > 40 {
				messages = messages[1:]
			}
			//logger.Printf("Inside Validation for block --> Inside SendAndSearchMessages (10)")
			//Store message for this specific miner connection.
			p.delayedMessages = append(messages, msg)
			//logger.Printf("Inside Validation for block --> Inside SendAndSearchMessages (11)")
		}
	}
}

//This function checks if a connection was already established once and if the peer "behind" the IP + Port changed.
// This can happen all time when new connecting, because e.g a new channel (p.ch) is set up once adding a new peer
// (even if it was added before). If the peer changes as well, it gets updated in teh sendingMap.
func isConnectionAlreadyInSendingMap(p *peer, sendingMap map[string]*delayedMessagesPerSender) (alreadyInSenderMap bool, needsUpdate bool) {
	//logger.Printf("Inside Validation for block --> Inside Broadcastservice (1.1)")
	for _, connection := range sendingMap {
		if connection.peer.getIPPort() == p.getIPPort() {
			if connection.peer != p {
				sendingMap[p.getIPPort()] = &delayedMessagesPerSender{p, connection.delayedMessages}
				//logger.Printf("Inside Validation for block --> Inside Broadcastservice (1.2)")
				return true, true
			} else {
				//logger.Printf("Inside Validation for block --> Inside Broadcastservice (1.3)")
				return true, false
			}
		}
	}
	//logger.Printf("Inside Validation for block --> Inside Broadcastservice (1.4)")
	return false, false
}

//Belongs to the broadcast service.
func peerBroadcast(p *peer) {
	logger.Printf("CreatedPeerbroadcast for %v", p.getIPPort())

	for msg := range p.ch {
		sendData(p, msg)
	}
}

//Single goroutine that makes sure the system is well connected.
func checkHealthService() {
	for {
		//time.Sleep(HEALTH_CHECK_INTERVAL * time.Second)  Between 5 and 30 seconds check interval.
		var nrOfMiners = 1
		knownConnections := peers.getAllPeers(PEERTYPE_MINER)
		if len(knownConnections) > 1 {
			nrOfMiners = len(knownConnections)
		}
		if len(knownConnections) > 6 {
			nrOfMiners = 6
		}

		time.Sleep(time.Duration(nrOfMiners) * 5 * time.Second)  //Dynamic searching for neighbours interval --> 5 times the number of miners

		if Ipport != storage.Bootstrap_Server && !peers.contains(storage.Bootstrap_Server, PEERTYPE_MINER) {
			p, err := initiateNewMinerConnection(storage.Bootstrap_Server)
			if p == nil || err != nil {
				selfConnect := "Cannot self-connect"
				if err.Error()[0:9] != selfConnect[0:9] {
					logger.Printf("Initiating new miner connection failed: %v", err)
				}
			} else {
				go peerConn(p)
			}
		}

		//Periodically check if we are well-connected
		if peers.len(PEERTYPE_MINER) >= MIN_MINERS {
			continue
		}

		//The only goto in the code (I promise), but best solution here IMHO.
	RETRY:
		select {
		//iplistChan gets filled with every incoming neighborRes, they're consumed here.
		case ipaddr := <-iplistChan:
			if !peerExists(ipaddr) && !peerSelfConn(ipaddr) {

				p, err := initiateNewMinerConnection(ipaddr)
				if err != nil {
					logger.Printf("Initiating new miner connection failed: %v", err)
				}
				if p == nil || err != nil {
					goto RETRY
				}
				go peerConn(p)
				break
			}
		default:
			//In case we don't have any ip addresses in the channel left, make a request to the network.
			PrintMinerCons()
			neighborReq()
			logger.Printf("    |-- Request Neighbors...        |\n                                                      |_______________________________|")
			break
		}
	}
}

//Calculates periodically system time from available sources and broadcasts the time to all connected peers.
func timeService() {
	//Initialize system time.
	systemTime = time.Now().Unix()
	go func() {
		for {
			time.Sleep(UPDATE_SYS_TIME * time.Second)
			writeSystemTime()
		}
	}()

	for {
		time.Sleep(TIME_BRDCST_INTERVAL * time.Second)
		packet := BuildPacket(TIME_BRDCST, getTime())
		minerBrdcstMsg <- packet
	}
}
