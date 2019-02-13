package p2p

import (
	"github.com/bazo-blockchain/bazo-miner/protocol"
)

var (
	//Block from the network, to the miner
	BlockIn chan []byte = make(chan []byte)
	//Block from the miner, to the network
	BlockOut       chan []byte = make(chan []byte)
	//BlockHeader from the miner, to the clients
	BlockHeaderOut chan []byte = make(chan []byte)

	VerifiedTxsOut chan []byte = make(chan []byte)

	//Data requested by miner, to allow parallelism, we have a chan for every tx type.
	FundsTxChan  = make(chan *protocol.FundsTx)
	AccTxChan    = make(chan *protocol.AccTx)
	ConfigTxChan = make(chan *protocol.ConfigTx)
	StakeTxChan  = make(chan *protocol.StakeTx)
	AggTxChan    = make(chan *protocol.AggSenderTx)

	BlockReqChan = make(chan []byte)

	receivedTXStash = make([]*protocol.FundsTx, 0)
)

//This is for blocks and txs that the miner successfully validated.
func forwardBlockBrdcstToMiner() {
	for {
		block := <-BlockOut
		toBrdcst := BuildPacket(BLOCK_BRDCST, block)
		minerBrdcstMsg <- toBrdcst
	}
}

func forwardBlockHeaderBrdcstToMiner() {
	for {
		blockHeader := <- BlockHeaderOut
		clientBrdcstMsg <- BuildPacket(BLOCK_HEADER_BRDCST, blockHeader)
	}
}

func forwardVerifiedTxsToMiner() {
	for {
		verifiedTxs := <- VerifiedTxsOut
		clientBrdcstMsg <- BuildPacket(VERIFIEDTX_BRDCST, verifiedTxs)
	}
}

func forwardBlockToMiner(p *peer, payload []byte) {
	BlockIn <- payload
}

//Checks if Tx Is in the received stash. If true, we received the transaction with a request already.
func txAlreadyInStash(slice []*protocol.FundsTx, newTXHash [32]byte) bool {
	for _, txInStash := range slice {
		if txInStash.Hash() == newTXHash {
			return true
		}
	}
	return false
}

//These are transactions the miner specifically requested.
func forwardTxReqToMiner(p *peer, payload []byte, txType uint8) {
	if payload == nil {
		return
	}

	switch txType {
	case FUNDSTX_RES:
		var fundsTx *protocol.FundsTx
		fundsTx = fundsTx.Decode(payload)
		if fundsTx == nil {
			return
		}
		// If TX is not received with the last 1000 Transaction, send it through the channel to the TX_FETCH.
		// Otherwise send nothing. This means, that the TX was sent before and we ensure, that only one TX per Broadcast
		// request is going through to the FETCH Request. This should prevent the "Received txHash did not correspond to
		// our request." error
		if !txAlreadyInStash(receivedTXStash, fundsTx.Hash()) {
			receivedTXStash = append(receivedTXStash, fundsTx)
			FundsTxChan <- fundsTx
			if len(receivedTXStash) > 1000 {
				receivedTXStash = append(receivedTXStash[:0], receivedTXStash[1:]...)
			}
		}
	case ACCTX_RES:
		var accTx *protocol.AccTx
		accTx = accTx.Decode(payload)
		if accTx == nil {
			return
		}
		AccTxChan <- accTx
	case CONFIGTX_RES:
		var configTx *protocol.ConfigTx
		configTx = configTx.Decode(payload)
		if configTx == nil {
			return
		}
		ConfigTxChan <- configTx
	case STAKETX_RES:
		var stakeTx *protocol.StakeTx
		stakeTx = stakeTx.Decode(payload)
		if stakeTx == nil {
			return
		}
		StakeTxChan <- stakeTx
	case AGGTX_RES:
		var aggTx *protocol.AggSenderTx
		aggTx = aggTx.Decode(payload)
		if aggTx == nil {
			return
		}
		logger.Printf("Received & Forwarded: %x,\n%v", aggTx.Hash(), aggTx)
		AggTxChan <- aggTx
	}
}

func forwardBlockReqToMiner(p *peer, payload []byte) {
	BlockReqChan <- payload
}

func ReadSystemTime() int64 {
	return systemTime
}
