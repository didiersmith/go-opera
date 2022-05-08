package gossip

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"math"
	"math/big"
	"math/rand"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Fantom-foundation/go-opera/contracts/fish3_lite"
	"github.com/Fantom-foundation/go-opera/contracts/uniswap_pair_lite"
	"github.com/Fantom-foundation/go-opera/evmcore"
	"github.com/Fantom-foundation/go-opera/inter"
	"github.com/Fantom-foundation/go-opera/inter/iblockproc"
	"github.com/Fantom-foundation/go-opera/opera"
	"github.com/Fantom-foundation/go-opera/utils"
	"github.com/Fantom-foundation/go-opera/utils/gsignercache"
	"github.com/Fantom-foundation/lachesis-base/common/bigendian"
	"github.com/Fantom-foundation/lachesis-base/hash"
	"github.com/Fantom-foundation/lachesis-base/inter/idx"
	"github.com/Fantom-foundation/lachesis-base/inter/pos"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	notify "github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
)

const (
	GAS_INITIAL = 30000
	// GAS_INITIAL  = 3000000
	GAS_TRANSFER = 27000
	GAS_SWAP     = 80000
	// GAS_INITIAL  = 0
	// GAS_TRANSFER = 0
	// GAS_SWAP     = 0
	GAS_FAIL = 140000
)

var (
	nullAddr           = common.HexToAddress("0x0000000000000000000000000000000000000000")
	syncEventTopic     = []byte{0x1c, 0x41, 0x1e, 0x9a, 0x96, 0xe0, 0x71, 0x24, 0x1c, 0x2f, 0x21, 0xf7, 0x72, 0x6b, 0x17, 0xae, 0x89, 0xe3, 0xca, 0xb4, 0xc7, 0x8b, 0xe5, 0x0e, 0x06, 0x2b, 0x03, 0xa9, 0xff, 0xfb, 0xba, 0xd1}
	pairGetReservesAbi = uniswap_pair_lite.GetReserves()
	pairToken0Abi      = uniswap_pair_lite.Token0()
	pairToken1Abi      = uniswap_pair_lite.Token1()
	startTokensIn      = new(big.Int).Exp(big.NewInt(10), big.NewInt(19), nil)
	maxAmountIn        = new(big.Int).Mul(big.NewInt(1825), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	exampleKey         = pairKeyFromStrs("0x21be370d5312f44cb42ce377bc9b8a0cef1a4c83", "0xd67de0e0a0fd7b15dc8348bb9be742f3c5850454", "0xF3369Bd152780B01A4F9Add38150BEEFf924DaA7")
	fishAddr           = common.HexToAddress("0xba164fB7530b24cF73d183ce0140AF9Ab8C35Cd8")
)

type PairKey [60]byte

type Dexter struct {
	svc                 *Service
	inTxChan            chan *types.Transaction
	inLogsChan          chan []*types.Log
	inLogsSub           notify.Subscription
	inBlockChan         chan evmcore.ChainHeadNotify
	inBlockSub          notify.Subscription
	inEpochChan         chan idx.Epoch
	inEpochSub          notify.Subscription
	inEventChan         chan *inter.EventPayload
	inEventSub          notify.Subscription
	pairsInfo           map[common.Address]*PairInfo
	lag                 time.Duration
	routeCache          RouteCache
	gunRefreshes        chan map[idx.ValidatorID][]accounts.Wallet
	signer              types.Signer
	guns                map[idx.ValidatorID][]accounts.Wallet
	watchedTxs          chan *TxSub
	ignoreTxs           map[common.Hash]struct{}
	clearIgnoreTxChan   chan common.Hash
	eventRaceChan       chan *RaceEntry
	txRaceChan          chan *RaceEntry
	pairsInfoUpdateChan chan *PairsInfoUpdate
	txLagRequestChan    chan common.Hash
	mu                  sync.RWMutex
}

type RaceEntry struct {
	PeerID string
	Hash   common.Hash
	T      time.Time
	Full   bool
}

type TxSub struct {
	Hash                common.Hash
	Label               string
	PredictedValidators []idx.ValidatorID
}

type PairInfo struct {
	reserve0     *big.Int
	reserve1     *big.Int
	token0       common.Address
	token1       common.Address
	feeNumerator *big.Int
}

type RouteCache struct {
	Routes          [][]*Leg
	PairToRouteIdxs map[PairKey][]uint
	Scores          []uint64
}

type Leg struct {
	From     common.Address
	To       common.Address
	PairAddr common.Address
}

type PairInfoJson struct {
	Addr         string `json:addr`
	Token0       string `json:token0`
	Token1       string `json:token1`
	ExchangeName string `json:exchangeName`
	FeeNumerator int64  `json:feeNumerator`
}

type LegJson struct {
	From     string `json:from`
	To       string `json:to`
	PairAddr string `json:pairAddr`
}

type RouteCacheJson struct {
	Routes          [][]LegJson       `json:routes`
	PairToRouteIdxs map[string][]uint `json:pairToRouteIdxs`
}

type Plan struct {
	AmountIn *big.Int
	GasPrice *big.Int
	GasCost  *big.Int
	Path     []fish3_lite.SwapCommand
}

// TODO  compute PairToRouteIdxs updates in background and send something like this to main goroutine

type PairsInfoUpdate struct {
	PairsInfoUpdates map[common.Address]*PairInfo
	PairToRouteIdxs  map[PairKey][]uint
	Scores           []uint64
}

func NewDexter(svc *Service) *Dexter {
	d := &Dexter{
		svc:                 svc,
		inTxChan:            make(chan *types.Transaction, 256),
		inLogsChan:          make(chan []*types.Log, 256),
		inBlockChan:         make(chan evmcore.ChainHeadNotify, 256),
		inEpochChan:         make(chan idx.Epoch, 256),
		inEventChan:         make(chan *inter.EventPayload, 256),
		pairsInfo:           make(map[common.Address]*PairInfo),
		lag:                 time.Minute * 60,
		signer:              gsignercache.Wrap(types.LatestSignerForChainID(svc.store.GetRules().EvmChainConfig().ChainID)),
		gunRefreshes:        make(chan map[idx.ValidatorID][]accounts.Wallet),
		watchedTxs:          make(chan *TxSub),
		ignoreTxs:           make(map[common.Hash]struct{}),
		clearIgnoreTxChan:   make(chan common.Hash, 16),
		eventRaceChan:       make(chan *RaceEntry, 4096),
		txRaceChan:          make(chan *RaceEntry, 4096),
		pairsInfoUpdateChan: make(chan *PairsInfoUpdate),
		txLagRequestChan:    make(chan common.Hash, 16),
	}
	d.inLogsSub = svc.feed.SubscribeNewLogs(d.inLogsChan)
	d.inBlockSub = svc.feed.SubscribeNewBlock(d.inBlockChan)
	d.inEpochSub = svc.feed.SubscribeNewEpoch(d.inEpochChan)
	svc.handler.SubscribeEvents(d.inEventChan)
	d.loadJson()
	for pairAddr, pairInfo := range d.pairsInfo {
		reserve0, reserve1 := d.getReserves(&pairAddr)
		token0, token1 := d.getPairTokens(&pairAddr)
		d.pairsInfo[pairAddr] = &PairInfo{
			reserve0:     reserve0,
			reserve1:     reserve1,
			token0:       token0,
			token1:       token1,
			feeNumerator: pairInfo.feeNumerator,
		}
	}
	d.refreshScores()
	d.routeCache.PairToRouteIdxs = d.sortPairToRouteIdxMap(d.routeCache.Scores)
	go d.processIncomingLogs()
	go d.processIncomingTxs()
	go d.watchEvents()
	go d.eventRace()
	// svc.handler.AttachDexter(d.inTxChan)
	svc.handler.RaceEvents(d.eventRaceChan)
	svc.handler.RaceTxs(d.txRaceChan)
	return d
}

func (d *Dexter) optimizePeers() {
	for {
		time.Sleep(10 * time.Second)
		// localNode := d.svc.p2pServer.LocalNode()
		// nodes := d.svc.p2pServer.DiscV5.AllNodes()
		// for i, node := range nodes {
		// 	log.Info("Node", "i", i, "IP", node.IP(), "node", node.ID(), "local id", localNode.ID(), "distance", enode.LogDist(localNode.ID(), node.ID()))
		// }
		peers := d.svc.p2pServer.Peers()
		log.Info("Peers connected", "len", len(peers))
		// for i, peer := range peers {
		// 	log.Info("Peer", "i", i, "ID", peer.Node().ID(), "IP", peer.Node().IP(), "distance", enode.LogDist(localNode.ID(), peer.Node().ID()))
		// }
	}
}

type Pair struct {
	Key   string
	Value int
}
type PairList []Pair

func (p PairList) Len() int           { return len(p) }
func (p PairList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p PairList) Less(i, j int) bool { return p[i].Value > p[j].Value }

func mapToSortedPairs(m map[string]int) PairList {
	pairs := make(PairList, len(m))
	i := 0
	for k, v := range m {
		pairs[i] = Pair{k, v}
		i++
	}
	sort.Sort(pairs)
	return pairs
}

type BytePair struct {
	Key   [4]byte
	Value int
}
type BytePairList []BytePair

func (p BytePairList) Len() int           { return len(p) }
func (p BytePairList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p BytePairList) Less(i, j int) bool { return p[i].Value > p[j].Value }

func mapToSortedBytePairs(m map[[4]byte]int) BytePairList {
	pairs := make(BytePairList, len(m))
	i := 0
	for k, v := range m {
		pairs[i] = BytePair{k, v}
		i++
	}
	sort.Sort(pairs)
	return pairs
}

func (d *Dexter) eventRace() {
	eventCapacity := 1000
	seenEvents := make(map[common.Hash]struct{}, eventCapacity)
	eventWins := make(map[string]int)

	txCapacity := 10000
	seenTxs := make(map[common.Hash]*RaceEntry, txCapacity)
	txWins := make(map[string]int)

	minLifetime := 5 * 60 * time.Second
	numSortedTxPeers := 8
	prevCullTime := time.Now()
	for {
		select {
		case e := <-d.eventRaceChan:
			if _, ok := seenEvents[e.Hash]; !ok {
				seenEvents[e.Hash] = struct{}{}
				if wins, ok := eventWins[e.PeerID]; ok {
					eventWins[e.PeerID] = wins + 1
				} else {
					eventWins[e.PeerID] = 1
				}
			}
			if len(seenEvents) >= eventCapacity {
				// log.Info("Resetting seen events")
				seenEvents = make(map[common.Hash]struct{}, eventCapacity)
			}
		case e := <-d.txRaceChan:
			if _, ok := seenTxs[e.Hash]; !ok {
				seenTxs[e.Hash] = e
				if wins, ok := txWins[e.PeerID]; ok {
					txWins[e.PeerID] = wins + 1
				} else {
					txWins[e.PeerID] = 1
				}
			}
			if len(seenTxs) >= txCapacity {
				// log.Info("Resetting seen txs")
				seenTxs = make(map[common.Hash]*RaceEntry, txCapacity)
			}
		case h := <-d.txLagRequestChan:
			if entry, ok := seenTxs[h]; ok {
				log.Info("Lag since first seen hash", "hash", h.Hex(), "lag", utils.PrettyDuration(time.Now().Sub(entry.T)), "full", entry.Full)
			}
		}
		now := time.Now()
		if now.Sub(prevCullTime) > minLifetime {
			// log.Info("Calculating winners")

			// txWinPairs := mapToSortedPairs(txWins)
			// for i, winPair := range txWinPairs {
			// 	log.Info("tx wins", "i", i, "peer", winPair.Key, "wins", winPair.Value)
			// }

			var losers []string
			peers := d.svc.handler.peers.List()
			for _, peer := range peers {
				if now.Sub(peer.created) < minLifetime {
					continue // Give the noob a chance
				}
				_, wonEvent := eventWins[peer.id]
				txWins, wonTx := txWins[peer.id]
				if !wonEvent && (!wonTx || txWins < 3) {
					losers = append(losers, peer.id)
				}
			}
			sortedPeers := make([]string, 0, len(peers)-len(losers))
			eventWinPairs := mapToSortedPairs(eventWins)
			for _, winPair := range eventWinPairs {
				sortedPeers = append(sortedPeers, winPair.Key)
			}
			sortedTxPeers := make([]string, 0, numSortedTxPeers)
			txWinPairs := mapToSortedPairs(txWins)
			for _, winPair := range txWinPairs {
				sortedTxPeers = append(sortedTxPeers, winPair.Key)
				if len(sortedTxPeers) == numSortedTxPeers {
					break
				}
			}
			sortedPeers = append(sortedPeers, losers...)
			log.Info("Detected winners and losers", "peers", len(peers), "eventWins", len(eventWins), "txWins", len(txWins), "losers", len(losers), "sorted", len(sortedPeers), "sortedTxPeers", len(sortedTxPeers))
			d.svc.handler.peers.Cull(losers)
			d.svc.handler.peers.SetSortedPeers(sortedPeers)
			d.svc.handler.peers.SetSortedTxPeers(sortedTxPeers)

			eventWins = make(map[string]int)
			txWins = make(map[string]int)
			prevCullTime = time.Now()
		}
	}
}

func (d *Dexter) loadJson() {
	root := "/home/ubuntu/dexter/carb/"

	log.Info("Loading routes")
	routeCacheRoutesFileName := root + "route_cache_routes_10ftm.json"
	routeCacheRoutesFile, err := os.Open(routeCacheRoutesFileName)
	if err != nil {
		log.Info("Error opening routeCacheRoutes", "routeCacheRoutesFileName", routeCacheRoutesFileName, "err", err)
		return
	}
	defer routeCacheRoutesFile.Close()
	routeCacheRoutesBytes, _ := ioutil.ReadAll(routeCacheRoutesFile)
	var routeCacheJson RouteCacheJson
	json.Unmarshal(routeCacheRoutesBytes, &(routeCacheJson.Routes))
	log.Info("Loaded routes")

	routeCachePairToRouteIdxsFileName := root + "route_cache_pairToRouteIdxs_10ftm.json"
	routeCachePairToRouteIdxsFile, err := os.Open(routeCachePairToRouteIdxsFileName)
	if err != nil {
		log.Info("Error opening routeCachePairToRouteIdxs", "routeCachePairToRouteIdxsFileName", routeCachePairToRouteIdxsFileName, "err", err)
		return
	}
	defer routeCachePairToRouteIdxsFile.Close()
	routeCachePairToRouteIdxsBytes, _ := ioutil.ReadAll(routeCachePairToRouteIdxsFile)
	json.Unmarshal(routeCachePairToRouteIdxsBytes, &(routeCacheJson.PairToRouteIdxs))
	log.Info("Loaded pairToRouteIdxs")

	routeCache := RouteCache{
		Routes:          make([][]*Leg, len(routeCacheJson.Routes)),
		PairToRouteIdxs: make(map[PairKey][]uint),
		Scores:          make([]uint64, len(routeCacheJson.Routes)),
	}
	pairAddrs := make(map[common.Address]struct{})
	for i, routeJson := range routeCacheJson.Routes {
		route := make([]*Leg, len(routeJson))
		for x, leg := range routeJson {
			pairAddr := common.HexToAddress(leg.PairAddr)
			if _, ok := pairAddrs[pairAddr]; !ok {
				pairAddrs[pairAddr] = struct{}{}
			}
			route[x] = &Leg{
				From:     common.HexToAddress(leg.From),
				To:       common.HexToAddress(leg.To),
				PairAddr: pairAddr,
			}
		}
		routeCache.Routes[i] = route
	}
	for strKey, routeIdxs := range routeCacheJson.PairToRouteIdxs {
		parts := strings.Split(strKey, "_")
		key := pairKeyFromStrs(parts[0], parts[1], parts[2])
		routeCache.PairToRouteIdxs[key] = routeIdxs
	}
	log.Info("Processed route cache")
	d.routeCache = routeCache
	pairsFileName := root + "pairs.json"
	pairsFile, err := os.Open(pairsFileName)
	if err != nil {
		log.Info("Error opening pairs", "pairsFileName", pairsFileName, "err", err)
		return
	}
	defer pairsFile.Close()
	pairsBytes, _ := ioutil.ReadAll(pairsFile)
	var jsonPairs []PairInfoJson
	json.Unmarshal(pairsBytes, &jsonPairs)
	for _, jsonPair := range jsonPairs {
		pairAddr := common.HexToAddress(jsonPair.Addr)
		if _, ok := pairAddrs[pairAddr]; ok {
			d.pairsInfo[pairAddr] = &PairInfo{
				feeNumerator: big.NewInt(jsonPair.FeeNumerator),
			}
		}
	}
}

func (d *Dexter) printPairsInfo() {
	for pairAddr, pairInfo := range d.pairsInfo {
		log.Info("Pair", "pairAddr", pairAddr, "reserve0", pairInfo.reserve0, "reserve1", pairInfo.reserve1, "info", pairInfo)
	}
	log.Info("Total pairs", "len", len(d.pairsInfo))
	// exampleRoutes := d.routeCache.PairToRouteIdxs[exampleKey]
	// for _, routeIdx := range exampleRoutes {
	// 	log.Info("Route", "routeIdx", routeIdx, "score", d.routeCache.Scores[routeIdx], "path", d.routeCache.Routes[routeIdx])
	// }
}

func (d *Dexter) printAccounts() {
	for w, wallet := range d.svc.accountManager.Wallets() {
		wStatus, err := wallet.Status()
		log.Info("Wallet", "idx", w, "status", wStatus, "err", err)
		for a, account := range wallet.Accounts() {
			log.Info("Account", "idx", a, "addr", account.Address)
		}
	}
}

func (d *Dexter) predictValidators(address common.Address, nonce uint64, validators *pos.Validators, epoch idx.Epoch, num int) []idx.ValidatorID {
	roundsHash := hash.Of(address.Bytes(), bigendian.Uint64ToBytes(nonce/32), epoch.Bytes())
	rounds := utils.WeightedPermutation(num, validators.SortedWeights(), roundsHash)
	ret := make([]idx.ValidatorID, 0, num)
	for i := 0; i < num; i++ {
		ret = append(ret, validators.GetID(idx.Validator(rounds[i])))
	}
	return ret
}

func (d *Dexter) refreshGuns() {
	guns := make(map[idx.ValidatorID][]accounts.Wallet)
	validators, epoch := d.svc.store.GetEpochValidators()
	wallets := d.svc.accountManager.Wallets()
	offset := rand.Intn(len(wallets))
	for i := 0; i < len(wallets); i++ {
		wallet := wallets[(i+offset)%len(wallets)]
		wStatus, _ := wallet.Status()
		if wStatus == "Locked" {
			continue
		}
		account := wallet.Accounts()[0]
		nonce := d.svc.txpool.Nonce(account.Address)
		validatorIDs := d.predictValidators(account.Address, nonce, validators, epoch, 1)
		validatorID := validatorIDs[0]
		// log.Info("Wallet", "idx", w, "status", wStatus, "account", account.Address, "nonce", nonce, "validator", validatorID)
		if gunQ, ok := guns[validatorID]; ok {
			guns[validatorID] = append(gunQ, wallet)
		} else {
			guns[validatorID] = []accounts.Wallet{wallet}
		}
	}
	d.gunRefreshes <- guns
}

func (d *Dexter) processIncomingLogs() {
	log.Info("Started dexter")
	scores := make([]uint64, len(d.routeCache.Scores))
	copy(scores, d.routeCache.Scores)
	methods := make(map[[4]byte]int)
	methodUsers := make(map[[4]byte]*common.Address)
	numUpdates := 0
	for {
		select {
		case logs := <-d.inLogsChan:
			updated := false
			pairsInfoUpdates := make(map[common.Address]*PairInfo)
			for _, l := range logs {
				pairAddr, reserve0, reserve1 := getReservesFromSyncLog(l)
				if pairAddr == nil {
					continue
				}
				d.mu.RLock()
				pairInfo, ok := d.pairsInfo[*pairAddr]
				d.mu.RUnlock()
				if !ok {
					continue
				}
				// log.Info("Updated pairsInfo", "pairAddr", pairAddr, "reserve0", reserve0, "reserve1", reserve1)
				pairsInfoUpdates[*pairAddr] = &PairInfo{
					reserve0:     reserve0,
					reserve1:     reserve1,
					token0:       pairInfo.token0,
					token1:       pairInfo.token1,
					feeNumerator: pairInfo.feeNumerator,
				}
				key0 := pairKeyFromAddrs(pairInfo.token0, pairInfo.token1, *pairAddr)
				key1 := pairKeyFromAddrs(pairInfo.token1, pairInfo.token0, *pairAddr)
				d.refreshScoresForPair(key0, pairsInfoUpdates, scores) // Updates scores
				d.refreshScoresForPair(key1, pairsInfoUpdates, scores)
				updated = true
				block := d.svc.EthAPI.state.GetBlock(common.Hash{}, l.BlockNumber)
				if block == nil || uint(len(block.Transactions)) <= l.TxIndex {
					continue
				}
				// log.Info("Got block", "num", l.BlockNumber)
				tx := block.Transactions[l.TxIndex]
				data := tx.Data()
				var method [4]byte
				if len(data) >= 4 {
					copy(method[:], data[:4])
					if methodScore, ok := methods[method]; ok {
						methods[method] = methodScore + 1
					} else {
						methods[method] = 1
						methodUsers[method] = tx.To()
					}
				}
				// log.Info("TX that caused updated:", "to", tx.To(), "method", method, "tx", tx)
			}
			if updated {
				numUpdates++
				pairToRouteIdxs := d.sortPairToRouteIdxMap(scores)
				update := &PairsInfoUpdate{
					PairsInfoUpdates: pairsInfoUpdates,
					PairToRouteIdxs:  pairToRouteIdxs,
				}
				copy(update.Scores, scores)
				d.pairsInfoUpdateChan <- update
			}
			if numUpdates > 1000 {
				numUpdates = 0

				log.Info("1000 updates, dumping methods")
				methodPairs := mapToSortedBytePairs(methods)
				for _, p := range methodPairs {
					log.Info("Method score", "method", p.Key, "score", p.Value, "user", methodUsers[p.Key])
				}
				methods = make(map[[4]byte]int)
			}
		case <-d.inLogsSub.Err():
			return
		}
	}
	d.inLogsSub.Unsubscribe()
}

func (d *Dexter) processIncomingTxs() {
	log.Info("Started dexter")
	txCapacity := 1000
	seenTxs := make(map[common.Hash]struct{}, txCapacity)
	for {
		select {
		case tx := <-d.inTxChan:
			if d.lag > 4*time.Second {
				continue
			}
			if _, ok := d.ignoreTxs[tx.Hash()]; ok {
				continue
			}
			if _, ok := seenTxs[tx.Hash()]; ok {
				continue
			}
			if len(seenTxs) > txCapacity {
				seenTxs = make(map[common.Hash]struct{}, txCapacity)
			}
			seenTxs[tx.Hash()] = struct{}{}
			// log.Info("Dexter received tx", "hash", tx.Hash().Hex())
			d.processTx(tx)
		case update := <-d.pairsInfoUpdateChan:
			// log.Info("Received update", "pairs", len(update.PairsInfoUpdates), "PairToRouteIdxs", len(update.PairToRouteIdxs))
			d.mu.Lock()
			d.routeCache.Scores = update.Scores
			d.routeCache.PairToRouteIdxs = update.PairToRouteIdxs
			for pairAddr, pairInfo := range update.PairsInfoUpdates {
				d.pairsInfo[pairAddr] = pairInfo
			}
			d.mu.Unlock()

		case n := <-d.inBlockChan:
			d.lag = time.Now().Sub(n.Block.Time.Time())
			if d.lag < 2*time.Second {
				go d.refreshGuns()
			}
		case <-d.inBlockSub.Err():
			return
		case <-d.inEpochChan:
			go d.refreshGuns()
		case <-d.inEpochSub.Err():
			return
		case guns := <-d.gunRefreshes:
			// log.Info("Updated guns", "guns", guns)
			d.guns = guns
		case h := <-d.clearIgnoreTxChan:
			delete(d.ignoreTxs, h)
		}
	}
	d.inLogsSub.Unsubscribe()
}

func (d *Dexter) watchEvents() {
	watchedTxMap := make(map[common.Hash]*TxSub)
	for {
		select {
		case e := <-d.inEventChan:
			// log.Info("New event", "event", e, "creator", e.Locator().Creator, "txs", e.Txs())
			for _, tx := range e.Txs() {
				if sub, ok := watchedTxMap[tx.Hash()]; ok {
					log.Info("Watched event", "id", e.ID(), "lamport", e.Locator().Lamport, "creator", e.Locator().Creator, "predicted", sub.PredictedValidators, "tx", tx.Hash().Hex(), "label", sub.Label)
					delete(watchedTxMap, tx.Hash())
					d.clearIgnoreTxChan <- tx.Hash()
				}
			}
		case w := <-d.watchedTxs:
			watchedTxMap[w.Hash] = w
		}
	}
}

func (d *Dexter) getReserves(addr *common.Address) (*big.Int, *big.Int) {
	evm := d.getReadOnlyEvm()
	msg := d.readOnlyMessage(addr, pairGetReservesAbi)
	gp := new(evmcore.GasPool).AddGas(math.MaxUint64)
	result, err := evmcore.ApplyMessage(evm, msg, gp)
	if err != nil {
		log.Info("getReserves error", "err", err)
		return nil, nil
	}
	reserve0 := new(big.Int).SetBytes(result.ReturnData[:32])
	reserve1 := new(big.Int).SetBytes(result.ReturnData[32:64])
	if reserve0.BitLen() == 0 || reserve1.BitLen() == 0 {
		log.Info("WARNING: getReserves() returned 0", "addr", addr, "reserve0", reserve0, "reserve1", reserve1, "returnData", result.ReturnData)
	}
	return reserve0, reserve1
}

func (d *Dexter) getPairTokens(addr *common.Address) (common.Address, common.Address) {
	evm := d.getReadOnlyEvm()
	msg0 := d.readOnlyMessage(addr, pairToken0Abi)
	gp0 := new(evmcore.GasPool).AddGas(math.MaxUint64)
	result0, err := evmcore.ApplyMessage(evm, msg0, gp0)
	if err != nil {
		log.Info("token0 error", "err", err)
		return common.Address{}, common.Address{}
	}
	token0 := common.BytesToAddress(result0.ReturnData)
	msg1 := d.readOnlyMessage(addr, pairToken1Abi)
	gp1 := new(evmcore.GasPool).AddGas(math.MaxUint64)
	result1, err := evmcore.ApplyMessage(evm, msg1, gp1)
	if err != nil {
		log.Info("token1 error", "err", err)
		return common.Address{}, common.Address{}
	}
	token1 := common.BytesToAddress(result1.ReturnData)
	return token0, token1
}

func (d *Dexter) readOnlyMessage(toAddr *common.Address, data []byte) types.Message {
	var accessList types.AccessList
	return types.NewMessage(nullAddr, toAddr, 0, new(big.Int), math.MaxUint64, new(big.Int), new(big.Int), new(big.Int), data, accessList, true)
}

func (d *Dexter) getReadOnlyEvm() *vm.EVM {
	bs := d.svc.store.GetBlockState().Copy()
	evmStateReader := &EvmStateReader{
		ServiceFeed: &d.svc.feed,
		store:       d.svc.store,
	}
	statedb, err := d.svc.store.evm.StateDB(bs.FinalizedStateRoot)
	if err != nil {
		log.Info("Could not make StateDB", "err", err)
		return nil
	}
	vmConfig := &opera.DefaultVMConfig
	txContext := vm.TxContext{
		Origin:   nullAddr,
		GasPrice: new(big.Int),
	}
	header := d.evmHeader(&bs, evmStateReader)
	context := evmcore.NewEVMBlockContext(header, evmStateReader, nil)
	config := d.svc.store.GetRules().EvmChainConfig()
	return vm.NewEVM(context, txContext, statedb, config, *vmConfig)
}

func (d *Dexter) evmHeader(bs *iblockproc.BlockState, reader evmcore.DummyChain) *evmcore.EvmHeader {
	baseFee := d.svc.store.GetRules().Economy.MinGasPrice
	if !d.svc.store.GetRules().Upgrades.London {
		baseFee = nil
	}
	return &evmcore.EvmHeader{
		Number:     utils.U64toBig(uint64(bs.LastBlock.Idx + 1)),
		Hash:       common.Hash{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		ParentHash: reader.GetHeader(common.Hash{}, uint64(bs.LastBlock.Idx)).Hash,
		Root:       common.Hash{},
		Time:       inter.Timestamp(uint64(time.Now().UnixNano())),
		Coinbase:   common.Address{},
		GasLimit:   math.MaxUint64,
		GasUsed:    0,
		BaseFee:    baseFee,
	}
}

func (d *Dexter) evmBlockWith(txs types.Transactions, bs *iblockproc.BlockState, reader evmcore.DummyChain) *evmcore.EvmBlock {
	return evmcore.NewEvmBlock(d.evmHeader(bs, reader), txs)
}

func (d *Dexter) processTx(tx *types.Transaction) {
	start := time.Now()
	data := tx.Data()
	if len(data) < 4 {
		return
	}
	method := data[:4]

	bs := d.svc.store.GetBlockState().Copy()
	evmStateReader := &EvmStateReader{
		ServiceFeed: &d.svc.feed,
		store:       d.svc.store,
	}
	statedb, err := d.svc.store.evm.StateDB(bs.FinalizedStateRoot)
	if err != nil {
		log.Info("Could not make StateDB", "err", err)
		return
	}
	evmProcessor := evmcore.NewStateProcessor(
		d.svc.store.GetRules().EvmChainConfig(),
		evmStateReader)
	txs := types.Transactions{tx}
	evmBlock := d.evmBlockWith(txs, &bs, evmStateReader)

	var gasUsed uint64
	pairsInfoOverride := make(map[common.Address]*PairInfo)
	var updatedKeys []PairKey
	evmProcessor.Process(evmBlock, statedb, opera.DefaultVMConfig, &gasUsed, false, func(l *types.Log, _ *state.StateDB) {
		pairAddr, reserve0, reserve1 := getReservesFromSyncLog(l)
		if pairAddr != nil {
			if pairInfo, ok := d.pairsInfo[*pairAddr]; ok {
				pairsInfoOverride[*pairAddr] = &PairInfo{
					reserve0:     reserve0,
					reserve1:     reserve1,
					token0:       pairInfo.token0,
					token1:       pairInfo.token1,
					feeNumerator: pairInfo.feeNumerator,
				}
				updatedKeys = append(updatedKeys,
					pairKeyFromAddrs(pairInfo.token0, pairInfo.token1, *pairAddr),
					pairKeyFromAddrs(pairInfo.token1, pairInfo.token0, *pairAddr))
			}
		}
	})
	if len(updatedKeys) == 0 {
		return
	}
	// log.Info("Finished processing tx", "hash", tx.Hash().Hex(), "t", utils.PrettyDuration(time.Now().Sub(start)), "updatedKeys", len(updatedKeys))
	// for pairAddr, pairInfo := range pairsInfoOverride {
	// 	log.Info("PairInfo override", "pairAddr", pairAddr, "reserve0", pairInfo.reserve0, "reserve1", pairInfo.reserve1, "info", *pairInfo)
	// }
	var allProfitableRoutes []uint
	candidateRoutes := 0
	for _, key := range updatedKeys {
		// log.Info("Evaluating updated key", "key", key)
		profitableRoutes := d.getProfitableRoutes(key, pairsInfoOverride)
		allProfitableRoutes = append(allProfitableRoutes, profitableRoutes...)
		candidateRoutes += len(d.routeCache.PairToRouteIdxs[key])
	}
	sort.Slice(allProfitableRoutes, func(a, b int) bool { return allProfitableRoutes[a] < allProfitableRoutes[b] })
	allProfitableRoutes = uniq(allProfitableRoutes)
	plan := d.getMostProfitablePath(allProfitableRoutes, pairsInfoOverride, tx.GasPrice())
	if plan == nil {
		return
	}
	log.Info("Target tx", "profitable", len(allProfitableRoutes), "/", candidateRoutes, "t", utils.PrettyDuration(time.Now().Sub(start)), "method", method, "hash", tx.Hash().Hex(), "gasPrice", tx.GasPrice(), "size", tx.Size())
	d.txLagRequestChan <- tx.Hash()
	log.Info("Found plan which is profitable after gas", "plan", plan)
	validators, epoch := d.svc.store.GetEpochValidators()
	from, err := types.Sender(d.signer, tx)
	if err != nil {
		log.Info("Could not get From for tx", "tx", tx.Hash().Hex(), "err", err)
		return
	}
	validatorIDs := d.predictValidators(from, tx.Nonce(), validators, epoch, 3)
	log.Info("Predicted validator", "validators", validatorIDs)
	gunQ, ok := d.guns[validatorIDs[0]]
	if !ok || len(gunQ) == 0 {
		log.Warn("Could not find validator", "validator", validatorIDs[0])
		return
	}
	wallet := gunQ[0]
	d.guns[validatorIDs[0]] = gunQ[1:]
	fishCall := fish3_lite.SwapSinglePath(maxAmountIn, plan.GasCost, plan.Path, fishAddr)
	account := wallet.Accounts()[0]
	nonce := d.svc.txpool.Nonce(account.Address)
	// log.Info("Source tx", "url", "https://ftmscan.com/tx/"+tx.Hash().Hex())
	fishTx := types.NewTransaction(nonce, fishAddr, common.Big0, 6e5, tx.GasPrice(), fishCall)
	signedTx, err := wallet.SignTx(account, fishTx, d.svc.store.GetRules().EvmChainConfig().ChainID)
	if err != nil {
		log.Error("Could not sign tx", "err", err)
		return
	}
	log.Info("FIRING GUN pew pew", "addr", account.Address, "targetTo", tx.To(), "method", method,
		"target", tx.Hash().Hex(), "gas", tx.GasPrice(), "size", tx.Size())
	// log.Info("Fired tx", "url", "https://ftmscan.com/tx/"+signedTx.Hash().Hex())
	d.ignoreTxs[tx.Hash()] = struct{}{}
	go d.fireGun(wallet, signedTx, tx, validatorIDs)

}

func (d *Dexter) fireGun(wallet accounts.Wallet, signedTx, targetTx *types.Transaction, validatorIDs []idx.ValidatorID) {
	d.svc.handler.BroadcastTxsAggressive([]*types.Transaction{signedTx})
	// d.svc.handler.BroadcastTxsAggressive([]*types.Transaction{targetTx, signedTx})
	d.svc.txpool.AddLocal(signedTx)
	d.watchedTxs <- &TxSub{Hash: targetTx.Hash(), Label: "source", PredictedValidators: validatorIDs}
	d.watchedTxs <- &TxSub{Hash: signedTx.Hash(), Label: targetTx.Hash().Hex(), PredictedValidators: validatorIDs}
}

func getReservesFromSyncLog(l *types.Log) (*common.Address, *big.Int, *big.Int) {
	if len(l.Topics) != 1 || bytes.Compare(l.Topics[0].Bytes(), syncEventTopic) != 0 {
		return nil, nil, nil
	}
	reserve0 := new(big.Int).SetBytes(l.Data[:32])
	reserve1 := new(big.Int).SetBytes(l.Data[32:])
	// if reserve0.BitLen() == 0 || reserve1.BitLen() == 0 {
	// 	log.Info("WARNING: getReservesFromSyncLog() returned 0", "addr", l.Address, "reserve0", reserve0, "reserve1", reserve1, "returnData", l.Data)
	// }
	return &l.Address, reserve0, reserve1
}

func uniq(sortedSlice []uint) (res []uint) {
	prev := ^uint(0) // MaxUint
	for _, v := range sortedSlice {
		if v != prev {
			res = append(res, v)
			prev = v
		}
	}
	return
}

func pairKeyFromStrs(from, to, pairAddr string) PairKey {
	return pairKeyFromAddrs(common.HexToAddress(from), common.HexToAddress(to), common.HexToAddress(pairAddr))
}

func pairKeyFromAddrs(from, to, pairAddr common.Address) PairKey {
	var key PairKey
	copy(key[:20], from[:])
	copy(key[20:], to[:])
	copy(key[40:], pairAddr[:])
	return key
}

func getAmountOut(amountIn, reserveIn, reserveOut, feeNumerator *big.Int) *big.Int {
	tenToSix := big.NewInt(int64(1e6))
	amountInWithFee := new(big.Int).Mul(amountIn, feeNumerator)
	numerator := new(big.Int).Mul(amountInWithFee, reserveOut)
	denominator := new(big.Int).Mul(reserveIn, tenToSix)
	denominator = denominator.Add(denominator, amountInWithFee)
	if denominator.BitLen() == 0 {
		log.Info("WARNING: getAmountOut returning 0", "reserveIn", reserveIn, "reserveOut", reserveOut, "amountIn", amountIn)
		return big.NewInt(0)
	}
	return numerator.Div(numerator, denominator)
}

func (d *Dexter) getPairInfo(pairsInfoOverride map[common.Address]*PairInfo, pairAddr common.Address) *PairInfo {
	if pairsInfoOverride != nil {
		if pairInfo, ok := pairsInfoOverride[pairAddr]; ok {
			if pairInfo.reserve0.BitLen() == 0 || pairInfo.reserve1.BitLen() == 0 {
				log.Info("WARNING: getPairInfo found override with 0 reserve0 or reserve1", "Addr", pairAddr, "len", len(pairsInfoOverride))
			}
			return pairInfo
		}
	}
	d.mu.RLock()
	pairInfo := d.pairsInfo[pairAddr]
	d.mu.RUnlock()
	if pairInfo.reserve0.BitLen() == 0 || pairInfo.reserve1.BitLen() == 0 {
		log.Info("WARNING: getPairInfo found baseline with 0 reserve0 or reserve1", "Addr", pairAddr, "reserve0", pairInfo.reserve0, "reserve1", pairInfo.reserve1)
	}
	return pairInfo
}

func (d *Dexter) getRouteAmountOut(route []*Leg, amountIn *big.Int, pairsInfoOverride map[common.Address]*PairInfo) *big.Int {
	var amountOut *big.Int
	for _, leg := range route {
		pairInfo := d.getPairInfo(pairsInfoOverride, leg.PairAddr)
		var reserveFrom *big.Int
		var reserveTo *big.Int
		if bytes.Compare(pairInfo.token0.Bytes(), leg.From.Bytes()) == 0 {
			reserveFrom, reserveTo = pairInfo.reserve0, pairInfo.reserve1
		} else {
			reserveFrom, reserveTo = pairInfo.reserve1, pairInfo.reserve0
		}
		amountOut = getAmountOut(amountIn, reserveFrom, reserveTo, pairInfo.feeNumerator)
		amountIn = amountOut
	}
	return amountOut
}

func (d *Dexter) getRouteAmountOutDebug(route []*Leg, amountIn *big.Int, pairsInfoOverride map[common.Address]*PairInfo, label string) *big.Int {
	var amountOut *big.Int
	for i, leg := range route {
		pairInfo := d.getPairInfo(pairsInfoOverride, leg.PairAddr)
		var reserveFrom *big.Int
		var reserveTo *big.Int
		if bytes.Compare(pairInfo.token0.Bytes(), leg.From.Bytes()) == 0 {
			reserveFrom, reserveTo = pairInfo.reserve0, pairInfo.reserve1
		} else {
			reserveFrom, reserveTo = pairInfo.reserve1, pairInfo.reserve0
		}
		amountOut = getAmountOut(amountIn, reserveFrom, reserveTo, pairInfo.feeNumerator)
		log.Info("DEBUG", "label", label, "i", i, "amountIn", amountIn, "amountOut", amountOut, "reserveFrom", reserveFrom, "reserveTo", reserveTo, "feeNumerator", pairInfo.feeNumerator)
		amountIn = amountOut
	}
	return amountOut
}

func (d *Dexter) getProfitableRoutes(key PairKey, pairsInfoOverride map[common.Address]*PairInfo) []uint {
	if routeIdxs, ok := d.routeCache.PairToRouteIdxs[key]; ok {
		for i, routeIdx := range routeIdxs {
			route := d.routeCache.Routes[routeIdx]
			amountOut := d.getRouteAmountOut(route, startTokensIn, pairsInfoOverride)
			if amountOut.Cmp(startTokensIn) < 1 {
				return routeIdxs[:i]
				// if end == -1 {
				// 	end = i
				// }
				// } else if end > -1 {
				// log.Warn("Profitable route detected after end", "end", end, "i", i)
			}
		}
	}
	return nil
}

func (d *Dexter) getScore(route []*Leg, pairsInfoOverride map[common.Address]*PairInfo) uint64 {
	amountOut := d.getRouteAmountOut(route, startTokensIn, pairsInfoOverride)
	amountOut.Div(amountOut, big.NewInt(1e12))
	return uint64(amountOut.Int64())
}

func (d *Dexter) refreshScores() {
	for i, route := range d.routeCache.Routes {
		d.routeCache.Scores[i] = d.getScore(route, nil)
	}
}

func (d *Dexter) refreshScoresForPair(key PairKey, pairsInfoOverride map[common.Address]*PairInfo, scores []uint64) {
	if routeIdxs, ok := d.routeCache.PairToRouteIdxs[key]; ok {
		for _, routeIdx := range routeIdxs {
			route := d.routeCache.Routes[routeIdx]
			scores[routeIdx] = d.getScore(route, pairsInfoOverride)
		}
	}
}

func (d *Dexter) sortPairToRouteIdxMap(scores []uint64) map[PairKey][]uint {
	pairToRouteIdxs := make(map[PairKey][]uint, len(d.routeCache.PairToRouteIdxs))
	d.mu.RLock()
	for pairKey, routeIdxs := range d.routeCache.PairToRouteIdxs {
		newRouteIdxs := make([]uint, len(routeIdxs))
		copy(newRouteIdxs, routeIdxs)
		pairToRouteIdxs[pairKey] = newRouteIdxs
	}
	d.mu.RUnlock()
	for _, routeIdxs := range pairToRouteIdxs {
		sort.Slice(routeIdxs, func(a, b int) bool {
			return scores[routeIdxs[a]] > scores[routeIdxs[b]]
		})
	}
	return pairToRouteIdxs
}

func (d *Dexter) getRouteOptimalAmountIn(route []*Leg, pairsInfoOverride map[common.Address]*PairInfo) *big.Int {
	startPairInfo := d.getPairInfo(pairsInfoOverride, route[0].PairAddr)
	leftAmount, rightAmount := new(big.Int), new(big.Int)
	if bytes.Compare(startPairInfo.token0.Bytes(), route[0].From.Bytes()) == 0 {
		leftAmount.Set(startPairInfo.reserve0)
		rightAmount.Set(startPairInfo.reserve1)
	} else {
		leftAmount.Set(startPairInfo.reserve1)
		rightAmount.Set(startPairInfo.reserve0)
	}
	// log.Info("Starting getRouteOptimalAmountIn", "leftAmount", leftAmount, "rightAmount", rightAmount, "reserve0", startPairInfo.reserve0, "reserve1", startPairInfo.reserve1, "startPairInfo", *startPairInfo)
	r1 := startPairInfo.feeNumerator
	tenToSix := big.NewInt(int64(1e6))
	for _, leg := range route[1:] {
		pairInfo := d.getPairInfo(pairsInfoOverride, leg.PairAddr)
		var reserveFrom, reserveTo *big.Int
		if bytes.Compare(pairInfo.token0.Bytes(), leg.From.Bytes()) == 0 {
			reserveFrom, reserveTo = pairInfo.reserve0, pairInfo.reserve1
		} else {
			reserveTo, reserveFrom = pairInfo.reserve0, pairInfo.reserve1
		}
		legFee := pairInfo.feeNumerator
		den := new(big.Int).Mul(rightAmount, legFee)
		den = den.Div(den, tenToSix)
		den = den.Add(den, reserveFrom)

		// log.Info("getRouteOptimalAmountIn step", "leftAmount", leftAmount, "rightAmount", rightAmount, "reserveFrom", reserveFrom, "reserveTo", reserveTo, "reserve0", pairInfo.reserve0, "reserve1", pairInfo.reserve1, "den", den, "from", leg.From.Bytes()[:2], "token0", pairInfo.token0.Bytes()[:2])
		leftAmount = leftAmount.Mul(leftAmount, reserveFrom)
		leftAmount = leftAmount.Div(leftAmount, den)

		rightAmount = rightAmount.Mul(rightAmount, reserveTo)
		rightAmount = rightAmount.Mul(rightAmount, legFee)
		rightAmount = rightAmount.Div(rightAmount, tenToSix)
		rightAmount = rightAmount.Div(rightAmount, den)
	}
	// log.Info("Computed left and right", "leftAmount", leftAmount, "rightAmount", rightAmount, "lbits", leftAmount.BitLen(), "rbits", rightAmount.BitLen())
	amountIn := new(big.Int).Mul(rightAmount, leftAmount)
	amountIn = amountIn.Mul(amountIn, r1)
	amountIn = amountIn.Div(amountIn, tenToSix)
	amountIn = amountIn.Sqrt(amountIn)
	amountIn = amountIn.Sub(amountIn, leftAmount)
	amountIn = amountIn.Mul(amountIn, tenToSix)
	amountIn = amountIn.Div(amountIn, r1)
	if amountIn.Cmp(maxAmountIn) == 1 { // amountIn > maxAmountIn
		return maxAmountIn
	}
	return amountIn
}

func estimateFishGas(numTransfers, numSwaps int, gasPrice *big.Int) *big.Int {
	gas := GAS_INITIAL + (GAS_TRANSFER * numTransfers) + (GAS_SWAP * numSwaps)
	return new(big.Int).Mul(gasPrice, big.NewInt(int64(gas)))
}

func estimateFailureCost(gasPrice *big.Int) *big.Int {
	return new(big.Int).Mul(gasPrice, big.NewInt(GAS_FAIL*1))
}

func (d *Dexter) getMostProfitablePath(routeIdxs []uint, pairsInfoOverride map[common.Address]*PairInfo, gasPrice *big.Int) *Plan {
	maxProfit := new(big.Int)
	var bestAmountIn, bestAmountOut, bestGas *big.Int
	var bestRouteIdx uint
	for _, routeIdx := range routeIdxs {
		route := d.routeCache.Routes[routeIdx]
		amountIn := d.getRouteOptimalAmountIn(route, pairsInfoOverride)
		if amountIn.Sign() == -1 {
			log.Info("WARNING: Negative amountIn for route", "routeIdx", routeIdx, "amountIn", amountIn)
			amountOut := d.getRouteAmountOut(route, startTokensIn, pairsInfoOverride)
			log.Info("Amount out for startTokensIn", "routeIdx", routeIdx, "amountOut", amountOut)

			continue
		}
		amountOut := d.getRouteAmountOut(route, amountIn, pairsInfoOverride)
		if amountOut.Cmp(amountIn) == -1 {
			// log.Info("WARNING: Negative profit for route")
			continue
		}
		profit := new(big.Int).Sub(amountOut, amountIn)

		// if amountIn.Cmp(maxAmountIn) == -1 {
		// 	amountIn2 := new(big.Int).Add(amountIn, new(big.Int).Div(amountIn, big.NewInt(2)))
		// 	if amountIn2.Cmp(maxAmountIn) == 1 {
		// 		amountIn2 = maxAmountIn
		// 	}
		// 	amountOut2 := d.getRouteAmountOut(route, amountIn2, pairsInfoOverride)
		// 	profit2 := new(big.Int).Sub(amountOut2, amountIn2)
		// 	if profit2.Cmp(profit) == 1 {
		// 		log.Warn("Profit2 > profit", "amountIn", amountIn, "amountOut", amountOut, "amountIn2", amountIn2, "amountOut2", amountOut2, "profit", profit, "profit2", profit2)
		// 		// d.getRouteAmountOutDebug(route, amountIn, pairsInfoOverride, "conventional")
		// 		// d.getRouteAmountOutDebug(route, amountIn2, pairsInfoOverride, "plus sized")
		// 	}
		// }

		gas := estimateFishGas(1, len(route), gasPrice)
		netProfit := new(big.Int).Sub(profit, gas)
		netProfitSubFailures := new(big.Int).Sub(netProfit, estimateFailureCost(gasPrice))
		if netProfitSubFailures.Cmp(maxProfit) == -1 {
			continue
		}
		// log.Info("New most profitable route", "amountIn", amountIn, "amountOut", amountOut, "profit", profit, "netProfit", netProfit, "gas", gas, "maxProfit", maxProfit, "routeIdx", routeIdx)
		maxProfit = netProfit
		bestAmountIn = amountIn
		bestAmountOut = amountOut
		bestGas = gas
		bestRouteIdx = routeIdx
	}
	if maxProfit.BitLen() == 0 {
		return nil
	}
	log.Info("Best route", "routeIdx", bestRouteIdx, "bestAmountIn", bestAmountIn, "bestAmountOut", bestAmountOut, "bestGas", bestGas)
	return d.makePlan(bestRouteIdx, bestGas, gasPrice, bestAmountIn)
}

func (d *Dexter) makePlan(routeIdx uint, gasCost, gasPrice, amountIn *big.Int) *Plan {
	route := d.routeCache.Routes[routeIdx]
	plan := &Plan{
		GasPrice: gasPrice,
		GasCost:  gasCost,
		AmountIn: amountIn,
		Path:     make([]fish3_lite.SwapCommand, len(route)),
	}
	for i, leg := range route {
		pairInfo := d.pairsInfo[leg.PairAddr] // No need to use override as we don't look up reserves
		fromToken0 := bytes.Compare(pairInfo.token0.Bytes(), leg.From.Bytes()) == 0
		plan.Path[i] = fish3_lite.SwapCommand{
			Pair:         leg.PairAddr,
			Token0:       pairInfo.token0,
			Token1:       pairInfo.token1,
			Fraction:     big.NewInt(1),
			FeeNumerator: pairInfo.feeNumerator,
			FromToken0:   fromToken0,
			SwapTo:       0,
		}
	}
	return plan
}
