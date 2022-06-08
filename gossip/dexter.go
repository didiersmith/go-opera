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

	"github.com/Fantom-foundation/go-opera/contracts/balancer_v2_stable_pool"
	"github.com/Fantom-foundation/go-opera/contracts/balancer_v2_vault"
	"github.com/Fantom-foundation/go-opera/contracts/balancer_v2_weighted_pool"
	"github.com/Fantom-foundation/go-opera/contracts/fish5_lite"
	"github.com/Fantom-foundation/go-opera/contracts/ierc20"
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
	lru "github.com/hashicorp/golang-lru"
)

const (
	numValidators    = 8
	maxShotsPerBlock = 3
	congestedPending = 200
)

var (
	// fishAddr = common.HexToAddress("0xa50B5c30537E000482A041cC2C5C62331739A3aC")
	fishAddr = common.HexToAddress("0x7B78cF4B384A1646B896a009adA1e95F3b3935f3")

	nullAddr                     = common.HexToAddress("0x0000000000000000000000000000000000000000")
	bVaultAddr                   = common.HexToAddress("0x20dd72Ed959b6147912C2e529F0a0C651c33c9ce")
	syncEventTopic               = []byte{0x1c, 0x41, 0x1e, 0x9a, 0x96, 0xe0, 0x71, 0x24, 0x1c, 0x2f, 0x21, 0xf7, 0x72, 0x6b, 0x17, 0xae, 0x89, 0xe3, 0xca, 0xb4, 0xc7, 0x8b, 0xe5, 0x0e, 0x06, 0x2b, 0x03, 0xa9, 0xff, 0xfb, 0xba, 0xd1}
	syncSolidlyEventTopic        = common.FromHex("0xcf2aa50876cdfbb541206f89af0ee78d44a2abf8d328e37fa4917f982149848a")
	swapEventTopic               = common.FromHex("0x2170c741c41531aec20e7c107c24eecfdd15e69c9bb0a8dd37b1840b9e0b207b")
	poolBalanceChangedEventTopic = common.FromHex("0xe5ce249087ce04f05a957192435400fd97868dba0e6a4b4c049abf8af80dae78")
	poolGetReservesAbi           = uniswap_pair_lite.GetReserves()
	poolToken0Abi                = uniswap_pair_lite.Token0()
	poolToken1Abi                = uniswap_pair_lite.Token1()
	decimalsAbi                  = ierc20.Decimals()
	getSwapFeePercentageAbi      = balancer_v2_weighted_pool.GetSwapFeePercentage()
	getAmplificationParameterAbi = balancer_v2_stable_pool.GetAmplificationParameter()
	getNormalizedWeightsAbi      = balancer_v2_weighted_pool.GetNormalizedWeights()
	root                         = "/home/ubuntu/dexter/carb/"
	MaxGasPrice                  = big.NewInt(5000e9)
	accuracyAlpha                = 0.8
	bravadoAlpha                 = 0.75
	gasAlpha                     = 0.5
	defaultValidatorIds          = []idx.ValidatorID{
		17,
		28,
		56,
		57,
		1,
		50,
		10,
		55,
	}
	arbitrageurs = map[common.Address]string{
		common.HexToAddress("0x000000006e983475CD576ae3CCe9caABEc9cF13E"): "Zero chill",
		common.HexToAddress("0x0306c72b69E62be7c7168828cB98Dd41A0B01f8B"): "Flash Boy",
		common.HexToAddress("0x244FAcabcf7a1026849B53295b1F7279a1bD597b"): "Topher Ford",
		common.HexToAddress("0x3BCC05Bc23a6D3223C09F1724bE631c3A2Ff3a77"): "Flipper",
		common.HexToAddress("0x4365C7DD814e1f4b3104B6b5C1079B32b1eA6E8D"): "Sam McGee",
		common.HexToAddress("0x4d339bbDF3f5B67220Eb2B78A7831D6dDcbC74B3"): "Goody two shoes",
		common.HexToAddress("0x4a634281D3C2aF3a21469f2Ba1ad47b59dA8b752"): "Old faithful",
		common.HexToAddress("0x64348f0c72D746b979E1b9d461Ef92dB7a52B6f8"): "Awful Albert",
		common.HexToAddress("0x66Cc927Cb26068E31c6dF653535586B16B6c376b"): "Sixty Six Sisi",
		common.HexToAddress("0x6C080c87e84e71C4c8364128dBE66f5e30a0e370"): "Starving Artist",
		common.HexToAddress("0x6db8d1131caae893e19dd684dcc134109c56aaaa"): "Six Decibel Swinger",
		common.HexToAddress("0x708E6A5E3b0109830cEF2121519849EfB4d8375D"): "Wife Beater",
		common.HexToAddress("0x7536b89f556533200b063e112db947192335a981"): "Broke uni student",
		common.HexToAddress("0x80850B0Cff30EBFDa6cC8cf3b200dD9477C25F34"): "Bob the amateur",
		common.HexToAddress("0xB8A78253Fc10Dac192027c197759da3446357E74"): "Mister Baiter", // todo: Research this guy.
		common.HexToAddress("0xC9A3d59AcbF2A8c856cA5E8c323623Ff11e3aBb2"): "Senile Amy",
		common.HexToAddress("0xD8E1D3608D927d370afFE5EaaF6B050BDc11df6E"): "Daisy with a Reputation",
		common.HexToAddress("0xF501d66f609290D1E6759dFcFA88F869A82fDC97"): "Husky 501",
		common.HexToAddress("0xa775eBB05Aff8c46048e66FF3fc76A53c3801245"): "Creative accountant",
		common.HexToAddress("0xc29aACEa6Fb1D5d1Ebe38D88Cd5C8a8eb397f316"): "New kid on the block",
		common.HexToAddress("0xcB00E3db4c4DCB7B07A2f9A89b6AaBac8E9c6B6D"): "Sea Boo",
		common.HexToAddress("0xd20d27d5cB769522410e0A2f32000C947af7Ffe5"): "Dee Twenny",
		common.HexToAddress("0xd8Fc012498F4278095F10190DD3F29a8A2f16a52"): "Mr Million",
		common.HexToAddress("0xe89ad2d3cc5df094175f2c29d22cfbab5d09e7b7"): "High roller",
		common.HexToAddress("0xf423cdfdB1a54876fAE0515Ec9Cf325c91B537A5"): "Ford Twenty Three",
		common.HexToAddress("0x0f8f339CE002166f32308C091B6e4EecBDdDD2Bd"): "High fiver",
		common.HexToAddress("0x11a2c2bc43c02b5c0893c82d0dcf86a4e9540536"): "Eleven Aces",
		common.HexToAddress("0x50aE81C68B499386065f7E6f89bBE14da64a6C04"): "Short Bus Susan",
		common.HexToAddress("0xedaa29dac8556de71b68440acfce284e755f0bb4"): "Stable Eddie",
		common.HexToAddress("0x063a37c556904f9a5a70d0cba4e5edaba2570753"): "Primitive Pete",
		common.HexToAddress("0x0d1e08ff8513947509162eb8bdb02e6ee892c7f3"): "Lazy Larry",
		common.HexToAddress("0x00F496939f165119eC0bBeaC346508f4a4D5ccAC"): "Egghead",
		common.HexToAddress("0x9e8727b8423a37609a7a03a4e961611b9713ea1f"): "The Humans Are Dead",
	}
	contracts = map[common.Address]string{
		common.HexToAddress("0xba164fB7530b24cF73d183ce0140AF9Ab8C35Cd8"): "Fish3",
		common.HexToAddress("0xBBFc9BD0a39C516182652276Da18eCA50b6bc5d5"): "Fish4",
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
	interestedPairs   map[common.Address]dexter.PoolType
	interestedPools   map[dexter.BalPoolId]dexter.PoolType
	clearIgnoreTxChan chan common.Hash
	eventRaceChan     chan *RaceEntry
	txRaceChan        chan *RaceEntry
	railgunChan       chan *dexter.RailgunPacket
	txLagRequestChan  chan common.Hash
	strategies        []dexter.Strategy
	strategyBravado   []float64
	firedTxChan       chan *FiredTx
	// tokenWhitelistChan chan common.Address
	poolsInfo      map[common.Address]*dexter.PoolInfo
	accuracy       float64
	gasFloors      map[idx.ValidatorID]int64
	globalGasFloor int64
	globalGasPrice int64
	numPending     int
	methodist      *dexter.Methodist
	mu             sync.RWMutex
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
	Hash         common.Hash
	StrategyID   int
	Time         time.Time
	Plan         *dexter.Plan
	TargetMethod dexter.Method
}

type PoolInfoJson struct {
	Addr         string `json:addr`
	Token0       string `json:token0`
	Token1       string `json:token1`
	ExchangeName string `json:exchangeName`
	FeeNumerator int64  `json:feeNumerator`
}

func NewDexter(svc *Service) *Dexter {
	d := &Dexter{
		svc:               svc,
		inTxChan:          make(chan *types.Transaction, 32),
		inLogsChan:        make(chan []*types.Log, 4096),
		inBlockChan:       make(chan evmcore.ChainHeadNotify, 4096),
		inEpochChan:       make(chan idx.Epoch, 4096),
		inEventChan:       make(chan *inter.EventPayload, 4096),
		interestedPairs:   make(map[common.Address]dexter.PoolType),
		interestedPools:   make(map[dexter.BalPoolId]dexter.PoolType),
		lag:               time.Minute * 60,
		signer:            gsignercache.Wrap(types.LatestSignerForChainID(svc.store.GetRules().EvmChainConfig().ChainID)),
		gunRefreshes:      make(chan GunList),
		watchedTxs:        make(chan *TxSub),
		firedTxChan:       make(chan *FiredTx),
		ignoreTxs:         make(map[common.Hash]struct{}),
		clearIgnoreTxChan: make(chan common.Hash, 16),
		eventRaceChan:     make(chan *RaceEntry, 8192),
		txRaceChan:        make(chan *RaceEntry, 8192),
		railgunChan:       make(chan *dexter.RailgunPacket, 8),
		txLagRequestChan:  make(chan common.Hash, 16),
		// tokenWhitelistChan: make(chan common.Address, 128),
		poolsInfo: make(map[common.Address]*dexter.PoolInfo),
		gasFloors: make(map[idx.ValidatorID]int64),
		methodist: dexter.NewMethodist(root, 60*time.Second),
	}
	d.strategies = []dexter.Strategy{

		dexter.NewLinearStrategy("Linear 2-3", 0, d.railgunChan, dexter.LinearStrategyConfig{
			RoutesFileName:          root + "route_caches/solidly_routes_len2-3.json",
			PoolToRouteIdxsFileName: root + "route_caches/solidly_pairToRouteIdxs_len2-3.json",
		}),

		dexter.NewLinearStrategy("Linear sans wftm", 1, d.railgunChan, dexter.LinearStrategyConfig{
			RoutesFileName:          root + "route_caches/solidly_routes_no_wftm_2-3.json",
			PoolToRouteIdxsFileName: root + "route_caches/solidly_pairToRouteIdxs_no_wftm_2-3.json",
		}),

		dexter.NewBalancerLinearStrategy("Balancer Stable", 2, d.railgunChan, dexter.BalancerLinearStrategyConfig{
			RoutesFileName:          root + "route_caches/solidly_balancer_routes_len2-3.json",
			PoolToRouteIdxsFileName: root + "route_caches/solidly_balancer_poolToRouteIdxs_len2-3.json",
		}),

		// dexter.NewLinearStrategy("Linear 2-4", 0, d.railgunChan, dexter.LinearStrategyConfig{
		// 	RoutesFileName:          root + "route_caches/solidly_routes_len2-4.json",
		// 	PoolToRouteIdxsFileName: root + "route_caches/solidly_pairToRouteIdxs_len2-4.json",
		// }),

		// dexter.NewLinearStrategy("Linear sans wftm", 1, d.railgunChan, dexter.LinearStrategyConfig{
		// 	RoutesFileName:          root + "route_caches/solidly_routes_no_wftm_2-4.json",
		// 	PoolToRouteIdxsFileName: root + "route_caches/solidly_pairToRouteIdxs_no_wftm_2-4.json",
		// }),

		// dexter.NewBalancerLinearStrategy("Balancer Stable", 2, d.railgunChan, dexter.BalancerLinearStrategyConfig{
		// 	RoutesFileName:          root + "route_caches/solidly_balancer_routes_len2-4.json",
		// 	PoolToRouteIdxsFileName: root + "route_caches/solidly_balancer_poolToRouteIdxs_len2-4.json",
		// }),
		//
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
		pairs, pools := s.GetInterestedPools()
		for poolAddr, t := range pairs {
			d.interestedPairs[poolAddr] = t
		}
		for poolAddr, t := range pools {
			d.interestedPools[poolAddr] = t
		}
	}

	edgePools := make(map[dexter.EdgeKey][]common.Address)
	for poolAddr, t := range d.interestedPairs {
		poolInfo, ok := d.poolsInfo[poolAddr]
		if !ok {
			log.Warn("Could not find PoolInfo for interested pool", "addr", poolAddr)
			continue
		}
		if t == dexter.UniswapV2Pair || t == dexter.SolidlyVolatilePool || t == dexter.SolidlyStablePool {
			reserve0, reserve1 := d.getReserves(&poolAddr)
			token0, token1 := d.getUniswapPairTokens(&poolAddr)
			d.poolsInfo[poolAddr] = &dexter.PoolInfo{
				Reserves:     map[common.Address]*big.Int{token0: reserve0, token1: reserve1},
				Tokens:       []common.Address{token0, token1},
				FeeNumerator: poolInfo.FeeNumerator,
				LastUpdate:   time.Now(),
			}
			edgeKey := dexter.MakeEdgeKey(token0, token1)
			if pools, ok := edgePools[edgeKey]; ok {
				edgePools[edgeKey] = append(pools, poolAddr)
			} else {
				edgePools[edgeKey] = []common.Address{poolAddr}
			}
		} else {
			log.Error("Interested in pair that is not uniswap pair?!", "addr", poolAddr.Hex())
		}
	}

	for poolId, t := range d.interestedPools {
		var poolAddr common.Address
		copy(poolAddr[:], poolId[:20])
		_, ok := d.poolsInfo[poolAddr]
		if ok {
			log.Error("Duplicate balancer pool addr", "addr", poolAddr, "id", poolId)
			continue
		}
		fee := d.getSwapFeePercentage(&poolAddr)
		poolInfo := dexter.PoolInfo{
			Fee:          fee,
			Reserves:     make(map[common.Address]*big.Int),
			Weights:      make(map[common.Address]*big.Int),
			ScaleFactors: make(map[common.Address]*big.Int),
			LastUpdate:   time.Now(),
			Type:         t,
		}
		if t == dexter.BalancerWeightedPool || t == dexter.BalancerStablePool {
			d.poolsInfo[poolAddr] = &poolInfo
			poolTokens := d.getPoolTokens(poolId)
			// log.Info("Balancer pool tokens", "id", poolId, "tokens", poolTokens.Tokens, "balances", poolTokens.Balances)
			poolInfo.Tokens = poolTokens.Tokens
			for _, tokAddr := range poolInfo.Tokens {
				decimals := d.decimals(&tokAddr)
				decimalsDiff := int64(18 - decimals)
				scaleFactor := new(big.Int).Exp(big.NewInt(10), big.NewInt(decimalsDiff), nil)
				poolInfo.ScaleFactors[tokAddr] = scaleFactor
			}
			if t == dexter.BalancerWeightedPool {
				weights := d.getNormalizedWeights(&poolAddr)
				for i, bal := range poolTokens.Balances {
					poolInfo.Reserves[poolTokens.Tokens[i]] = bal
					poolInfo.Weights[poolTokens.Tokens[i]] = weights[i]
				}
			} else if t == dexter.BalancerStablePool {
				amp := d.getAmplificationParameter(&poolAddr)
				poolInfo.AmplificationParam = amp
				log.Info("Initializing stable pool", "amp", amp, "id", poolId)
				for i, bal := range poolTokens.Balances {
					poolInfo.Reserves[poolTokens.Tokens[i]] = bal
				}
			}
			// log.Info("PoolInfo", "id", poolId, "info", poolInfo)
		}
		d.poolsInfo[poolAddr] = &poolInfo
	}
	log.Info("Min gas price", "minGasPrice", d.svc.store.GetRules().Economy.MinGasPrice)

	for _, s := range d.strategies {
		s.SetPoolsInfo(d.poolsInfo)
		s.SetEdgePools(edgePools)
		s.Start()
	}

	go d.processIncomingLogs()
	go d.processIncomingTxs()
	go d.watchEvents()
	go d.eventRace()
	go d.runRailgun()
	go d.updateMethods()
	svc.handler.RaceEvents(d.eventRaceChan)
	svc.handler.RaceTxs(d.txRaceChan)
	return d
}

type Pool struct {
	Key   string
	Value int
}
type PoolList []Pool

func (p PoolList) Len() int           { return len(p) }
func (p PoolList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p PoolList) Less(i, j int) bool { return p[i].Value > p[j].Value }

func mapToSortedPools(m map[string]int) PoolList {
	pools := make(PoolList, len(m))
	i := 0
	for k, v := range m {
		pools[i] = Pool{k, v}
		i++
	}
	sort.Sort(pools)
	return pools
}

type BytePool struct {
	Key   [4]byte
	Value int
}
type BytePoolList []BytePool

func (p BytePoolList) Len() int           { return len(p) }
func (p BytePoolList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p BytePoolList) Less(i, j int) bool { return p[i].Value > p[j].Value }

func mapToSortedBytePools(m map[[4]byte]int) BytePoolList {
	pools := make(BytePoolList, len(m))
	i := 0
	for k, v := range m {
		pools[i] = BytePool{k, v}
		i++
	}
	sort.Sort(pools)
	return pools
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

			// txWinPools := mapToSortedPools(txWins)
			// for i, winPool := range txWinPools {
			// 	log.Info("tx wins", "i", i, "peer", winPool.Key, "wins", winPool.Value)
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
			eventWinPools := mapToSortedPools(eventWins)
			for _, winPool := range eventWinPools {
				sortedPeers = append(sortedPeers, winPool.Key)
			}
			sortedTxPeers := make([]string, 0, numSortedTxPeers)
			txWinPools := mapToSortedPools(txWins)
			for _, winPool := range txWinPools {
				sortedTxPeers = append(sortedTxPeers, winPool.Key)
				if len(sortedTxPeers) == numSortedTxPeers {
					break
				}
			}
			sortedPeers = append(sortedPeers, losers...)
			// log.Info("Detected winners and losers", "peers", len(peers), "eventWins", len(eventWins), "txWins", len(txWins), "losers", len(losers), "sorted", len(sortedPeers), "sortedTxPeers", len(sortedTxPeers))
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
	poolsFileName := root + "pairs.json"
	poolsFile, err := os.Open(poolsFileName)
	if err != nil {
		log.Info("Error opening pools", "poolsFileName", poolsFileName, "err", err)
		return
	}
	defer poolsFile.Close()
	poolsBytes, _ := ioutil.ReadAll(poolsFile)
	var jsonPools []PoolInfoJson
	json.Unmarshal(poolsBytes, &jsonPools)
	for _, jsonPool := range jsonPools {
		poolAddr := common.HexToAddress(jsonPool.Addr)
		d.poolsInfo[poolAddr] = &dexter.PoolInfo{
			FeeNumerator: big.NewInt(jsonPool.FeeNumerator),
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
func (g GunList) Del(x int) GunList {
	copy(g[x:], g[x+1:])
	g[len(g)-1] = Gun{}
	g = g[:len(g)-1]
	return g
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
	for {
		// FIXME: Convert this to a map[common.Address]PoolUpdate to dedupe.
		var updates []*dexter.PoolUpdate
		select {
		case logs := <-d.inLogsChan:
			var lastTxIndex uint = math.MaxUint64
			for _, l := range logs {
				updated := false
				poolAddr, reserve0, reserve1 := getReservesFromSyncLog(l)
				if poolAddr != nil {
					_, ok := d.interestedPairs[*poolAddr]
					if !ok {
						continue
					}
					token0, token1 := d.getUniswapPairTokens(poolAddr)
					updates = append(updates, &dexter.PoolUpdate{
						Addr:     *poolAddr,
						Reserves: map[common.Address]*big.Int{token0: reserve0, token1: reserve1},
					})
					updated = true
				} else if len(l.Topics) >= 2 && (bytes.Compare(l.Topics[0].Bytes(), swapEventTopic) == 0 || bytes.Compare(l.Topics[0].Bytes(), poolBalanceChangedEventTopic) == 0) {
					var poolId dexter.BalPoolId
					copy(poolId[:], l.Topics[1].Bytes())
					var poolAddr common.Address = common.BytesToAddress(poolId[:20])
					// log.Info("Balancer swap event", "l", l, "poolId", poolId)
					pool := d.getPoolTokens(poolId)
					// log.Info("Balancer updated pool tokens", "pool", pool, "addr", poolAddr)
					u := &dexter.PoolUpdate{
						Addr:     poolAddr,
						Reserves: make(map[common.Address]*big.Int),
					}
					for i, tokAddr := range pool.Tokens {
						u.Reserves[tokAddr] = pool.Balances[i]
					}
					updates = append(updates, u)
					updated = true
				}
				if updated && l.TxIndex != lastTxIndex {
					lastTxIndex = l.TxIndex
					block := d.svc.EthAPI.state.GetBlock(common.Hash{}, l.BlockNumber)
					if block == nil || uint(len(block.Transactions)) <= l.TxIndex {
						continue
					}
					tx := block.Transactions[l.TxIndex]
					data := tx.Data()
					to := tx.To()
					if len(data) >= 4 && to != nil {
						if _, ok := arbitrageurs[*to]; !ok {
							var method [4]byte
							copy(method[:], data[:4])
							d.methodist.Record(dexter.MethodEvent{method, dexter.UpdatedReserves, tx.Hash()})
						}
					}
				}
			}
			if len(updates) > 0 {
				for _, s := range d.strategies {
					s.ProcessPermUpdates(updates)
				}
			}
			// if numUpdates > 100 {
			// 	d.svc.txpool.AddMethods(methods)
			// }
		case <-d.inLogsSub.Err():
			return
		}
	}
	d.inLogsSub.Unsubscribe()
}

func (d *Dexter) processIncomingTxs() {
	log.Info("Started dexter")
	// txCapacity := 1000
	// seenTxs := make(map[common.Hash]struct{}, txCapacity)
	seenTxs, _ := lru.New(4096)
	watchedTxMap := make(map[common.Hash]*FiredTx)
	numFiredThisBlock := 0
	for {
		select {
		case tx := <-d.inTxChan:
			if _, ok := seenTxs.Get(tx.Hash()); ok {
				continue
			}
			seenTxs.Add(tx.Hash(), struct{}{})
			if numFiredThisBlock >= maxShotsPerBlock {
				continue
			}
			if d.lag > 4*time.Second {
				continue
			}
			if _, ok := d.ignoreTxs[tx.Hash()]; ok {
				continue
			}
			// if _, ok := seenTxs[tx.Hash()]; ok {
			// 	continue
			// }
			// if len(seenTxs) > txCapacity {
			// 	seenTxs = make(map[common.Hash]struct{}, txCapacity)
			// }
			// seenTxs[tx.Hash()] = struct{}{}
			// log.Info("Dexter received tx", "hash", tx.Hash().Hex(), "queue", len(d.inTxChan))
			d.processTx(tx)
			d.numPending, _ = d.svc.txpool.Stats()
		case n := <-d.inBlockChan:
			numFiredThisBlock = 0
			d.lag = time.Now().Sub(n.Block.Time.Time())
			if d.lag < 4*time.Second {
				go d.refreshGuns()
			}
			for strategyID, bravado := range d.strategyBravado {
				d.strategyBravado[strategyID] = math.Min(1.0, bravado+0.005)
				d.strategies[strategyID].SetGasPrice(d.globalGasPrice)
			}
			receipts, err := d.svc.EthAPI.GetReceipts(context.TODO(), n.Block.Hash)
			if err != nil {
				log.Error("Could not get block receipts", "err", err)
				continue
			}
			for i, tx := range n.Block.Transactions {
				data := tx.Data()
				if len(data) < 4 {
					continue
				}
				var method [4]byte
				copy(method[:], data[:4])
				if d.methodist.Interested(method) {
					d.methodist.Record(dexter.MethodEvent{method, dexter.Confirmed, tx.Hash()})
				}
				if f, ok := watchedTxMap[tx.Hash()]; ok {
					receipt := receipts[i]
					gasEst := new(big.Int).Div(f.Plan.GasCost, f.Plan.GasPrice)
					if receipt.Status == types.ReceiptStatusSuccessful {
						d.methodist.Record(dexter.MethodEvent{f.TargetMethod, dexter.Success, tx.Hash()})

						d.strategyBravado[f.StrategyID] = d.strategyBravado[f.StrategyID]*bravadoAlpha + (1 - bravadoAlpha)
						log.Info("SUCCESS", "tx", tx.Hash().Hex(), "strategy", f.StrategyID, "new bravado", d.strategyBravado[f.StrategyID], "lag", utils.PrettyDuration(time.Now().Sub(f.Time)), "gas", receipt.GasUsed, "estimated", gasEst, "est profit", dexter.BigIntToFloat(f.Plan.NetProfit)/1e18)
					} else {
						// d.diagnoseTx(tx, f)
						d.strategyBravado[f.StrategyID] = d.strategyBravado[f.StrategyID] * bravadoAlpha
						log.Info("Fail", "tx", tx.Hash().Hex(), "strategy", f.StrategyID, "new bravado", d.strategyBravado[f.StrategyID], "lag", utils.PrettyDuration(time.Now().Sub(f.Time)), "gas", receipt.GasUsed, "estimated", gasEst, "est profit", dexter.BigIntToFloat(f.Plan.NetProfit)/1e18)
					}
					delete(watchedTxMap, tx.Hash())
				}
			}
		case <-d.inBlockSub.Err():
			return
		case f := <-d.firedTxChan:
			watchedTxMap[f.Hash] = f
			numFiredThisBlock++
		case h := <-d.clearIgnoreTxChan:
			delete(d.ignoreTxs, h)

		}
	}
	d.inLogsSub.Unsubscribe()
}

func (d *Dexter) processTx(tx *types.Transaction) {
	start := time.Now()
	from, _ := types.Sender(d.signer, tx)
	validators, epoch := d.svc.store.GetEpochValidators() // TODO: Move this up?
	validatorIDs := d.predictValidators(from, tx.Nonce(), validators, epoch, numValidators)
	d.watchedTxs <- &TxSub{Hash: tx.Hash(), PredictedValidators: validatorIDs, Print: false, StartTime: start}
	data := tx.Data()
	if len(data) < 4 {
		return
	}
	var method [4]byte
	copy(method[:], data[:4])
	d.methodist.Record(dexter.MethodEvent{method, dexter.Pending, tx.Hash()})
	// if tx.GasPrice().Cmp(MaxGasPrice) == 1 {
	// 	return
	// }
	// if d.numPending > congestedPending {
	// 	if gasFloor, ok := d.gasFloors[validatorIDs[0]]; ok {
	// 		if tx.GasPrice().Int64() < (gasFloor+d.globalGasFloor)/2 {
	// 			return
	// 		}
	// 	}
	// }
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
		StartTime:    start,
	}
	var updatedPools []dexter.BalPoolId
	evmProcessor.Process(evmBlock, statedb, opera.DefaultVMConfig, &gasUsed, false, func(l *types.Log, _ *state.StateDB) {
		poolAddr, reserve0, reserve1 := getReservesFromSyncLog(l)
		if poolAddr != nil {
			if _, ok := d.interestedPairs[*poolAddr]; ok {
				token0, token1 := d.getUniswapPairTokens(poolAddr)
				ptx.Updates = append(ptx.Updates, dexter.PoolUpdate{
					Addr:     *poolAddr,
					Reserves: map[common.Address]*big.Int{token0: reserve0, token1: reserve1},
				})
			}
		} else if len(l.Topics) >= 2 && (bytes.Compare(l.Topics[0].Bytes(), swapEventTopic) == 0 || bytes.Compare(l.Topics[0].Bytes(), poolBalanceChangedEventTopic) == 0) {
			var poolId dexter.BalPoolId
			copy(poolId[:], l.Topics[1].Bytes())
			updatedPools = append(updatedPools, poolId)
			// log.Info("Balancer swap event", "topics", l.Topics, "l", l.Data)
		}
	})
	if len(ptx.Updates) == 0 && len(updatedPools) == 0 {
		return
	}
	for _, poolId := range updatedPools {
		evm := d.getEvm(statedb, evmStateReader, bs)
		var poolAddr common.Address = common.BytesToAddress(poolId[:20])
		pool := d.getPoolTokensFromEvm(poolId, evm)
		u := dexter.PoolUpdate{
			Addr:     poolAddr,
			Reserves: make(map[common.Address]*big.Int),
		}
		for i, tokAddr := range pool.Tokens {
			u.Reserves[tokAddr] = pool.Balances[i]
		}
		ptx.Updates = append(ptx.Updates, u)
	}
	for _, s := range d.strategies {
		s.ProcessPossibleTx(ptx)
	}
}

// func (d *Dexter) processEvent(txs []*types.Transaction) {
// 	start := time.Now()
// 	bs := d.svc.store.GetBlockState().Copy()
// 	evmStateReader := &EvmStateReader{
// 		ServiceFeed: &d.svc.feed,
// 		store:       d.svc.store,
// 	}
// 	statedb, err := d.svc.store.evm.StateDB(bs.FinalizedStateRoot)
// 	if err != nil {
// 		log.Info("Could not make StateDB", "err", err)
// 		return
// 	}
// 	evmProcessor := evmcore.NewStateProcessor(
// 		d.svc.store.GetRules().EvmChainConfig(),
// 		evmStateReader)
// 	evmBlock := d.evmBlockWith(txs, &bs, evmStateReader)
// 	var gasUsed uint64
// 	ptx := &dexter.PossibleTx{
// 		Tx:           tx,
// 		ValidatorIDs: validatorIDs,
// 		StartTime:    start,
// 	}
// 	var updatedPools []dexter.BalPoolId
// 	evmProcessor.Process(evmBlock, statedb, opera.DefaultVMConfig, &gasUsed, false, func(l *types.Log, _ *state.StateDB) {
// 		poolAddr, reserve0, reserve1 := getReservesFromSyncLog(l)
// 		if poolAddr != nil {
// 			if _, ok := d.interestedPairs[*poolAddr]; ok {
// 				token0, token1 := d.getUniswapPairTokens(poolAddr)
// 				ptx.Updates = append(ptx.Updates, dexter.PoolUpdate{
// 					Addr:     *poolAddr,
// 					Reserves: map[common.Address]*big.Int{token0: reserve0, token1: reserve1},
// 				})
// 			}
// 		} else if len(l.Topics) >= 2 && (bytes.Compare(l.Topics[0].Bytes(), swapEventTopic) == 0 || bytes.Compare(l.Topics[0].Bytes(), poolBalanceChangedEventTopic) == 0) {
// 			var poolId dexter.BalPoolId
// 			copy(poolId[:], l.Topics[1].Bytes())
// 			updatedPools = append(updatedPools, poolId)
// 			// log.Info("Balancer swap event", "topics", l.Topics, "l", l.Data)
// 		}
// 	})
// 	if len(ptx.Updates) == 0 && len(updatedPools) == 0 {
// 		return
// 	}
// 	for _, poolId := range updatedPools {
// 		evm := d.getEvm(statedb, evmStateReader, bs)
// 		var poolAddr common.Address = common.BytesToAddress(poolId[:20])
// 		pool := d.getPoolTokensFromEvm(poolId, evm)
// 		u := dexter.PoolUpdate{
// 			Addr:     poolAddr,
// 			Reserves: make(map[common.Address]*big.Int),
// 		}
// 		for i, tokAddr := range pool.Tokens {
// 			u.Reserves[tokAddr] = pool.Balances[i]
// 		}
// 		ptx.Updates = append(ptx.Updates, u)
// 	}
// 	for _, s := range d.strategies {
// 		s.ProcessPossibleTx(ptx)
// 	}
// }

func getReservesFromSyncLog(l *types.Log) (*common.Address, *big.Int, *big.Int) {
	if len(l.Topics) != 1 || (bytes.Compare(l.Topics[0].Bytes(), syncEventTopic) != 0 && bytes.Compare(l.Topics[0].Bytes(), syncSolidlyEventTopic) != 0) {
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
			// log.Info("Received railgun request", "strategy", p.StrategyID)
			// deadline := time.After(5 * time.Millisecond)
			// loop:
			// for {
			// 	select {
			// 	case p2 := <-d.railgunChan:
			// 		if p2.Response.NetProfit.Cmp(p.Response.NetProfit) > 0 {
			// 			log.Info("Overriding plan with more profitable plan", "strategy1", p.StrategyID, "strategy2", p2.StrategyID, "profit1", p.Response.NetProfit, "profit2", p2.Response.NetProfit)
			// 			p = p2
			// 		}
			// 	case <-deadline:
			// 		break loop
			// 	}
			// }
			bravado := d.strategyBravado[p.StrategyID]
			probAdjustedPayoff := new(big.Int).Mul(p.Response.NetProfit, big.NewInt(int64(d.accuracy*bravado)))
			failCost := new(big.Int).Mul(p.Response.GasPrice, big.NewInt(dexter.GAS_FAIL))
			probAdjustedFailCost := new(big.Int).Mul(failCost, big.NewInt(int64(1e6-(d.accuracy*bravado))))
			if probAdjustedPayoff.Cmp(probAdjustedFailCost) == -1 {
				// log.Info("Trade unprofitable after adjusting for accuracy", "accuracy", d.accuracy, "bravado", bravado, "probAdjustedPayoff", dexter.BigIntToFloat(probAdjustedPayoff)/1e18, "lose", dexter.BigIntToFloat(probAdjustedFailCost)/1e18, "accuracy", d.accuracy, "bravado", bravado, "unadjusted win", dexter.BigIntToFloat(p.Response.NetProfit)/1e18, "unadjusted lose", dexter.BigIntToFloat(failCost)/1e18)
				continue
			}
			validatorIDs := p.ValidatorIDs
			if len(validatorIDs) == 0 {
				validatorIDs = defaultValidatorIds
			}
			gunIdx := d.gunList.Search(validatorIDs)
			if gunIdx < 0 {
				log.Warn("Gun list not initialized")
				continue
			}
			gun := d.gunList[gunIdx] // TODO: Prevent gun reuse with multiple strategies
			if gun.ValidatorIDs[0] != validatorIDs[0] {
				log.Warn("Could not find validator", "validator", validatorIDs[0])
				continue
			}
			d.gunList = d.gunList.Del(gunIdx)
			wallet := gun.Wallet
			account := wallet.Accounts()[0]
			nonce := d.svc.txpool.Nonce(account.Address)
			var fishCall []byte
			if p.Type == dexter.SwapSinglePath {
				fishCall = fish5_lite.SwapLinear(p.Response.AmountIn, p.Response.MinProfit, p.Response.Path, fishAddr)
				// fishCall = fish4_lite.SwapLinear(big.NewInt(0), p.Response.MinProfit, p.Response.Path, fishAddr)
			} else {
				log.Error("Unsupported call type", "type", p.Type)
				continue
			}
			gas := uint64(dexter.BigIntToFloat(p.Response.GasCost) / dexter.BigIntToFloat(p.Response.GasPrice) * 1.3)
			var fishTx *types.Transaction
			if p.Target.Type() == types.LegacyTxType {
				fishTx = types.NewTransaction(nonce, fishAddr, common.Big0, gas, p.Response.GasPrice, fishCall)
			} else {
				fishTx = types.NewTx(&types.DynamicFeeTx{
					ChainID:   p.Target.ChainId(),
					Nonce:     nonce,
					GasTipCap: p.Target.GasTipCap(),
					GasFeeCap: p.Target.GasFeeCap(),
					Gas:       gas,
					To:        &fishAddr,
					Value:     common.Big0,
					Data:      fishCall,
				})
			}
			signedTx, err := wallet.SignTx(account, fishTx, d.svc.store.GetRules().EvmChainConfig().ChainID)
			if err != nil {
				log.Error("Could not sign tx", "err", err)
				return
			}
			var label string
			if p.Target != nil {
				label = p.Target.Hash().Hex()
				d.ignoreTxs[p.Target.Hash()] = struct{}{}
			} else {
				label = "block state"
			}
			log.Info("FIRING GUN pew pew",
				"lag", utils.PrettyDuration(time.Now().Sub(p.StartTime)),
				"total", utils.PrettyDuration(time.Now().Sub(p.Target.Time())),
				"hash", signedTx.Hash().Hex(), "strategy", p.StrategyID, "gas", p.Response.GasPrice)
			// log.Info("Fired tx", "url", "https://ftmscan.com/tx/"+signedTx.Hash().Hex())
			go d.fireGun(wallet, signedTx, p, gun.ValidatorIDs, label)
		}
	}
}

func (d *Dexter) fireGun(wallet accounts.Wallet, signedTx *types.Transaction, p *dexter.RailgunPacket, gunValidatorIDs []idx.ValidatorID, label string) {
	d.svc.handler.BroadcastTxsAggressive([]*types.Transaction{signedTx})
	d.svc.txpool.AddLocal(signedTx)
	if p.Target != nil {
		d.watchedTxs <- &TxSub{
			Hash:                p.Target.Hash(),
			Label:               "target",
			PredictedValidators: p.ValidatorIDs,
			Print:               true,
			StartTime:           time.Now(),
		}
	}
	d.watchedTxs <- &TxSub{
		Hash:                signedTx.Hash(),
		Label:               label,
		PredictedValidators: gunValidatorIDs,
		TargetValidators:    p.ValidatorIDs,
		Print:               true,
		StartTime:           time.Now(),
	}
	var method [4]byte
	copy(method[:], p.Target.Data()[:4])
	d.methodist.Record(dexter.MethodEvent{method, dexter.Fired, p.Target.Hash()})
	d.firedTxChan <- &FiredTx{
		Hash:         signedTx.Hash(),
		StrategyID:   p.StrategyID,
		Time:         time.Now(),
		Plan:         p.Response,
		TargetMethod: method,
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
			interestedGas := make(map[int64]struct{})
			txLabels := make(map[common.Hash]string)
			var lowestGas int64 = 0
			var highestGas int64 = 0
			for _, tx := range e.Txs() {
				if highestGas == 0 {
					highestGas = tx.GasPrice().Int64()
				}
				lowestGas = tx.GasPrice().Int64()
				if sub, ok := watchedTxMap[tx.Hash()]; ok {
					if e.Locator().Creator == sub.PredictedValidators[0] {
						d.accuracy = d.accuracy*accuracyAlpha + 1e6*(1-accuracyAlpha)
					} else {
						d.accuracy = d.accuracy * accuracyAlpha
					}
					if sub.Print {
						interested = true
						interestedGas[tx.GasPrice().Int64()] = struct{}{}
						// log.Info(msg, "id", e.ID(), "lamport", e.Locator().Lamport, "creator", e.Locator().Creator, "predicted", sub.PredictedValidators, "target", sub.TargetValidators, "lag", utils.PrettyDuration(time.Now().Sub(sub.StartTime)), "tx", tx.Hash().Hex(), "label", sub.Label)
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
					if _, ok := interestedGas[tx.GasPrice().Int64()]; !ok {
						continue
					}
					var txLabel string
					if label, ok := txLabels[tx.Hash()]; ok {
						txLabel = label
					} else {
						to := tx.To()
						if to != nil {
							if toLabel, ok := contracts[*tx.To()]; ok {
								txLabel = toLabel
							} else if toLabel, ok := arbitrageurs[*tx.To()]; ok {
								txLabel = "A:" + toLabel
							}
						}
					}
					effectiveGasTip, err := tx.EffectiveGasTip(d.svc.store.GetRules().Economy.MinGasPrice)
					if err != nil {
						log.Error("Could not get effective gas tip", "tx", tx, "err", err)
					}
					from, _ := types.Sender(d.signer, tx)
					log.Info("TX in event block", "gasTip", effectiveGasTip, "size", tx.Size(), "from", from, "hash", tx.Hash().Hex(), "to", tx.To(), "label", txLabel)
				}
			}
			if lowestGas != 0 && len(e.Txs()) > 1 {
				prevGasFloor := d.gasFloors[e.Locator().Creator]
				d.gasFloors[e.Locator().Creator] = (prevGasFloor*gasAlpha1 + lowestGas*gasAlpha2) / 100
				d.globalGasFloor = (d.globalGasFloor*gasAlpha1 + lowestGas*gasAlpha2) / 100
				gasPrice := (2*highestGas + lowestGas) / 3
				d.globalGasPrice = (d.globalGasPrice*gasAlpha1 + gasPrice*gasAlpha2) / 100
			}
		case w := <-d.watchedTxs:
			watchedTxMap[w.Hash] = w
		}
	}
}

func (d *Dexter) getReserve(poolType dexter.PoolType, poolId dexter.BalPoolId, token common.Address) *big.Int {
	if poolType == dexter.UniswapV2Pair || poolType == dexter.SolidlyVolatilePool || poolType == dexter.SolidlyStablePool {
		poolAddr := common.BytesToAddress(poolId[:20])
		token0, _ := d.getUniswapPairTokens(&poolAddr)
		reserve0, reserve1 := d.getReserves(&poolAddr)
		if bytes.Compare(token0.Bytes(), token.Bytes()) == 0 {
			return reserve0
		}
		return reserve1
	} else {
		pool := d.getPoolTokens(poolId)
		for i, candTok := range pool.Tokens {
			if bytes.Compare(candTok.Bytes(), token.Bytes()) == 0 {
				return pool.Balances[i]
			}
		}
	}
	return big.NewInt(0)
}
func (d *Dexter) diagnoseTx(tx *types.Transaction, f *FiredTx) {
	for _, reserve := range f.Plan.Reserves {
		reserve.Actual = d.getReserve(reserve.Type, reserve.PoolId, reserve.Token)
		actualFloat := dexter.BigIntToFloat(reserve.Actual)
		log.Info("reserve", "pool",
			common.Bytes2Hex([]byte(reserve.PoolId[:])),
			"tok", reserve.Token,
			"o", reserve.Original,
			"p", reserve.Predicted,
			"a", reserve.Actual,
			"diff", new(big.Int).Sub(reserve.Actual, reserve.Predicted),
			"%", (100 * (actualFloat - dexter.BigIntToFloat(reserve.Predicted)) / actualFloat),
		)
	}
}

func (d *Dexter) getReserves(addr *common.Address) (*big.Int, *big.Int) {
	evm := d.getReadOnlyEvm()
	msg := d.readOnlyMessage(addr, poolGetReservesAbi)
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

func (d *Dexter) getUniswapPairTokens(addr *common.Address) (common.Address, common.Address) {
	evm := d.getReadOnlyEvm()
	msg0 := d.readOnlyMessage(addr, poolToken0Abi)
	gp0 := new(evmcore.GasPool).AddGas(math.MaxUint64)
	result0, err := evmcore.ApplyMessage(evm, msg0, gp0)
	if err != nil {
		log.Info("token0 error", "err", err)
		return common.Address{}, common.Address{}
	}
	token0 := common.BytesToAddress(result0.ReturnData)
	msg1 := d.readOnlyMessage(addr, poolToken1Abi)
	gp1 := new(evmcore.GasPool).AddGas(math.MaxUint64)
	result1, err := evmcore.ApplyMessage(evm, msg1, gp1)
	if err != nil {
		log.Info("token1 error", "err", err)
		return common.Address{}, common.Address{}
	}
	token1 := common.BytesToAddress(result1.ReturnData)
	return token0, token1
}

func (d *Dexter) getSwapFeePercentage(addr *common.Address) *big.Int {
	evm := d.getReadOnlyEvm()
	msg := d.readOnlyMessage(addr, getSwapFeePercentageAbi)
	gp := new(evmcore.GasPool).AddGas(math.MaxUint64)
	result, err := evmcore.ApplyMessage(evm, msg, gp)
	if err != nil {
		log.Error("getSwapFeePercentage error", "err", err)
		return nil
	}
	fee := new(big.Int).SetBytes(result.ReturnData[:32])
	return fee
}

func (d *Dexter) getAmplificationParameter(addr *common.Address) *big.Int {
	evm := d.getReadOnlyEvm()
	msg := d.readOnlyMessage(addr, getAmplificationParameterAbi)
	gp := new(evmcore.GasPool).AddGas(math.MaxUint64)
	result, err := evmcore.ApplyMessage(evm, msg, gp)
	if err != nil {
		log.Error("getAmplificationParameter error", "err", err)
		return nil
	}
	amp := new(big.Int).SetBytes(result.ReturnData[:32])
	return amp
}

func (d *Dexter) getNormalizedWeights(addr *common.Address) []*big.Int {
	evm := d.getReadOnlyEvm()
	msg := d.readOnlyMessage(addr, getNormalizedWeightsAbi)
	gp := new(evmcore.GasPool).AddGas(math.MaxUint64)
	result, err := evmcore.ApplyMessage(evm, msg, gp)
	if err != nil {
		log.Error("getNormalizedWeights error", "err", err)
		return nil
	}
	return balancer_v2_weighted_pool.UnpackGetNormalizedWeights(result.ReturnData)
}

func (d *Dexter) decimals(addr *common.Address) uint8 {
	evm := d.getReadOnlyEvm()
	msg := d.readOnlyMessage(addr, decimalsAbi)
	gp := new(evmcore.GasPool).AddGas(math.MaxUint64)
	result, err := evmcore.ApplyMessage(evm, msg, gp)
	if err != nil {
		log.Error("decimals error", "err", err)
		return 18
	}
	return uint8(result.ReturnData[31])
}

func (d *Dexter) getPoolTokens(poolId dexter.BalPoolId) struct {
	Tokens          []common.Address
	Balances        []*big.Int
	LastChangeBlock *big.Int
} {
	evm := d.getReadOnlyEvm()
	return d.getPoolTokensFromEvm(poolId, evm)
}

func (d *Dexter) getPoolTokensFromEvm(poolId dexter.BalPoolId, evm *vm.EVM) struct {
	Tokens          []common.Address
	Balances        []*big.Int
	LastChangeBlock *big.Int
} {
	msg := d.readOnlyMessage(&bVaultAddr, balancer_v2_vault.GetPoolTokens(poolId))
	gp := new(evmcore.GasPool).AddGas(math.MaxUint64)
	result, err := evmcore.ApplyMessage(evm, msg, gp)
	if err != nil {
		log.Error("getPoolTokens error", "err", err)
	}
	// log.Info("getPoolTokens", "addr", bVaultAddr, "poolId", poolId, "result", result.ReturnData)
	return balancer_v2_vault.UnpackGetPoolTokens(result.ReturnData)
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
	return d.getEvm(statedb, evmStateReader, bs)
}

func (d *Dexter) getEvm(statedb *state.StateDB, evmStateReader *EvmStateReader, bs iblockproc.BlockState) *vm.EVM {
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

func (d *Dexter) updateMethods() {
	for {
		time.Sleep(60 * time.Second)
		white, black := d.methodist.GetLists()
		d.svc.txpool.UpdateMethods(white, black)
	}
}

// func (d *Dexter) runTokenWhitelister() {
// 	whitelist := make(map[common.Address]int)
// 	for i := 0; ; i++ {
// 		addr := <-d.tokenWhitelistChan
// 		if cnt, ok := whitelist[addr]; ok {
// 			whitelist[addr] = cnt + 1
// 		} else {
// 			whitelist[addr] = cnt + 1
// 		}
// 		if i%1000 == 0 {
// 			whitelistJson := make(map[string]int)
// 			for addr, cnt := range whitelist {
// 				whitelistJson[addr.Hex()] = cnt
// 			}
// 			log.Info("Dumping token whitelist", "len1", len(whitelist), "len2", len(whitelistJson))
// 			file, err := os.OpenFile(root+"/data/arbitrageur_token_whitelist.json", os.O_CREATE|os.O_WRONLY, os.ModePerm)
// 			if err != nil {
// 				log.Error("Could not open arbitrageur whitelist file", "err", err)
// 				continue
// 			}
// 			encoder := json.NewEncoder(file)
// 			err = encoder.Encode(whitelistJson)
// 			if err != nil {
// 				log.Error("JSON encoding error", "err", err)
// 			}
// 			file.Close()
// 		}
// 	}
// }
