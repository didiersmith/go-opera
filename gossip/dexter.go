package gossip

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"math"
	"math/big"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/Fantom-foundation/go-opera/contracts/fish3_lite"
	"github.com/Fantom-foundation/go-opera/contracts/uniswap_pair_lite"
	"github.com/Fantom-foundation/go-opera/dexter"
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
	numValidators = 8
)

var (
	fishAddr           = common.HexToAddress("0xba164fB7530b24cF73d183ce0140AF9Ab8C35Cd8")
	nullAddr           = common.HexToAddress("0x0000000000000000000000000000000000000000")
	syncEventTopic     = []byte{0x1c, 0x41, 0x1e, 0x9a, 0x96, 0xe0, 0x71, 0x24, 0x1c, 0x2f, 0x21, 0xf7, 0x72, 0x6b, 0x17, 0xae, 0x89, 0xe3, 0xca, 0xb4, 0xc7, 0x8b, 0xe5, 0x0e, 0x06, 0x2b, 0x03, 0xa9, 0xff, 0xfb, 0xba, 0xd1}
	pairGetReservesAbi = uniswap_pair_lite.GetReserves()
	pairToken0Abi      = uniswap_pair_lite.Token0()
	pairToken1Abi      = uniswap_pair_lite.Token1()
	root               = "/home/ubuntu/dexter/carb/"
	MaxGasPrice        = big.NewInt(25000000000000)
	accuracyAlpha      = 0.8
	bravadoAlpha       = 0.75
	gasAlpha           = 0.5
	contracts          = map[common.Address]string{
		common.HexToAddress("0xba164fB7530b24cF73d183ce0140AF9Ab8C35Cd8"): "Fish3",
		common.HexToAddress("0xd8Fc012498F4278095F10190DD3F29a8A2f16a52"): "Mr Million",
		common.HexToAddress("0x6C080c87e84e71C4c8364128dBE66f5e30a0e370"): "Starving Artist",
		common.HexToAddress("0x244FAcabcf7a1026849B53295b1F7279a1bD597b"): "Topher Ford",
		common.HexToAddress("0xd20d27d5cB769522410e0A2f32000C947af7Ffe5"): "Dee Twenny",
		common.HexToAddress("0x0306c72b69E62be7c7168828cB98Dd41A0B01f8B"): "Flash Boy",
		common.HexToAddress("0x4365C7DD814e1f4b3104B6b5C1079B32b1eA6E8D"): "Sam McGee",
		common.HexToAddress("0xD8E1D3608D927d370afFE5EaaF6B050BDc11df6E"): "Daisy with a reputation",
		common.HexToAddress("0x3BCC05Bc23a6D3223C09F1724bE631c3A2Ff3a77"): "Flipper",
		common.HexToAddress("0xF4C587a0972Ac2039BFF67Bc44574bB403eF5235"): "ProtoFi",
		common.HexToAddress("0x845E76A8691423fbc4ECb8Dd77556Cb61c09eE25"): "JetSwap",
		common.HexToAddress("0x6D0176C5ea1e44b08D3dd001b0784cE42F47a3A7"): "TombSwap",
		common.HexToAddress("0x4D2cf285a519261F30b4d9c2c344Baf260d65Fa2"): "Elk",
		common.HexToAddress("0x8aC868293D97761A1fED6d4A01E9FF17C5594Aa3"): "Morpheus",
		common.HexToAddress("0xc8Fe105cEB91e485fb0AC338F2994Ea655C78691"): "Excalibur",
		common.HexToAddress("0x5023882f4D1EC10544FCB2066abE9C1645E95AA0"): "Wigo",
		common.HexToAddress("0x045312C737a6b7a115906Be0aD0ef53A6AA38106"): "Dark Knight",
		common.HexToAddress("0x16327E3FbDaCA3bcF7E38F5Af2599D2DDc33aE52"): "SpiritSwap",
		common.HexToAddress("0x7b17021fcb7bc888641dc3bedfed3734fcaf2c87"): "WakaSwap",
		common.HexToAddress("0x53c153a0df7E050BbEFbb70eE9632061f12795fB"): "HyperJump",
		common.HexToAddress("0xf491e7b69e4244ad4002bc14e878a34207e38c29"): "SpookySwap",
		common.HexToAddress("0xb9799De71100e20aC1cdbCc63C69ddA2D0D81710"): "fBomb",
		common.HexToAddress("0xfD000ddCEa75a2E23059881c3589F6425bFf1AbB"): "PaintSwap",
		common.HexToAddress("0x5F54B226f08d1fB9b612ADfa07f2CfF726c6cA46"): "DefySwap",
		common.HexToAddress("0xcdA8f0fB4132D977AD427d18555E0cb1b1dfA363"): "Degen",
	}
)

type Dexter struct {
	svc               *Service
	inTxChan          chan *types.Transaction
	inLogsChan        chan []*types.Log
	inLogsSub         notify.Subscription
	inBlockChan       chan evmcore.ChainHeadNotify
	inBlockSub        notify.Subscription
	inEpochChan       chan idx.Epoch
	inEpochSub        notify.Subscription
	inEventChan       chan *inter.EventPayload
	inEventSub        notify.Subscription
	lag               time.Duration
	gunRefreshes      chan GunList
	signer            types.Signer
	guns              map[idx.ValidatorID][]accounts.Wallet // Deprecated
	gunList           GunList
	watchedTxs        chan *TxSub
	ignoreTxs         map[common.Hash]struct{}
	interestedPairs   map[common.Address]struct{}
	clearIgnoreTxChan chan common.Hash
	eventRaceChan     chan *RaceEntry
	txRaceChan        chan *RaceEntry
	railgunChan       chan *dexter.RailgunPacket
	txLagRequestChan  chan common.Hash
	strategies        []dexter.Strategy
	strategyBravado   []float64
	firedTxChan       chan *FiredTx
	pairsInfo         map[common.Address]*dexter.PairInfo
	accuracy          float64
	gasFloors         map[idx.ValidatorID]int64
	globalGasFloor    int64
	numPending        int
	mu                sync.RWMutex
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
	TargetValidators    []idx.ValidatorID
	Print               bool
	StartTime           time.Time
}

type FiredTx struct {
	Hash       common.Hash
	StrategyID int
	Time       time.Time
}

type PairInfoJson struct {
	Addr         string `json:addr`
	Token0       string `json:token0`
	Token1       string `json:token1`
	ExchangeName string `json:exchangeName`
	FeeNumerator int64  `json:feeNumerator`
}

func NewDexter(svc *Service) *Dexter {
	d := &Dexter{
		svc:               svc,
		inTxChan:          make(chan *types.Transaction, 256),
		inLogsChan:        make(chan []*types.Log, 256),
		inBlockChan:       make(chan evmcore.ChainHeadNotify, 256),
		inEpochChan:       make(chan idx.Epoch, 256),
		inEventChan:       make(chan *inter.EventPayload, 256),
		interestedPairs:   make(map[common.Address]struct{}),
		lag:               time.Minute * 60,
		signer:            gsignercache.Wrap(types.LatestSignerForChainID(svc.store.GetRules().EvmChainConfig().ChainID)),
		gunRefreshes:      make(chan GunList),
		watchedTxs:        make(chan *TxSub),
		firedTxChan:       make(chan *FiredTx),
		ignoreTxs:         make(map[common.Hash]struct{}),
		clearIgnoreTxChan: make(chan common.Hash, 16),
		eventRaceChan:     make(chan *RaceEntry, 4096),
		txRaceChan:        make(chan *RaceEntry, 4096),
		railgunChan:       make(chan *dexter.RailgunPacket, 8),
		txLagRequestChan:  make(chan common.Hash, 16),
		pairsInfo:         make(map[common.Address]*dexter.PairInfo),
		gasFloors:         make(map[idx.ValidatorID]int64),
	}
	d.strategies = []dexter.Strategy{
		dexter.NewLinearStrategy("Linear", 0, d.railgunChan, dexter.LinearStrategyConfig{
			RoutesFileName:          root + "route_cache_routes_10ftm.json",
			PairToRouteIdxsFileName: root + "route_cache_pairToRouteIdxs_10ftm.json",
		}),
		// dexter.NewLinearStrategy("Linear 2", 0, d.railgunChan, dexter.LinearStrategyConfig{
		// 	RoutesFileName:          root + "route_cache_routes_len2.json",
		// 	PairToRouteIdxsFileName: root + "route_cache_pairToRouteIdxs_len2.json",
		// }),
		// dexter.NewLinearStrategy("Linear 3", 1, d.railgunChan, dexter.LinearStrategyConfig{
		// 	RoutesFileName:          root + "route_cache_routes_len3.json",
		// 	PairToRouteIdxsFileName: root + "route_cache_pairToRouteIdxs_len3.json",
		// }),
		// dexter.NewLinearStrategy("Linear 4", 2, d.railgunChan, dexter.LinearStrategyConfig{
		// 	RoutesFileName:          root + "route_cache_routes_len4.json",
		// 	PairToRouteIdxsFileName: root + "route_cache_pairToRouteIdxs_len4.json",
		// }),
	}
	d.strategyBravado = make([]float64, len(d.strategies))
	for i := 0; i < len(d.strategyBravado); i++ {
		d.strategyBravado[i] = 1
	}
	// d.strategies[1].AddSubStrategy(d.strategies[1])
	// d.strategies[1].AddSubStrategy(d.strategies[2])
	d.inLogsSub = svc.feed.SubscribeNewLogs(d.inLogsChan)
	d.inBlockSub = svc.feed.SubscribeNewBlock(d.inBlockChan)
	d.inEpochSub = svc.feed.SubscribeNewEpoch(d.inEpochChan)
	svc.handler.SubscribeEvents(d.inEventChan)
	d.loadJson()

	for _, s := range d.strategies {
		for pairAddr, _ := range s.GetInterestedPairs() {
			d.interestedPairs[pairAddr] = struct{}{}
		}
	}

	for pairAddr, _ := range d.interestedPairs {
		pairInfo, ok := d.pairsInfo[pairAddr]
		if !ok {
			log.Warn("Could not find PairInfo for interested pair", "addr", pairAddr)
			continue
		}
		reserve0, reserve1 := d.getReserves(&pairAddr)
		token0, token1 := d.getPairTokens(&pairAddr)
		d.pairsInfo[pairAddr] = &dexter.PairInfo{
			Reserves:     []*big.Int{reserve0, reserve1},
			Token0:       token0,
			Token1:       token1,
			FeeNumerator: pairInfo.FeeNumerator,
		}
	}

	for _, s := range d.strategies {
		s.SetPairsInfo(d.pairsInfo)
		s.Start()
	}

	go d.processIncomingLogs()
	go d.processIncomingTxs()
	go d.watchEvents()
	go d.eventRace()
	go d.runRailgun()
	svc.handler.RaceEvents(d.eventRaceChan)
	svc.handler.RaceTxs(d.txRaceChan)
	return d
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
			// d.svc.handler.peers.Cull(losers)
			d.svc.handler.peers.SetSortedPeers(sortedPeers)
			d.svc.handler.peers.SetSortedTxPeers(sortedTxPeers)

			eventWins = make(map[string]int)
			txWins = make(map[string]int)
			prevCullTime = time.Now()
		}
	}
}

func (d *Dexter) loadJson() {
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
		d.pairsInfo[pairAddr] = &dexter.PairInfo{
			FeeNumerator: big.NewInt(jsonPair.FeeNumerator),
		}
	}
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

type Gun struct {
	ValidatorIDs []idx.ValidatorID
	Wallet       accounts.Wallet
}
type GunList []Gun

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
func CmpValidatorIDs(v1, v2 []idx.ValidatorID) int {
	for i := 0; i < len(v1); i++ {
		if v1[i] < v2[i] {
			return -(i + 1)
		}
		if v1[i] > v2[i] {
			return i + 1
		}
	}
	return 0
}

func (g GunList) Len() int      { return len(g) }
func (g GunList) Swap(i, j int) { g[i], g[j] = g[j], g[i] }
func (g GunList) Less(i, j int) bool {
	return CmpValidatorIDs(g[i].ValidatorIDs, g[j].ValidatorIDs) < 0
}
func (g GunList) Search(needle []idx.ValidatorID) int {
	if len(g) < 1 {
		return -1
	}
	low := 0
	high := len(g) - 1
	for low < high-1 {
		median := (low + high) / 2
		score := CmpValidatorIDs(g[median].ValidatorIDs, needle)
		if score < 0 {
			low = median
		} else {
			high = median
		}
	}
	if abs(CmpValidatorIDs(g[low].ValidatorIDs, needle)) > abs(CmpValidatorIDs(g[high].ValidatorIDs, needle)) {
		return low
	}
	return high
}

func (d *Dexter) refreshGuns() {
	validators, epoch := d.svc.store.GetEpochValidators()
	wallets := d.svc.accountManager.Wallets()
	guns := make(GunList, 0, len(wallets))
	for i := 0; i < len(wallets); i++ {
		wallet := wallets[i]
		wStatus, _ := wallet.Status()
		if wStatus == "Locked" {
			continue
		}
		account := wallet.Accounts()[0]
		nonce := d.svc.txpool.Nonce(account.Address)
		validatorIDs := d.predictValidators(account.Address, nonce, validators, epoch, numValidators)
		guns = append(guns, Gun{
			ValidatorIDs: validatorIDs,
			Wallet:       wallet,
		})
	}
	sort.Sort(guns)
	d.gunRefreshes <- guns
}

func (d *Dexter) processIncomingLogs() {
	log.Info("Started dexter")
	methods := make(map[[4]byte]int)
	methodUsers := make(map[[4]byte]*common.Address)
	numUpdates := 0
	for {
		var updates []*dexter.PairUpdate
		select {
		case logs := <-d.inLogsChan:
			for _, l := range logs {
				pairAddr, reserve0, reserve1 := getReservesFromSyncLog(l)
				if pairAddr == nil {
					continue
				}
				_, ok := d.interestedPairs[*pairAddr]
				if !ok {
					continue
				}
				updates = append(updates, &dexter.PairUpdate{
					Addr:     *pairAddr,
					Reserves: []*big.Int{reserve0, reserve1},
				})
				block := d.svc.EthAPI.state.GetBlock(common.Hash{}, l.BlockNumber)
				if block == nil || uint(len(block.Transactions)) <= l.TxIndex {
					continue
				}
				tx := block.Transactions[l.TxIndex]
				data := tx.Data()
				var method [4]byte
				if len(data) >= 4 {
					numUpdates++
					copy(method[:], data[:4])
					if methodScore, ok := methods[method]; ok {
						methods[method] = methodScore + 1
					} else {
						methods[method] = 1
						methodUsers[method] = tx.To()
					}
				}
			}
			if len(updates) > 0 {
				for _, s := range d.strategies {
					s.ProcessPermUpdates(updates)
				}
			}
			// if numUpdates > 1000 {
			// 	numUpdates = 0
			// 	log.Info("1000 updates, dumping methods")
			// 	methodPairs := mapToSortedBytePairs(methods)
			// 	for _, p := range methodPairs {
			// 		log.Info("Method score", "method", p.Key, "score", p.Value, "user", methodUsers[p.Key])
			// 	}
			// 	methods = make(map[[4]byte]int)
			// }
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
	watchedTxMap := make(map[common.Hash]*FiredTx)
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
			d.numPending, _ = d.svc.txpool.Stats()
		case n := <-d.inBlockChan:
			d.lag = time.Now().Sub(n.Block.Time.Time())
			if d.lag < 4*time.Second {
				go d.refreshGuns()
			}
			for strategyID, bravado := range d.strategyBravado {
				d.strategyBravado[strategyID] = math.Min(1.0, bravado+0.005)
			}
			receipts, err := d.svc.EthAPI.GetReceipts(context.TODO(), n.Block.Hash)
			if err != nil {
				log.Error("Could not get block receipts", "err", err)
				continue
			}
			for i, tx := range n.Block.Transactions {
				if f, ok := watchedTxMap[tx.Hash()]; ok {
					receipt := receipts[i]
					if receipt.Status == types.ReceiptStatusSuccessful {

						d.strategyBravado[f.StrategyID] = d.strategyBravado[f.StrategyID]*bravadoAlpha + (1 - bravadoAlpha)
						log.Info("SUCCESS", "tx", tx.Hash().Hex(), "strategy", f.StrategyID, "new bravado", d.strategyBravado[f.StrategyID], "lag", utils.PrettyDuration(time.Now().Sub(f.Time)))
					} else {
						d.strategyBravado[f.StrategyID] = d.strategyBravado[f.StrategyID] * bravadoAlpha
						log.Info("Fail", "tx", tx.Hash().Hex(), "strategy", f.StrategyID, "new bravado", d.strategyBravado[f.StrategyID], "lag", utils.PrettyDuration(time.Now().Sub(f.Time)))
					}
					delete(watchedTxMap, tx.Hash())
				}
			}
		case <-d.inBlockSub.Err():
			return
		case f := <-d.firedTxChan:
			watchedTxMap[f.Hash] = f
		case h := <-d.clearIgnoreTxChan:
			delete(d.ignoreTxs, h)

		}
	}
	d.inLogsSub.Unsubscribe()
}

func (d *Dexter) processTx(tx *types.Transaction) {
	from, _ := types.Sender(d.signer, tx)
	validators, epoch := d.svc.store.GetEpochValidators() // TODO: Move this up?
	validatorIDs := d.predictValidators(from, tx.Nonce(), validators, epoch, numValidators)
	d.watchedTxs <- &TxSub{Hash: tx.Hash(), PredictedValidators: validatorIDs, Print: false, StartTime: time.Now()}
	if tx.GasPrice().Cmp(MaxGasPrice) == 1 {
		return
	}
	if d.numPending > 50 {
		if gasFloor, ok := d.gasFloors[validatorIDs[0]]; ok {
			if tx.GasPrice().Int64() < (gasFloor+d.globalGasFloor)/2 {
				return
			}
		}
	}
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
	ptx := &dexter.PossibleTx{
		Tx:           tx,
		ValidatorIDs: validatorIDs,
	}
	evmProcessor.Process(evmBlock, statedb, opera.DefaultVMConfig, &gasUsed, false, func(l *types.Log, _ *state.StateDB) {
		pairAddr, reserve0, reserve1 := getReservesFromSyncLog(l)
		if pairAddr != nil {
			if _, ok := d.interestedPairs[*pairAddr]; ok {
				ptx.Updates = append(ptx.Updates, dexter.PairUpdate{
					Addr:     *pairAddr,
					Reserves: []*big.Int{reserve0, reserve1},
				})
			}
		}
	})
	if len(ptx.Updates) == 0 {
		return
	}
	for _, s := range d.strategies {
		s.ProcessPossibleTx(ptx)
	}
}

func getReservesFromSyncLog(l *types.Log) (*common.Address, *big.Int, *big.Int) {
	if len(l.Topics) != 1 || bytes.Compare(l.Topics[0].Bytes(), syncEventTopic) != 0 {
		return nil, nil, nil
	}
	reserve0 := new(big.Int).SetBytes(l.Data[:32])
	reserve1 := new(big.Int).SetBytes(l.Data[32:])
	return &l.Address, reserve0, reserve1
}

func (d *Dexter) runRailgun() {
	for {
		select {
		case <-d.inEpochChan:
			go d.refreshGuns()
		case <-d.inEpochSub.Err():
			return
		case guns := <-d.gunRefreshes:
			d.gunList = guns
		case p := <-d.railgunChan:
			bravado := d.strategyBravado[p.StrategyID]
			probAdjustedPayoff := new(big.Int).Mul(p.Response.NetProfit, big.NewInt(int64(d.accuracy*bravado)))
			failCost := new(big.Int).Mul(p.Response.GasPrice, big.NewInt(dexter.GAS_FAIL))
			probAdjustedFailCost := new(big.Int).Mul(failCost, big.NewInt(int64(100-d.accuracy)))
			// log.Info("Win/lose costs", "win", probAdjustedPayoff, "lose", probAdjustedFailCost, "accuracy", d.accuracy, "bravado", bravado, "unadjusted win", p.Response.NetProfit, "unadjusted lose", failCost)
			if probAdjustedPayoff.Cmp(probAdjustedFailCost) == -1 {
				log.Info("Trade unprofitable after adjusting for accuracy", "accuracy", d.accuracy, "bravado", bravado)
				continue
			}

			gunIdx := d.gunList.Search(p.ValidatorIDs)
			if gunIdx < 0 {
				log.Warn("Gun list not initialized")
				continue
			}
			gun := d.gunList[gunIdx] // TODO: Prevent gun reuse with multiple strategies
			if gun.ValidatorIDs[0] != p.ValidatorIDs[0] {
				log.Warn("Could not find validator", "validator", p.ValidatorIDs[0])
				continue
			}
			wallet := gun.Wallet
			account := wallet.Accounts()[0]
			nonce := d.svc.txpool.Nonce(account.Address)
			// log.Info("Source tx", "url", "https://ftmscan.com/tx/"+tx.Hash().Hex())
			var fishCall []byte
			if p.Type == dexter.SwapSinglePath {
				fishCall = fish3_lite.SwapSinglePath(dexter.MaxAmountIn, p.Response.GasCost, p.Response.Path, fishAddr)
			} else {
				log.Error("Unsupported call type", "type", p.Type)
				continue
			}
			fishTx := types.NewTransaction(nonce, fishAddr, common.Big0, 6e5, p.Target.GasPrice(), fishCall)
			signedTx, err := wallet.SignTx(account, fishTx, d.svc.store.GetRules().EvmChainConfig().ChainID)
			if err != nil {
				log.Error("Could not sign tx", "err", err)
				return
			}
			// TODO: Here is a fine spot to write onto txLagRequestChan
			log.Info("FIRING GUN pew pew", "addr", account.Address, "targetTo", p.Target.To(),
				"target", p.Target.Hash().Hex(), "gas", p.Target.GasPrice(), "size", p.Target.Size())
			// log.Info("Fired tx", "url", "https://ftmscan.com/tx/"+signedTx.Hash().Hex())
			d.ignoreTxs[p.Target.Hash()] = struct{}{}
			go d.fireGun(wallet, signedTx, p, gun.ValidatorIDs)
		}
	}
}

func (d *Dexter) fireGun(wallet accounts.Wallet, signedTx *types.Transaction, p *dexter.RailgunPacket, gunValidatorIDs []idx.ValidatorID) {
	// d.svc.handler.BroadcastTxsAggressive([]*types.Transaction{signedTx})
	d.svc.handler.BroadcastTxsAggressive([]*types.Transaction{p.Target, signedTx})
	d.svc.txpool.AddLocal(signedTx)
	d.watchedTxs <- &TxSub{
		Hash:                p.Target.Hash(),
		Label:               "target",
		PredictedValidators: p.ValidatorIDs,
		Print:               true,
		StartTime:           time.Now(),
	}
	d.watchedTxs <- &TxSub{
		Hash:                signedTx.Hash(),
		Label:               p.Target.Hash().Hex(),
		PredictedValidators: gunValidatorIDs,
		TargetValidators:    p.ValidatorIDs,
		Print:               true,
		StartTime:           time.Now(),
	}
	d.firedTxChan <- &FiredTx{
		Hash:       signedTx.Hash(),
		StrategyID: p.StrategyID,
		Time:       time.Now(),
	}
}

func (d *Dexter) watchEvents() {
	watchedTxMap := make(map[common.Hash]*TxSub)
	gasAlpha1 := int64(100.0 * gasAlpha)
	gasAlpha2 := int64(100.0 * (1.0 - gasAlpha))
	for {
		select {
		case e := <-d.inEventChan:
			interested := false
			txLabels := make(map[common.Hash]string)
			var lowestGas int64 = 0
			for _, tx := range e.Txs() {
				lowestGas = tx.GasPrice().Int64()
				if sub, ok := watchedTxMap[tx.Hash()]; ok {
					var msg string
					if e.Locator().Creator == sub.PredictedValidators[0] {
						d.accuracy = d.accuracy*accuracyAlpha + 100.0*(1-accuracyAlpha)
						msg = "Correct validator prediction"
					} else {
						d.accuracy = d.accuracy * accuracyAlpha
						msg = "Wrong validator prediction"
					}
					if sub.Print {
						interested = true
						log.Info(msg, "id", e.ID(), "lamport", e.Locator().Lamport, "creator", e.Locator().Creator, "predicted", sub.PredictedValidators, "target", sub.TargetValidators, "lag", utils.PrettyDuration(time.Now().Sub(sub.StartTime)), "tx", tx.Hash().Hex(), "label", sub.Label)
						if sub.Label == "target" {
							txLabels[tx.Hash()] = "TARGET"
						} else {
							txLabels[tx.Hash()] = sub.Label
						}
					}
					delete(watchedTxMap, tx.Hash())
					d.clearIgnoreTxChan <- tx.Hash()
				}
			}
			if interested {
				for _, tx := range e.Txs() {
					var txLabel string
					if label, ok := txLabels[tx.Hash()]; ok {
						txLabel = label
					} else {
						to := tx.To()
						if to != nil {
							if toLabel, ok := contracts[*tx.To()]; ok {
								txLabel = toLabel
							}
						}
					}
					log.Info("TX in interesting event block", "nonce", tx.Nonce(), "gas price", tx.GasPrice(), "hash", tx.Hash().Hex(), "to", tx.To(), "label", txLabel)
				}
			}
			if lowestGas != 0 && len(e.Txs()) > 1 {
				prevGasFloor := d.gasFloors[e.Locator().Creator]
				d.gasFloors[e.Locator().Creator] = (prevGasFloor*gasAlpha1 + lowestGas*gasAlpha2) / 100
				d.globalGasFloor = (d.globalGasFloor*gasAlpha1 + lowestGas*gasAlpha2) / 100
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
