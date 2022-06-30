package gossip

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"math/big"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Fantom-foundation/go-opera/contracts/balancer_v2_stable_pool"
	"github.com/Fantom-foundation/go-opera/contracts/balancer_v2_vault"
	"github.com/Fantom-foundation/go-opera/contracts/balancer_v2_weighted_pool"
	"github.com/Fantom-foundation/go-opera/contracts/curve_factory"
	"github.com/Fantom-foundation/go-opera/contracts/curve_registry"
	"github.com/Fantom-foundation/go-opera/contracts/fish8_lite"
	"github.com/Fantom-foundation/go-opera/contracts/hansel_lite"
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
	"github.com/ethereum/go-ethereum/p2p/enode"
	lru "github.com/hashicorp/golang-lru"
)

const (
	numValidators    = 3
	maxShotsPerBlock = 3
	congestedPending = 200
	GAS_HANSEL       = 400000
)

var (
	// fishAddr         = common.HexToAddress("0x58b9fC2F82f07E67296f0d30124e4378616e8295")
	// fishAddr = common.HexToAddress("0xa50B5c30537E000482A041cC2C5C62331739A3aC")
	fishAddr         = common.HexToAddress("0xD4e83785632Ecb6c7b2ad8267C40d08CF1453391")
	hanselAddr       = common.HexToAddress("0x0a81c8e5c85D8bACFe9b038c6F7fC2C5186C47B3")
	hanselSearchAddr = common.HexToAddress("0xDFc41cC7D14F39e15DEc0CF959a9A0DA8F9C3921")

	nullAddr                     = common.HexToAddress("0x0000000000000000000000000000000000000000")
	ownerAddr                    = common.HexToAddress("0x1fb4820c368EFA3282e696CA9AAed9C3Cade2340")
	bVaultAddr                   = common.HexToAddress("0x20dd72Ed959b6147912C2e529F0a0C651c33c9ce")
	cRegistryAddr                = common.HexToAddress("0x0f854EA9F38ceA4B1c2FC79047E9D0134419D5d6")
	cFactoryAddr                 = common.HexToAddress("0x686d67265703D1f124c45E33d47d794c566889Ba")
	syncEventTopic               = []byte{0x1c, 0x41, 0x1e, 0x9a, 0x96, 0xe0, 0x71, 0x24, 0x1c, 0x2f, 0x21, 0xf7, 0x72, 0x6b, 0x17, 0xae, 0x89, 0xe3, 0xca, 0xb4, 0xc7, 0x8b, 0xe5, 0x0e, 0x06, 0x2b, 0x03, 0xa9, 0xff, 0xfb, 0xba, 0xd1}
	syncSolidlyEventTopic        = common.FromHex("0xcf2aa50876cdfbb541206f89af0ee78d44a2abf8d328e37fa4917f982149848a")
	swapUniswapEventTopic        = common.FromHex("0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822")
	swapBalancerEventTopic       = common.FromHex("0x2170c741c41531aec20e7c107c24eecfdd15e69c9bb0a8dd37b1840b9e0b207b")
	poolBalanceChangedEventTopic = common.FromHex("0xe5ce249087ce04f05a957192435400fd97868dba0e6a4b4c049abf8af80dae78")
	curveEventTopic              = map[[32]byte]struct{}{
		// TokenExchange event
		[32]byte{0x8b, 0x3e, 0x96, 0xf2, 0xb8, 0x89, 0xfa, 0x77, 0x1c, 0x53, 0xc9, 0x81, 0xb4, 0x0d, 0xaf, 0x00, 0x5f, 0x63, 0xf6, 0x37, 0xf1, 0x86, 0x9f, 0x70, 0x70, 0x52, 0xd1, 0x5a, 0x3d, 0xd9, 0x71, 0x40}: struct{}{},
		// Addliquidity events
		[32]byte{0x26, 0xf5, 0x5a, 0x85, 0x08, 0x1d, 0x24, 0x97, 0x4e, 0x85, 0xc6, 0xc0, 0x00, 0x45, 0xd0, 0xf0, 0x45, 0x39, 0x91, 0xe9, 0x58, 0x73, 0xf5, 0x2b, 0xff, 0x0d, 0x21, 0xaf, 0x40, 0x79, 0xa7, 0x68}: struct{}{},
		[32]byte{0x42, 0x3f, 0x64, 0x95, 0xa0, 0x8f, 0xc6, 0x52, 0x42, 0x5c, 0xf4, 0xed, 0x0d, 0x1f, 0x9e, 0x37, 0xe5, 0x71, 0xd9, 0xb9, 0x52, 0x9b, 0x1c, 0x1c, 0x23, 0xcc, 0xe7, 0x80, 0xb2, 0xe7, 0xdf, 0x0d}: struct{}{},
		[32]byte{0x3f, 0x19, 0x15, 0x77, 0x5e, 0x0c, 0x9a, 0x38, 0xa5, 0x7a, 0x7b, 0xb7, 0xf1, 0xf9, 0x00, 0x5f, 0x48, 0x6f, 0xb9, 0x04, 0xe1, 0xf8, 0x4a, 0xa2, 0x15, 0x36, 0x4d, 0x56, 0x73, 0x19, 0xa5, 0x8d}: struct{}{},
		// RemoveLiquidity events
		[32]byte{0x7c, 0x36, 0x38, 0x54, 0xcc, 0xf7, 0x96, 0x23, 0x41, 0x1f, 0x89, 0x95, 0xb3, 0x62, 0xbc, 0xe5, 0xed, 0xdf, 0xf1, 0x8c, 0x92, 0x7e, 0xdc, 0x6f, 0x5d, 0xbb, 0xb5, 0xe0, 0x58, 0x19, 0xa8, 0x2c}: struct{}{},
		[32]byte{0xa4, 0x9d, 0x4c, 0xf0, 0x26, 0x56, 0xae, 0xbf, 0x8c, 0x77, 0x1f, 0x5a, 0x85, 0x85, 0x63, 0x8a, 0x2a, 0x15, 0xee, 0x6c, 0x97, 0xcf, 0x72, 0x05, 0xd4, 0x20, 0x8e, 0xd7, 0xc1, 0xdf, 0x25, 0x2d}: struct{}{},
		[32]byte{0x98, 0x78, 0xca, 0x37, 0x5e, 0x10, 0x6f, 0x2a, 0x43, 0xc3, 0xb5, 0x99, 0xfc, 0x62, 0x45, 0x68, 0x13, 0x1c, 0x4c, 0x9a, 0x4b, 0xa6, 0x6a, 0x14, 0x56, 0x37, 0x15, 0x76, 0x3b, 0xe9, 0xd5, 0x9d}: struct{}{},
		// RemoveLiquidityOne events
		[32]byte{0x5a, 0xd0, 0x56, 0xf2, 0xe2, 0x8a, 0x8c, 0xec, 0x23, 0x20, 0x15, 0x40, 0x6b, 0x84, 0x36, 0x68, 0xc1, 0xe3, 0x6c, 0xda, 0x59, 0x81, 0x27, 0xec, 0x3b, 0x8c, 0x59, 0xb8, 0xc7, 0x27, 0x73, 0xa0}: struct{}{},
		[32]byte{}: struct{}{},
	}
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
	mttsAlpha                    = 0.1
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
		fishAddr:   "Fish",
		hanselAddr: "Hansel",
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
		common.HexToAddress("0xa0677656E1939fc53244B6e383164e3Ac2a1eEed"): "Alexandra Occasional Cortex",
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
		common.HexToAddress("0x8Ec7DE3e664F3e64d6bB6f017911fb609e7D9B70"): "Fiat Panda",
		common.HexToAddress("0xa517750EB8D93bA37F6595773153ea9Ec7aE0AF8"): "Die hard try hard",
		common.HexToAddress("0xcAAe8702B51478E89ff69436106C00446e4f557F"): "Calcaholics anonymous",
		common.HexToAddress("0x1713E0F1010F8a3fD2629dCaA25cd96939F5E5f8"): "Bianca Castafiore",
		common.HexToAddress("0x2162D7E1FE3C29d14be28c5B78E3B6264006e806"): "Twenty-one Pilots",
		common.HexToAddress("0x3952a14252A9a7ae93017C10146BAD7110a526dd"): "Farmer Frank",
		common.HexToAddress("0x8b39ffc41b82ef37bfd97d9ea390cd19b437a22c"): "Blue Chip Buster",
		common.HexToAddress("0x4992a76c6835f03afe73a20cec8445336daea05a"): "Nein nein nein",
		common.HexToAddress("0x7adfF61E398a979F8eD618984a6e2FB7cfa5b54E"): "Seven Army",
		common.HexToAddress("0xe639d78bd9281ebeb3f0bef25e1fea95835dea57"): "Monosodium Glutamate",
		common.HexToAddress("0x30cE95DD82fE8EeF226222eC653a7B14A3222103"): "Try hard Tony",
		common.HexToAddress("0x5e6d798717f09cb56aff739eed30bf69206d640f"): "Billy Goat Gruff",
		common.HexToAddress("0xEC4A27C437722749be32a107F97eC2E628b571Ee"): "Easy Peasy",
		common.HexToAddress("0x4AC549FF9b50aEb047Ca9D77a5cC569f4Db5fB4E"): "Fancy Pants",
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
	startTokensIn = new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
)

type Dexter struct {
	nodeType           NodeType
	svc                *Service
	inTxChan           chan *dexter.TxWithTimeLog
	inFriendlyFireChan chan *types.Transaction
	inLogsChan         chan []*types.Log
	inLogsSub          notify.Subscription
	inBlockChan        chan evmcore.ChainHeadNotify
	inBlockSub         notify.Subscription
	inEpochChan        chan idx.Epoch
	inEpochSub         notify.Subscription
	inEventChan        chan *inter.EventPayload
	inEventSub         notify.Subscription
	lag                time.Duration
	gunRefreshes       chan GunList
	gunLastFired       map[common.Address]time.Time
	signer             types.Signer
	gunList            GunList
	watchedTxs         chan *TxSub
	// ignoreTxs         map[common.Hash]struct{}
	interestedPairs   map[common.Address]dexter.PoolType
	interestedPools   map[dexter.BalPoolId]dexter.PoolType
	clearIgnoreTxChan chan common.Hash
	txRaceChan        chan *RaceEntry
	railgunChan       chan *dexter.RailgunPacket
	strategies        []dexter.Strategy
	strategyBravado   []float64
	mtts              []time.Duration
	numFired          []int64
	numFiredThisBlock int
	firedTxChan       chan *FiredTx
	// tokenWhitelistChan chan common.Address
	poolsInfo                map[common.Address]*dexter.PoolInfo
	accuracy                 float64
	gasFloors                map[idx.ValidatorID]int64
	globalGasFloor           int64
	globalGasPrice           int64
	methodist                *dexter.Methodist
	evmState                 *EvmState
	validators               *pos.Validators
	epoch                    idx.Epoch
	mu                       sync.RWMutex
	lastValidatorCheckedTime time.Time
	tournament               *Tournament
	validatorMu              sync.RWMutex
	gunMu                    sync.Mutex
}

type EvmState struct {
	statedb        *state.StateDB
	evmStateReader *EvmStateReader
	bs             iblockproc.BlockState
	pendingEvents  hash.OrderedEvents
	pendingUpdates map[common.Address]*dexter.PoolUpdate
	mu             sync.Mutex
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

type NodeType int

const (
	GENERAL NodeType = iota
	SCOUT
	PHILOSOPHER
	BIGGUNS
)

func NewDexter(svc *Service) *Dexter {
	nodeType := GENERAL
	nodeTypeStr, ok := os.LookupEnv("DEXTER_NODE_TYPE")
	if ok {
		switch nodeTypeStr {
		case "SCOUT":
			nodeType = SCOUT
		case "PHILOSOPHER":
			nodeType = PHILOSOPHER
		case "BIGGUNS":
			nodeType = BIGGUNS
		}
	}
	log.Info("Creating dexter", "nodeType", nodeType)
	d := &Dexter{
		nodeType:           nodeType,
		svc:                svc,
		inTxChan:           make(chan *dexter.TxWithTimeLog, 256),
		inFriendlyFireChan: make(chan *types.Transaction, 32),
		inLogsChan:         make(chan []*types.Log, 4096),
		inBlockChan:        make(chan evmcore.ChainHeadNotify, 4096),
		inEpochChan:        make(chan idx.Epoch, 4096),
		inEventChan:        make(chan *inter.EventPayload, 4096),
		interestedPairs:    make(map[common.Address]dexter.PoolType),
		interestedPools:    make(map[dexter.BalPoolId]dexter.PoolType),
		lag:                time.Minute * 60,
		signer:             gsignercache.Wrap(types.LatestSignerForChainID(svc.store.GetRules().EvmChainConfig().ChainID)),
		gunRefreshes:       make(chan GunList, 256),
		gunLastFired:       make(map[common.Address]time.Time),
		watchedTxs:         make(chan *TxSub, 256),
		firedTxChan:        make(chan *FiredTx, 256),
		// ignoreTxs:         make(map[common.Hash]struct{}),
		clearIgnoreTxChan: make(chan common.Hash, 16),
		txRaceChan:        make(chan *RaceEntry, 8192),
		railgunChan:       make(chan *dexter.RailgunPacket, 8),
		// tokenWhitelistChan: make(chan common.Address, 128),
		poolsInfo: make(map[common.Address]*dexter.PoolInfo),
		gasFloors: make(map[idx.ValidatorID]int64),
		methodist: dexter.NewMethodist(root, 60*time.Second),
		evmState: &EvmState{
			pendingUpdates: make(map[common.Address]*dexter.PoolUpdate),
		},
		tournament: NewTournament(svc),
	}
	d.validators, d.epoch = d.svc.store.GetEpochValidators()
	if d.nodeType == GENERAL {
		d.strategies = []dexter.Strategy{
			dexter.NewLinearStrategy("Linear 2-3", 0, d.railgunChan, dexter.LinearStrategyConfig{
				RoutesFileName:          root + "route_caches/solidly_routes_len2-3.json",
				PoolToRouteIdxsFileName: root + "route_caches/solidly_pairToRouteIdxs_len2-3.json",
			}),

			dexter.NewBalancerLinearStrategy("Balancer 2-3 combined", 1, d.railgunChan, dexter.BalancerLinearStrategyConfig{
				RoutesFileName:          root + "route_caches/solidly_balancer_routes_len2-3.json",
				PoolToRouteIdxsFileName: root + "route_caches/solidly_balancer_poolToRouteIdxs_len2-3.json",
			}),

			dexter.NewBalancerLinearStrategy("Curve 2-4", 2, d.railgunChan, dexter.BalancerLinearStrategyConfig{
				RoutesFileName:          root + "route_caches/curve_plain_routes_len2-4.json",
				PoolToRouteIdxsFileName: root + "route_caches/curve_plain_poolToRouteIdxs_len2-4.json",
			}),
		}
	} else if d.nodeType == BIGGUNS {
		d.strategies = []dexter.Strategy{
			// dexter.NewLinearStrategy("Linear 2-3", 0, d.railgunChan, dexter.LinearStrategyConfig{
			// 	RoutesFileName:          root + "route_caches/solidly_routes_len2-3.json",
			// 	PoolToRouteIdxsFileName: root + "route_caches/solidly_pairToRouteIdxs_len2-3.json",
			// }),

			// dexter.NewBalancerLinearStrategy("Balancer 2-3 combined", 1, d.railgunChan, dexter.BalancerLinearStrategyConfig{
			// 	RoutesFileName:          root + "route_caches/solidly_balancer_routes_len2-3.json",
			// 	PoolToRouteIdxsFileName: root + "route_caches/solidly_balancer_poolToRouteIdxs_len2-3.json",
			// }),
			dexter.NewLinearStrategy("Linear 2-4", 0, d.railgunChan, dexter.LinearStrategyConfig{
				RoutesFileName:          root + "route_caches/solidly_routes_len2-4.json",
				PoolToRouteIdxsFileName: root + "route_caches/solidly_pairToRouteIdxs_len2-4.json",
			}),

			dexter.NewBalancerLinearStrategy("Balancer 2-4 Sans", 1, d.railgunChan, dexter.BalancerLinearStrategyConfig{
				RoutesFileName:          root + "route_caches/solidly_balancer_no_wftm_poolToRouteIdxs_len2-4.json",
				PoolToRouteIdxsFileName: root + "route_caches/solidly_balancer_no_wftm_routes_len2-4.json",
			}),

			dexter.NewBalancerLinearStrategy("Balancer Stable", 2, d.railgunChan, dexter.BalancerLinearStrategyConfig{
				RoutesFileName:          root + "route_caches/solidly_balancer_routes_len2-4.json",
				PoolToRouteIdxsFileName: root + "route_caches/solidly_balancer_poolToRouteIdxs_len2-4.json",
			}),

			dexter.NewBalancerLinearStrategy("Curve 2-4", 3, d.railgunChan, dexter.BalancerLinearStrategyConfig{
				RoutesFileName:          root + "route_caches/curve_plain_routes_len2-4.json",
				PoolToRouteIdxsFileName: root + "route_caches/curve_plain_poolToRouteIdxs_len2-4.json",
			}),
		}
	}

	// dexter.NewBalancerLinearStrategy("Balancer 2-3 Weighted", 1, d.railgunChan, dexter.BalancerLinearStrategyConfig{
	// 	RoutesFileName:          root + "route_caches/balancer_poolToRouteIdxs_len2-3.json",
	// 	PoolToRouteIdxsFileName: root + "route_caches/balancer_routes_len2-3.json",
	// }),

	// dexter.NewLinearStrategy("Linear 2", 0, d.railgunChan, dexter.LinearStrategyConfig{
	// 	RoutesFileName:          root + "route_caches/solidly_routes_len2.json",
	// 	PoolToRouteIdxsFileName: root + "route_caches/solidly_pairToRouteIdxs_len2.json",
	// }),

	// dexter.NewBalancerLinearStrategy("Balancer Stable", 1, d.railgunChan, dexter.BalancerLinearStrategyConfig{
	// 	RoutesFileName:          root + "route_caches/balancer_routes_len2.json",
	// 	PoolToRouteIdxsFileName: root + "route_caches/balancer_poolToRouteIdxs_len2.json",
	// }),

	// dexter.NewBalancerLinearStrategy("Balancer Sans", 1, d.railgunChan, dexter.BalancerLinearStrategyConfig{
	// 	RoutesFileName:          root + "route_caches/solidly_balancer_no_wftm_routes_len2-4.json",
	// 	PoolToRouteIdxsFileName: root + "route_caches/solidly_balancer_no_wftm_poolToRouteIdxs_len2-4.json",
	// }),

	// dexter.NewLinearStrategy("Linear sans wftm", 1, d.railgunChan, dexter.LinearStrategyConfig{
	// 	RoutesFileName:          root + "route_caches/solidly_routes_no_wftm_2-4.json",
	// 	PoolToRouteIdxsFileName: root + "route_caches/solidly_pairToRouteIdxs_no_wftm_2-4.json",
	// }),

	// dexter.NewBalancerLinearStrategy("Balancer Stable", 2, d.railgunChan, dexter.BalancerLinearStrategyConfig{
	// 	RoutesFileName:          root + "route_caches/solidly_balancer_routes_len2-3.json",
	// 	PoolToRouteIdxsFileName: root + "route_caches/solidly_balancer_poolToRouteIdxs_len2-3.json",
	// }),

	// dexter.NewLinearStrategy("Linear 2-4", 0, d.railgunChan, dexter.LinearStrategyConfig{
	// 	RoutesFileName:          root + "route_caches/solidly_routes_len2-4.json",
	// 	PoolToRouteIdxsFileName: root + "route_caches/solidly_pairToRouteIdxs_len2-4.json",
	// }),

	// dexter.NewLinearStrategy("Linear sans wftm", 1, d.railgunChan, dexter.LinearStrategyConfig{
	// 	RoutesFileName:          root + "route_caches/solidly_routes_no_wftm_2-4.json",
	// 	PoolToRouteIdxsFileName: root + "route_caches/solidly_pairToRouteIdxs_no_wftm_2-4.json",
	// }),

	if d.nodeType != SCOUT {
		d.strategyBravado = make([]float64, len(d.strategies)+1)
		d.mtts = make([]time.Duration, len(d.strategies)+1)
		d.numFired = make([]int64, len(d.strategies)+1)
		for i := 0; i < len(d.strategyBravado); i++ {
			d.strategyBravado[i] = 1
			d.mtts[i] = time.Duration(0)
		}
		// d.strategies[1].AddSubStrategy(d.strategies[1])
		// d.strategies[1].AddSubStrategy(d.strategies[2])
		d.loadJson()

		for _, s := range d.strategies {
			pairs, pools := s.GetInterestedPools()
			for pairAddr, t := range pairs {
				d.interestedPairs[pairAddr] = t
			}
			// log.Info("Got interested pools", "i", i, "len", len(pools))
			for poolAddr, t := range pools {
				d.interestedPools[poolAddr] = t
			}
		}

		edgePools := make(map[dexter.EdgeKey][]common.Address)
		for pairAddr, t := range d.interestedPairs {
			pairInfo, ok := d.poolsInfo[pairAddr]
			if !ok {
				log.Warn("Could not find pairInfo for interested pair", "addr", pairAddr)
				continue
			}
			if t == dexter.UniswapV2Pair || t == dexter.SolidlyVolatilePool || t == dexter.SolidlyStablePool {
				reserve0, reserve1 := d.getReserves(&pairAddr)
				token0, token1 := d.getUniswapPairTokens(&pairAddr)
				d.poolsInfo[pairAddr] = &dexter.PoolInfo{
					Reserves:     map[common.Address]*big.Int{token0: reserve0, token1: reserve1},
					Tokens:       []common.Address{token0, token1},
					FeeNumerator: pairInfo.FeeNumerator,
					LastUpdate:   time.Now(),
				}
				edgeKey := dexter.MakeEdgeKey(token0, token1)
				if pools, ok := edgePools[edgeKey]; ok {
					edgePools[edgeKey] = append(pools, pairAddr)
				} else {
					edgePools[edgeKey] = []common.Address{pairAddr}
				}
			} else {
				log.Error("Interested in pair that is not uniswap pair?!", "addr", pairAddr.Hex())
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
			fee := d.getSwapFeePercentage(&poolAddr, t)
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
					amp := d.getAmplificationParameter(poolAddr, t)
					poolInfo.AmplificationParam = amp
					for i, bal := range poolTokens.Balances {
						poolInfo.Reserves[poolTokens.Tokens[i]] = bal
					}
					// log.Info("Balancer stable pool info", "id", poolId, "info", poolInfo)
				}
				// log.Info("Balancer pool info", "id", poolId, "info", poolInfo)
			} else if t == dexter.CurveBasePlainPool || t == dexter.CurveFactoryPlainPool ||
				t == dexter.CurveFactoryMetaPool {
				d.poolsInfo[poolAddr] = &poolInfo
				poolInfo.Tokens = d.getTokensCurve(poolAddr, t)
				// log.Info("Curve pool tokens", "addr", poolAddr, "tokens", poolInfo.Tokens, "fee", poolInfo.Fee)
				for _, tokAddr := range poolInfo.Tokens {
					decimals := d.decimals(&tokAddr)
					decimalsDiff := int64(18 - decimals)
					scaleFactor := new(big.Int).Exp(big.NewInt(10), big.NewInt(decimalsDiff), nil)
					poolInfo.ScaleFactors[tokAddr] = scaleFactor
				}
				poolInfo.AmplificationParam = d.getAmplificationParameter(poolAddr, t)
				balances := d.getBalancesCurve(poolAddr, t)
				// log.Info("Curve pool info", "addr", poolAddr, "info", poolInfo)
				for i, bal := range balances {
					poolInfo.Reserves[poolInfo.Tokens[i]] = bal
				}
				// log.Info("Initialised curve pool", "addr", poolAddr, "info", poolInfo)
			}
		}
		log.Info("Min gas price", "minGasPrice", d.svc.store.GetRules().Economy.MinGasPrice)

		for _, s := range d.strategies {
			s.SetPoolsInfo(d.poolsInfo)
			s.SetEdgePools(edgePools)
			s.Start()
		}
	}

	blacklist := make(map[common.Address]struct{})
	for addr, _ := range arbitrageurs {
		blacklist[addr] = struct{}{}
	}
	delete(blacklist, fishAddr)
	d.svc.txpool.SetToBlacklist(blacklist)
	go d.updateMethods()
	go d.watchFriendlyFire()
	d.inBlockSub = svc.feed.SubscribeNewBlock(d.inBlockChan)
	go d.processIncomingBlocks()
	d.inEpochSub = svc.feed.SubscribeNewEpoch(d.inEpochChan)
	go d.runRailgun()
	svc.handler.RaceEvents(d.tournament.eventRaceChan)
	if d.nodeType == SCOUT {
		go d.scoutIncomingTxs()
	} else {
		go d.processIncomingTxs()
		// d.inLogsSub = svc.feed.SubscribeNewLogs(d.inLogsChan)
		svc.handler.SubscribeEvents(d.inEventChan)
		d.refreshEvmState(types.Receipts{})
		// go d.eventRace()
		go d.watchEvents()
		// go d.processIncomingLogs()
	}
	// svc.handler.RaceTxs(d.txRaceChan)
	return d
}

func (d *Dexter) AddTrustedNodes() {
	time.Sleep(10 * time.Second) // FIXME: Race condition
	trustedNodesStr, ok := os.LookupEnv("TRUSTED_NODES")
	if !ok {
		log.Info("Could not find trusted nodes")
		return
	}
	nodesArr := strings.Split(trustedNodesStr, ",")
	for _, url := range nodesArr {
		node, err := enode.Parse(enode.ValidSchemes, url)
		if err != nil {
			log.Error("Trusted URL invalid", "enode", url, "err", err)
			continue
		}
		d.svc.p2pServer.AddPeer(node)
		d.svc.p2pServer.AddTrustedPeer(node)
		log.Info("Added trusted peer", "node", node)
	}
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

// func (d *Dexter) eventRace() {
// 	seenTxs, _ := lru.New(512)
// 	seenEvents, _ := lru.New(4096)
// 	eventWins := make(map[string]int)
// 	txWins := make(map[string]int)
// 	minLifetime := 5 * 60 * time.Second
// 	numSortedTxPeers := 8
// 	prevCullTime := time.Now()
// 	for {
// 		select {
// 		case e := <-d.eventRaceChan:
// 			if _, ok := seenEvents.Get(e.Hash); !ok {
// 				seenEvents.Add(e.Hash, struct{}{})
// 				if wins, ok := eventWins[e.PeerID]; ok {
// 					eventWins[e.PeerID] = wins + 1
// 				} else {
// 					eventWins[e.PeerID] = 1
// 				}
// 			}
// 		case e := <-d.txRaceChan:
// 			if _, ok := seenTxs.Get(e.Hash); !ok {
// 				seenTxs.Add(e.Hash, e)
// 				if wins, ok := txWins[e.PeerID]; ok {
// 					txWins[e.PeerID] = wins + 1
// 				} else {
// 					txWins[e.PeerID] = 1
// 				}
// 			}
// 		}
// 		now := time.Now()
// 		if now.Sub(prevCullTime) > minLifetime {
// 			// txWinPools := mapToSortedPools(txWins)
// 			// for i, winPool := range txWinPools {
// 			// 	log.Info("tx wins", "i", i, "peer", winPool.Key, "wins", winPool.Value)
// 			// }
// 			var losers []string
// 			peers := d.svc.handler.peers.List()
// 			for _, peer := range peers {
// 				if now.Sub(peer.created) < minLifetime {
// 					continue // Give the noob a chance
// 				}
// 				_, wonEvent := eventWins[peer.id]
// 				// txWins, wonTx := txWins[peer.id]
// 				// if !wonEvent && (!wonTx || txWins < 3) {
// 				if !wonEvent {
// 					losers = append(losers, peer.id)
// 				}
// 			}
// 			sortedPeers := make([]string, 0, len(peers)-len(losers))
// 			eventWinPools := mapToSortedPools(eventWins)
// 			for _, winPool := range eventWinPools {
// 				sortedPeers = append(sortedPeers, winPool.Key)
// 			}
// 			sortedTxPeers := make([]string, 0, numSortedTxPeers)
// 			// txWinPools := mapToSortedPools(txWins)
// 			// for _, winPool := range txWinPools {
// 			// 	sortedTxPeers = append(sortedTxPeers, winPool.Key)
// 			// 	if len(sortedTxPeers) == numSortedTxPeers {
// 			// 		break
// 			// 	}
// 			// }
// 			sortedPeers = append(sortedPeers, losers...)
// 			// log.Info("Detected winners and losers", "peers", len(peers), "eventWins", len(eventWins), "txWins", len(txWins), "losers", len(losers), "sorted", len(sortedPeers), "sortedTxPeers", len(sortedTxPeers))
// 			// d.svc.handler.peers.Cull(losers)
// 			d.svc.handler.peers.SetSortedPeers(sortedPeers)
// 			d.svc.handler.peers.SetSortedTxPeers(sortedTxPeers)
// 			// log.Info("Connected peers", "peers", strings.Join(d.svc.handler.peers.GetSortedIPs(), ", "))
// 			eventWins = make(map[string]int)
// 			txWins = make(map[string]int)
// 			prevCullTime = time.Now()
// 		}
// 	}
// }

func (d *Dexter) loadJson() {
	poolsFileName := root + "pairs.json"
	poolsFile, err := os.Open(poolsFileName)
	if err != nil {
		log.Info("Error opening pools", "poolsFileName", poolsFileName, "err", err)
		return
	}
	defer poolsFile.Close()
	poolsBytes, _ := ioutil.ReadAll(poolsFile)
	var jsonPools []dexter.PoolInfoJson
	json.Unmarshal(poolsBytes, &jsonPools)
	for _, jsonPool := range jsonPools {
		poolAddr := common.HexToAddress(jsonPool.Addr)
		poolType := dexter.UniswapV2Pair
		if jsonPool.ExchangeType == "SolidlyVolatilePool" {
			poolType = dexter.SolidlyVolatilePool
		} else if jsonPool.ExchangeType == "SolidlyStablePool" {
			poolType = dexter.SolidlyStablePool
		}
		d.poolsInfo[poolAddr] = &dexter.PoolInfo{
			FeeNumerator: big.NewInt(jsonPool.FeeNumerator),
			Type:         poolType,
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

func (d *Dexter) watchFriendlyFire() {
	seenTxs, _ := lru.New(512)
	var rtLag time.Duration
	for {
		tx := <-d.inFriendlyFireChan
		if d.lag > 4*time.Second {
			continue
		}
		if _, ok := seenTxs.Get(tx.Hash()); ok {
			continue
		}
		seenTxs.Add(tx.Hash(), struct{}{})
		data := tx.Data()
		var txs []*types.Transaction
		if data[0] == 0x66 && len(data) > 36 { // fish7
			var targetTxHash common.Hash
			copy(targetTxHash[:], data[4:36])
			// targetTxHash := common.Hash(data[4:36])
			targetTx := d.svc.txpool.Get(targetTxHash)
			if targetTx != nil {
				txs = append(txs, targetTx)
				lag := time.Now().Sub(targetTx.Time())
				rtLag = time.Duration(float64(lag)*mttsAlpha + float64(rtLag)*(1-mttsAlpha))
				log.Info("Found source tx", "round trip lag", utils.PrettyDuration(lag), "avg rtlag", rtLag)
			}
		}
		txs = append(txs, tx)
		from, _ := types.Sender(d.signer, tx)
		validatorIDs := d.predictValidators(from, tx.Nonce(), d.validators, d.epoch, 1)
		log.Info("Amplifying tx", "len", len(txs), "hash", tx.Hash().Hex(), "lag", utils.PrettyDuration(time.Now().Sub(tx.Time())))
		peers := d.tournament.GetSortedPeers(validatorIDs[0])
		go d.svc.handler.BroadcastTxsAggressive(txs, BroadcastNonTrusted, peers)

		if d.nodeType != SCOUT {
			from, _ := types.Sender(d.signer, tx)
			d.gunMu.Lock()
			d.gunLastFired[from] = time.Now()
			d.gunMu.Unlock()
		}
	}
}

func (d *Dexter) processIncomingLogs() {
	log.Info("Started dexter")
	for {
		// FIXME: Convert this to a map[common.Address]PoolUpdate to dedupe.
		// var updates []*dexter.PoolUpdate
		select {
		case logs := <-d.inLogsChan:
			var lastTxIndex uint = math.MaxUint64
			for _, l := range logs {
				updated := false
				poolAddr, _, _ := getReservesFromSyncLog(l)
				if poolAddr != nil {
					_, ok := d.interestedPairs[*poolAddr]
					if !ok {
						continue
					}
					// token0, token1 := d.getUniswapPairTokens(poolAddr)
					// updates = append(updates, &dexter.PoolUpdate{
					// 	Addr:     *poolAddr,
					// 	Reserves: map[common.Address]*big.Int{token0: reserve0, token1: reserve1},
					// })
					updated = true
				} else if len(l.Topics) >= 2 && (bytes.Compare(l.Topics[0].Bytes(), swapBalancerEventTopic) == 0 || bytes.Compare(l.Topics[0].Bytes(), poolBalanceChangedEventTopic) == 0) {
					// var poolId dexter.BalPoolId
					// copy(poolId[:], l.Topics[1].Bytes())
					// var poolAddr common.Address = common.BytesToAddress(poolId[:20])
					// // log.Info("Balancer swap event", "l", l, "poolId", poolId)
					// pool := d.getPoolTokens(poolId)
					// // log.Info("Balancer updated pool tokens", "pool", pool, "addr", poolAddr)
					// u := &dexter.PoolUpdate{
					// 	Addr:     poolAddr,
					// 	Reserves: make(map[common.Address]*big.Int),
					// }
					// for i, tokAddr := range pool.Tokens {
					// 	u.Reserves[tokAddr] = pool.Balances[i]
					// }
					// updates = append(updates, u)
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
			// if len(updates) > 0 {
			// 	for _, s := range d.strategies {
			// 		s.ProcessPermUpdates(updates)
			// 	}
			// }
		case <-d.inLogsSub.Err():
			return
		}
	}
	d.inLogsSub.Unsubscribe()
}

func (d *Dexter) scoutIncomingTxs() {
	log.Info("Started dexter")
	seenTxs, _ := lru.New(8192)
	for {
		var txs []*types.Transaction
		txWTL := <-d.inTxChan
		if _, ok := seenTxs.Get(txWTL.Tx.Hash()); ok {
			continue
		}
		seenTxs.Add(txWTL.Tx.Hash(), struct{}{})
		if d.lag > 4*time.Second {
			continue
		}
		txs = append(txs, txWTL.Tx)
	loop:
		for {
			select {
			case txWTL = <-d.inTxChan:
				if _, ok := seenTxs.Get(txWTL.Tx.Hash()); ok {
					continue
				}
				seenTxs.Add(txWTL.Tx.Hash(), struct{}{})
				txs = append(txs, txWTL.Tx)
			default:
				break loop
			}
		}
		// log.Info("Reporting interesting tx", "lag", utils.PrettyDuration(time.Now().Sub(txs[0].Time())), "len", len(txs))
		go d.svc.handler.BroadcastTxsAggressive(txs, BroadcastTrusted, nil)
	}
}

func (d *Dexter) processIncomingTxs() {
	log.Info("Started dexter")
	seenTxs, _ := lru.New(8192)
	for {
		txWTL := <-d.inTxChan
		if _, ok := seenTxs.Get(txWTL.Tx.Hash()); ok {
			continue
		}
		seenTxs.Add(txWTL.Tx.Hash(), struct{}{})
		if d.numFiredThisBlock > 2 {
			continue
		}
		if d.lag > 4*time.Second {
			continue
		}
		txWTL.Log.RecordTime(dexter.DexterReceived)
		go d.processTx(txWTL)
		// log.Info("Received interesting tx", "len", len(d.inTxChan))
	}
}

func (d *Dexter) processIncomingBlocks() {
	log.Info("Started dexter")
	watchedTxMap, _ := lru.New(512)
	for {
		select {
		case n := <-d.inBlockChan:
			d.lag = time.Now().Sub(n.Block.Time.Time())
			if d.nodeType == SCOUT {
				continue
			}
			if d.lag < 4*time.Second {
				go d.refreshGuns()
			}
			d.numFiredThisBlock = 0
			receipts, err := d.svc.EthAPI.GetReceipts(context.TODO(), n.Block.Hash)
			if err != nil {
				log.Error("Could not get block receipts", "err", err)
				continue
			}
			d.refreshEvmState(receipts)
			for strategyID, bravado := range d.strategyBravado {
				d.strategyBravado[strategyID] = math.Min(1.0, bravado+0.005)
				// d.strategies[strategyID].SetGasPrice(d.globalGasPrice)
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
				if f_, ok := watchedTxMap.Get(tx.Hash()); ok {
					f := f_.(*FiredTx)
					receipt := receipts[i]
					gasEst := new(big.Int).Div(f.Plan.GasCost, f.Plan.GasPrice)
					if receipt.Status == types.ReceiptStatusSuccessful {
						d.methodist.Record(dexter.MethodEvent{f.TargetMethod, dexter.Success, tx.Hash()})

						d.strategyBravado[f.StrategyID] = d.strategyBravado[f.StrategyID]*bravadoAlpha + (1 - bravadoAlpha)
						log.Info("SUCCESS", "est profit", dexter.BigIntToFloat(f.Plan.NetProfit)/1e18, "tx", tx.Hash().Hex(), "strategy", f.StrategyID, "new bravado", d.strategyBravado[f.StrategyID], "lag", utils.PrettyDuration(time.Now().Sub(f.Time)), "gas", receipt.GasUsed, "estimated", gasEst)
					} else {
						d.diagnoseTx(tx, f)
						d.strategyBravado[f.StrategyID] = d.strategyBravado[f.StrategyID] * bravadoAlpha
						log.Info("Fail", "est profit", dexter.BigIntToFloat(f.Plan.NetProfit)/1e18, "tx", tx.Hash().Hex(), "strategy", f.StrategyID, "new bravado", d.strategyBravado[f.StrategyID], "lag", utils.PrettyDuration(time.Now().Sub(f.Time)), "gas", receipt.GasUsed, "estimated", gasEst)
					}
					watchedTxMap.Remove(tx.Hash())
				}
			}
		case <-d.inBlockSub.Err():
			return
		case f := <-d.firedTxChan:
			watchedTxMap.Add(f.Hash, f)
		}
	}
	d.inLogsSub.Unsubscribe()
}

func (d *Dexter) playPendingEvents(pendingEvents hash.OrderedEvents, evmStateReader *EvmStateReader, lastBlockIdx idx.Block, statedb *state.StateDB) map[common.Address]*dexter.PoolUpdate {
	txs := types.Transactions{}
	for _, id := range pendingEvents {
		e := d.svc.store.GetEventPayload(id)
		txs = append(txs, e.Txs()...)
	}
	evmBlock := d.evmBlockWith(txs, lastBlockIdx, evmStateReader)
	evmProcessor := evmcore.NewStateProcessor(d.svc.store.GetRules().EvmChainConfig(), evmStateReader)
	var gasUsed uint64
	updates := make(map[common.Address]*dexter.PoolUpdate)
	evmProcessor.Process(evmBlock, statedb, opera.DefaultVMConfig, &gasUsed, false, func(l *types.Log, _ *state.StateDB) {
		update := d.processPendingLogs(l, statedb)
		if update != nil {
			updates[update.Addr] = update
		}
	})
	return updates
}

func (d *Dexter) refreshEvmState(receipts types.Receipts) {
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
	permUpdates := make(map[common.Address]*dexter.PoolUpdate)
	for _, receipt := range receipts {
		for _, l := range receipt.Logs {
			update := d.processPendingLogs(l, statedb)
			if update != nil {
				permUpdates[update.Addr] = update
			}
		}
	}
	pendingUpdates := make(map[common.Address]*dexter.PoolUpdate)
	var newPendingEvents hash.OrderedEvents
	if d.lag < 4*time.Second {
		start := 0
		d.evmState.mu.Lock()
		for ; start < len(d.evmState.pendingEvents); start++ {
			e := d.evmState.pendingEvents[start]
			if bytes.Compare(bs.LastBlock.Atropos.Bytes(), e.Bytes()) == -1 {
				break
			}
		}
		newPendingEvents := make(hash.OrderedEvents, len(d.evmState.pendingEvents)-start)
		copy(newPendingEvents, d.evmState.pendingEvents[start:])
		d.evmState.mu.Unlock()
		pendingUpdates = d.playPendingEvents(newPendingEvents, evmStateReader, bs.LastBlock.Idx, statedb)
	}
	if len(permUpdates) > 0 || len(pendingUpdates) > 0 {
		pendingUpdatesSend := make(map[common.Address]*dexter.PoolUpdate)
		for a, u := range d.evmState.pendingUpdates {
			pendingUpdatesSend[a] = u // Copy this to avoid concurrent access issues
		}
		for _, s := range d.strategies {
			s.ProcessStateUpdates(dexter.StateUpdate{
				PermUpdates:    permUpdates,
				PendingUpdates: pendingUpdatesSend,
			})
		}
	}
	d.evmState.mu.Lock()
	defer d.evmState.mu.Unlock()
	d.evmState.statedb = statedb
	d.evmState.evmStateReader = evmStateReader
	d.evmState.bs = bs
	d.evmState.pendingUpdates = pendingUpdates
	d.evmState.pendingEvents = newPendingEvents
}

func (d *Dexter) advanceEvmState(e *inter.EventPayload) {
	d.evmState.mu.Lock() // LOCK
	d.evmState.pendingEvents = append(d.evmState.pendingEvents, e.Event.ID())
	updated := false
	if len(d.evmState.pendingEvents) > 1 && bytes.Compare(e.Event.ID().Bytes(), d.evmState.pendingEvents[len(d.evmState.pendingEvents)-2].Bytes()) == -1 {
		// Sort and replay events
		newPendingEvents := make(hash.OrderedEvents, len(d.evmState.pendingEvents))
		copy(newPendingEvents, d.evmState.pendingEvents)
		statedb, _ := d.svc.store.evm.StateDB(d.evmState.bs.FinalizedStateRoot)
		lastBlockIdx := d.evmState.bs.LastBlock.Idx
		d.evmState.mu.Unlock() // UNLOCK - for batch replay
		newPendingEvents.ByEpochAndLamport()
		evmStateReader := &EvmStateReader{
			ServiceFeed: &d.svc.feed,
			store:       d.svc.store,
		}
		pendingUpdates := d.playPendingEvents(newPendingEvents, evmStateReader, lastBlockIdx, statedb)
		d.evmState.mu.Lock() // LOCK
		d.evmState.statedb = statedb
		d.evmState.evmStateReader = evmStateReader
		d.evmState.pendingUpdates = pendingUpdates
		d.evmState.pendingEvents = newPendingEvents
		updated = true
	} else {
		// Event in order; play on top of existing state
		evmBlock := d.evmBlockWith(e.Txs(), d.evmState.bs.LastBlock.Idx, d.evmState.evmStateReader)
		evmProcessor := evmcore.NewStateProcessor(d.svc.store.GetRules().EvmChainConfig(), d.evmState.evmStateReader)
		var gasUsed uint64
		evmProcessor.Process(evmBlock, d.evmState.statedb, opera.DefaultVMConfig, &gasUsed, false, func(l *types.Log, _ *state.StateDB) {
			update := d.processPendingLogs(l, d.evmState.statedb)
			if update != nil {
				d.evmState.pendingUpdates[update.Addr] = update
				updated = true
			}
		})
	}
	if !updated {
		d.evmState.mu.Unlock()
		return
	}
	pendingUpdatesSend := make(map[common.Address]*dexter.PoolUpdate)
	for a, u := range d.evmState.pendingUpdates {
		pendingUpdatesSend[a] = u // Copy this to avoid concurrent access issues
	}
	d.evmState.mu.Unlock()
	for _, s := range d.strategies {
		s.ProcessStateUpdates(dexter.StateUpdate{
			PermUpdates:    nil,
			PendingUpdates: pendingUpdatesSend,
		})
	}
}

func (d *Dexter) processPendingLogs(l *types.Log, statedb *state.StateDB) *dexter.PoolUpdate {
	poolAddr, reserve0, reserve1 := getReservesFromSyncLog(l)
	if poolAddr != nil {
		_, ok := d.interestedPairs[*poolAddr]
		if !ok {
			return nil
		}
		token0, token1 := d.getUniswapPairTokens(poolAddr)
		return &dexter.PoolUpdate{
			Addr:     *poolAddr,
			Reserves: map[common.Address]*big.Int{token0: reserve0, token1: reserve1},
			Time:     time.Now(),
		}
	} else if len(l.Topics) >= 2 && (bytes.Compare(l.Topics[0].Bytes(), swapBalancerEventTopic) == 0 || bytes.Compare(l.Topics[0].Bytes(), poolBalanceChangedEventTopic) == 0) {
		var poolId dexter.BalPoolId
		copy(poolId[:], l.Topics[1].Bytes())
		var poolAddr common.Address = common.BytesToAddress(poolId[:20])
		evm := d.getEvm(statedb, d.evmState.evmStateReader, d.evmState.bs.LastBlock.Idx)
		pool := d.getPoolTokensFromEvm(poolId, evm)
		u := &dexter.PoolUpdate{
			Addr:     poolAddr,
			Reserves: make(map[common.Address]*big.Int),
		}
		for i, tokAddr := range pool.Tokens {
			u.Reserves[tokAddr] = pool.Balances[i]
		}
		return u
	} else if poolInfo, ok := d.poolsInfo[l.Address]; ok && (poolInfo.Type == dexter.CurveBasePlainPool || poolInfo.Type == dexter.CurveFactoryPlainPool) {
		evm := d.getEvm(statedb, d.evmState.evmStateReader, d.evmState.bs.LastBlock.Idx)
		tokens := d.getTokensCurveFromEvm(l.Address, poolInfo.Type, evm)
		balances := d.getBalancesCurveFromEvm(l.Address, poolInfo.Type, evm)
		u := &dexter.PoolUpdate{
			Addr:     l.Address,
			Reserves: make(map[common.Address]*big.Int),
		}
		for i, tokAddr := range tokens {
			u.Reserves[tokAddr] = balances[i]
		}
		// log.Info("Processed pending log of curve pool", "addr", l.Address, "reserves", u.Reserves)
		return u
	}
	return nil
}

func (d *Dexter) processTx(txWTL *dexter.TxWithTimeLog) {
	txWTL.Log.RecordTime(dexter.ProcessTxStarted)
	start := time.Now()
	data := txWTL.Tx.Data()
	if len(data) < 4 {
		return
	}
	if start.Sub(d.lastValidatorCheckedTime) > 200*time.Millisecond {
		d.lastValidatorCheckedTime = start
		go func() {
			from, _ := types.Sender(d.signer, txWTL.Tx)
			d.validatorMu.RLock()
			validatorIDs := d.predictValidators(from, txWTL.Tx.Nonce(), d.validators, d.epoch, numValidators)
			d.validatorMu.RUnlock()
			select {
			case d.watchedTxs <- &TxSub{
				Hash:                txWTL.Tx.Hash(),
				PredictedValidators: validatorIDs,
				Print:               false, StartTime: start,
			}:
			default:
			}
		}()
	}
	var method [4]byte
	copy(method[:], data[:4])
	txs := types.Transactions{txWTL.Tx}
	var gasUsed uint64
	var ptxUpdates []dexter.PoolUpdate
	var updatedPools []dexter.BalPoolId
	updatedCurvePools := make(map[common.Address]dexter.PoolType)
	var crumbs []hansel_lite.Breadcrumb
	d.evmState.mu.Lock() // LOCK MUTEX
	txWTL.Log.RecordTime(dexter.EvmStateCopyStarted)
	statedb := d.evmState.statedb.Copy()
	evmStateReader := &EvmStateReader{
		ServiceFeed: &d.svc.feed,
		store:       d.svc.store,
	}
	lastBlockIdx := d.evmState.bs.LastBlock.Idx
	d.evmState.mu.Unlock() // UNLOCK MUTEX
	txWTL.Log.RecordTime(dexter.EvmStateCopyFinished)
	evmBlock := d.evmBlockWith(txs, lastBlockIdx, evmStateReader)
	evmProcessor := evmcore.NewStateProcessor(d.svc.store.GetRules().EvmChainConfig(), evmStateReader)
	txWTL.Log.RecordTime(dexter.TxExecuteStarted)
	evmProcessor.Process(evmBlock, statedb, opera.DefaultVMConfig, &gasUsed, false, func(l *types.Log, _ *state.StateDB) {
		poolAddr, reserve0, reserve1 := getReservesFromSyncLog(l)
		if poolAddr != nil {
			if _, ok := d.interestedPairs[*poolAddr]; ok {
				token0, token1 := d.getUniswapPairTokens(poolAddr)
				ptxUpdates = append(ptxUpdates, dexter.PoolUpdate{
					Addr:     *poolAddr,
					Reserves: map[common.Address]*big.Int{token0: reserve0, token1: reserve1},
				})
			}
		} else if len(l.Topics) >= 2 && (bytes.Compare(l.Topics[0].Bytes(), swapBalancerEventTopic) == 0 || bytes.Compare(l.Topics[0].Bytes(), poolBalanceChangedEventTopic) == 0) {
			var poolId dexter.BalPoolId
			copy(poolId[:], l.Topics[1].Bytes())
			updatedPools = append(updatedPools, poolId)
			// log.Info("Balancer swap event", "topics", l.Topics, "l", l.Data)
		} else if poolInfo, ok := d.poolsInfo[l.Address]; ok && (poolInfo.Type == dexter.CurveBasePlainPool || poolInfo.Type == dexter.CurveFactoryPlainPool) {
			updatedCurvePools[l.Address] = poolInfo.Type
			// log.Warn("Curve pool event", "topics", l.Topics, "l", l.Data)
			// } else if poolAddr, amountIn0, _, _, _ := getAmountsFromSwapLog(l); poolAddr != nil { // Hansel
			// 	if poolInfo, ok := d.poolsInfo[*poolAddr]; ok {
			// 		if poolInfo.Type != dexter.UniswapV2Pair && poolInfo.Type != dexter.SolidlyVolatilePool {
			// 			return
			// 		}
			// 		token0, token1 := d.getUniswapPairTokens(poolAddr)
			// 		crumb := hansel_lite.Breadcrumb{
			// 			FeeNumerator: poolInfo.FeeNumerator,
			// 			PoolType:     uint8(poolInfo.Type),
			// 		}
			// 		if amountIn0.BitLen() == 0 {
			// 			// Target goes from 1 -> 0, so we go from 0 -> 1
			// 			crumb.TokenFrom, crumb.TokenTo = token0, token1
			// 		} else {
			// 			crumb.TokenFrom, crumb.TokenTo = token1, token0
			// 		}
			// 		copy(crumb.PoolId[:], poolAddr.Bytes())
			// 		crumbs = append(crumbs, crumb)
			// 	}
		}
	})
	txWTL.Log.RecordTime(dexter.TxExecuteFinished)
	for _, poolId := range updatedPools {
		evm := d.getEvm(statedb, evmStateReader, lastBlockIdx)
		var poolAddr common.Address = common.BytesToAddress(poolId[:20])
		pool := d.getPoolTokensFromEvm(poolId, evm)
		u := dexter.PoolUpdate{
			Addr:     poolAddr,
			Reserves: make(map[common.Address]*big.Int),
		}
		for i, tokAddr := range pool.Tokens {
			u.Reserves[tokAddr] = pool.Balances[i]
		}
		ptxUpdates = append(ptxUpdates, u)
	}
	for poolAddr, t := range updatedCurvePools {
		evm := d.getEvm(statedb, d.evmState.evmStateReader, d.evmState.bs.LastBlock.Idx)
		tokens := d.getTokensCurveFromEvm(poolAddr, t, evm)
		balances := d.getBalancesCurveFromEvm(poolAddr, t, evm)
		u := dexter.PoolUpdate{
			Addr:     poolAddr,
			Reserves: make(map[common.Address]*big.Int),
		}
		for i, tokAddr := range tokens {
			u.Reserves[tokAddr] = balances[i]
		}
		ptxUpdates = append(ptxUpdates, u)
	}
	txWTL.Log.RecordTime(dexter.TxPoolReservesUpdated)
	if len(ptxUpdates) > 0 {
		// runtime.LockOSThread()
		for _, s := range d.strategies {
			ptx := &dexter.PossibleTx{
				Tx:        txWTL.Tx,
				StartTime: start,
				Updates:   ptxUpdates,
				Log:       make(dexter.TimeLog, len(dexter.TimingMomentLabels)),
			}
			copy(ptx.Log, txWTL.Log)
			s.ProcessPossibleTx(ptx)
		}
		// runtime.UnlockOSThread()
	}
	if true || len(crumbs) == 0 { // No Hansel
		d.methodist.Record(dexter.MethodEvent{method, dexter.Pending, txWTL.Tx.Hash()})
		return
	}
	d.methodist.Record(dexter.MethodEvent{method, dexter.Pending, txWTL.Tx.Hash()})
	evm := d.getEvm(statedb, evmStateReader, lastBlockIdx)
	msg := d.readOnlyMessage(&hanselSearchAddr, hansel_lite.FindRoute(crumbs))
	hanselStart := time.Now()
	gp := new(evmcore.GasPool).AddGas(math.MaxUint64)
	result, err := evmcore.ApplyMessage(evm, msg, gp)
	if err != nil {
		log.Error("Hansel error", "err", err)
		return
	}
	path, profit := hansel_lite.UnpackFindRoute(result.ReturnData)
	if profit == nil {
		return
	}
	profitFloat := dexter.BigIntToFloat(profit)
	gasEstimate := dexter.BigIntToFloat(txWTL.Tx.GasPrice()) * GAS_HANSEL
	if profitFloat < gasEstimate {
		return
	}
	log.Info("Hansel computed profitable path", "time", utils.PrettyDuration(time.Now().Sub(hanselStart)), "cumulative time", utils.PrettyDuration(time.Now().Sub(start)), "profit", profitFloat/1e18, "gas", gasEstimate/1e18)
	d.prepAndFirePlan(&dexter.RailgunPacket{
		Type:       dexter.HanselSwapLinear,
		StrategyID: len(d.strategies),
		Target:     txWTL.Tx,
		Response: &dexter.Plan{
			AmountIn:  startTokensIn,
			GasPrice:  txWTL.Tx.GasPrice(),
			GasCost:   dexter.FloatToBigInt(gasEstimate),
			NetProfit: dexter.FloatToBigInt(profitFloat - gasEstimate),
			MinProfit: dexter.FloatToBigInt(gasEstimate),
		},
		HanselResponse: &dexter.HanselPlan{
			Path: path,
		},
		StartTime: start,
	})
}

func getReservesFromSyncLog(l *types.Log) (*common.Address, *big.Int, *big.Int) {
	if len(l.Topics) != 1 || (bytes.Compare(l.Topics[0].Bytes(), syncEventTopic) != 0 && bytes.Compare(l.Topics[0].Bytes(), syncSolidlyEventTopic) != 0) {
		return nil, nil, nil
	}
	reserve0 := new(big.Int).SetBytes(l.Data[:32])
	reserve1 := new(big.Int).SetBytes(l.Data[32:])
	return &l.Address, reserve0, reserve1
}

func getAmountsFromSwapLog(l *types.Log) (*common.Address, *big.Int, *big.Int, *big.Int, *big.Int) {
	if len(l.Topics) != 3 || bytes.Compare(l.Topics[0].Bytes(), swapUniswapEventTopic) != 0 {
		return nil, nil, nil, nil, nil
	}
	amount0In := new(big.Int).SetBytes(l.Data[:32])
	amount1In := new(big.Int).SetBytes(l.Data[32:64])
	amount0Out := new(big.Int).SetBytes(l.Data[64:96])
	amount1Out := new(big.Int).SetBytes(l.Data[96:])
	return &l.Address, amount0In, amount1In, amount0Out, amount1Out
}

func (d *Dexter) prepAndFirePlan(p *dexter.RailgunPacket) {
	p.Log.RecordTime(dexter.PrepAndFirePlanStarted)
	log.Info("Ready to fire gun, returning")
	return
	bravado := d.strategyBravado[p.StrategyID]
	probAdjustedPayoff := new(big.Int).Mul(p.Response.NetProfit, big.NewInt(int64(d.accuracy*bravado)))
	failCost := new(big.Int).Mul(p.Response.GasPrice, big.NewInt(dexter.GAS_FAIL))
	probAdjustedFailCost := new(big.Int).Mul(failCost, big.NewInt(int64(1e6-(d.accuracy*bravado))))
	if probAdjustedPayoff.Cmp(probAdjustedFailCost) == -1 {
		log.Info("Trade unprofitable after adjusting for accuracy", "accuracy", d.accuracy, "bravado", bravado, "probAdjustedPayoff", dexter.BigIntToFloat(probAdjustedPayoff)/1e18, "lose", dexter.BigIntToFloat(probAdjustedFailCost)/1e18, "accuracy", d.accuracy, "bravado", bravado, "unadjusted win", dexter.BigIntToFloat(p.Response.NetProfit)/1e18, "unadjusted lose", dexter.BigIntToFloat(failCost)/1e18)
		return
	}
	from, _ := types.Sender(d.signer, p.Target)
	d.validatorMu.RLock()
	validatorIDs := d.predictValidators(from, p.Target.Nonce(), d.validators, d.epoch, numValidators)
	p.ValidatorIDs = validatorIDs
	d.validatorMu.RUnlock()
	p.Log.RecordTime(dexter.ValidatorsPredicted)
	if len(validatorIDs) == 0 {
		log.Warn("No predicted validators")
		return
	}
	d.gunMu.Lock() // LOCK MUTEX
	gunIdx := d.gunList.Search(validatorIDs)
	if gunIdx < 0 {
		log.Warn("Gun list not initialized")
		d.gunMu.Unlock() // UNLOCK MUTEX
		return
	}
	gun := d.gunList[gunIdx]
	if gun.ValidatorIDs[0] != validatorIDs[0] {
		log.Warn("Could not find validator", "validator", validatorIDs[0])
		d.gunMu.Unlock() // UNLOCK MUTEX
		return
	}
	d.gunList = d.gunList.Del(gunIdx)
	wallet := gun.Wallet
	account := wallet.Accounts()[0]
	// if d.svc.txpool.NumFrom(account.Address) > 0 {
	// 	d.gunMu.Unlock() // UNLOCK MUTEX
	// 	return
	// }
	if lastFiredTime, ok := d.gunLastFired[account.Address]; ok && time.Now().Sub(lastFiredTime) < time.Second {
		log.Info("Gun already fired, returning", "gun", account.Address, "lastFired", utils.PrettyDuration(time.Now().Sub(lastFiredTime)))
		d.gunMu.Unlock() // UNLOCK MUTEX
		return
	}
	d.gunMu.Unlock() // UNLOCK MUTEX
	p.Log.RecordTime(dexter.GunSelected)
	nonce := d.svc.txpool.Nonce(account.Address)
	p.Log.RecordTime(dexter.NonceLocated)
	var callData []byte
	var toAddr *common.Address
	if p.Type == dexter.SwapSinglePath {
		// fishCall = fish4_lite.SwapLinear(big.NewInt(0), p.Response.MinProfit, p.Response.Path, fishAddr)
		// callData = fish5_lite.SwapLinear(p.Response.AmountIn, p.Response.MinProfit, p.Response.Path, fishAddr)
		// callData = fish7_lite.SwapLinear(p.Target.Hash(), p.Response.AmountIn, p.Response.MinProfit, p.Response.Path, fishAddr)
		callData = fish8_lite.SwapLinear(
			p.Target.Hash(), p.Response.AmountIn, p.Response.MinProfit, p.Response.Path, fishAddr,
			big.NewInt(time.Now().Unix()+60))
		toAddr = &fishAddr
	} else if p.Type == dexter.HanselSwapLinear {
		callData = hansel_lite.SwapLinear(p.Response.AmountIn, p.Response.MinProfit, p.HanselResponse.Path, hanselAddr)
		toAddr = &hanselAddr
	} else {
		log.Error("Unsupported call type", "type", p.Type)
		return
	}
	gas := uint64(dexter.BigIntToFloat(p.Response.GasCost) / dexter.BigIntToFloat(p.Response.GasPrice) * 1.6)
	var responseTx *types.Transaction
	if p.Target.Type() == types.LegacyTxType {
		responseTx = types.NewTransaction(nonce, *toAddr, common.Big0, gas, p.Response.GasPrice, callData)
	} else {
		responseTx = types.NewTx(&types.DynamicFeeTx{
			ChainID:   p.Target.ChainId(),
			Nonce:     nonce,
			GasTipCap: p.Target.GasTipCap(),
			GasFeeCap: p.Target.GasFeeCap(),
			Gas:       gas,
			To:        toAddr,
			Value:     common.Big0,
			Data:      callData,
		})
	}
	p.Log.RecordTime(dexter.ResponseTxCreated)
	signedTx, err := wallet.SignTx(account, responseTx, d.svc.store.GetRules().EvmChainConfig().ChainID)
	p.Log.RecordTime(dexter.ResponseTxSigned)
	if err != nil {
		log.Error("Could not sign tx", "err", err)
		return
	}
	var label string
	if p.Target != nil {
		label = p.Target.Hash().Hex()
		// d.ignoreTxs[p.Target.Hash()] = struct{}{} This is a bad spot, concurrent map access.
	} else {
		label = "block state"
	}
	lag := time.Now().Sub(p.Target.Time())
	peers := d.tournament.GetSortedPeers(validatorIDs[0])
	// log.Info("Got sorted peers for validator", "vid", validatorIDs[0], "len", len(peers))
	log.Info("FIRING GUN pew pew",
		"profit", dexter.BigIntToFloat(p.Response.NetProfit)/1e18,
		"lag", utils.PrettyDuration(time.Now().Sub(p.StartTime)),
		"total", utils.PrettyDuration(lag),
		"mtts", utils.PrettyDuration(d.mtts[p.StrategyID]),
		"strategy", d.strategies[p.StrategyID].GetName(),
		"hash", signedTx.Hash().Hex(),
		"gas", p.Response.GasPrice)
	p.Log.RecordTime(dexter.GunFireStarted)
	d.svc.handler.BroadcastTxsAggressive([]*types.Transaction{p.Target, signedTx}, BroadcastAll, peers)
	p.Log.RecordTime(dexter.GunFireComplete)
	var method [4]byte
	copy(method[:], p.Target.Data()[:4])
	fmt.Printf("Fired: %s source: %s  method: %v \n%s\n", signedTx.Hash().Hex(), p.Target.Hash().Hex(), method, p.Log.Format())
	go d.accountFiredGun(wallet, signedTx, p, gun.ValidatorIDs, label, lag)
}

func (d *Dexter) runRailgun() {
	for {
		select {
		case <-d.inEpochChan:
			validators, epoch := d.svc.store.GetEpochValidators()
			d.validatorMu.Lock()
			d.validators, d.epoch = validators, epoch
			d.validatorMu.Unlock()
			go d.refreshGuns()
		case <-d.inEpochSub.Err():
			return
		case guns := <-d.gunRefreshes:
			d.gunMu.Lock()
			d.gunList = guns
			d.gunMu.Unlock()
		case p := <-d.railgunChan:
			p.Log.RecordTime(dexter.RailgunReceived)
			go d.prepAndFirePlan(p)
		}
	}
}

func (d *Dexter) accountFiredGun(wallet accounts.Wallet, signedTx *types.Transaction, p *dexter.RailgunPacket, gunValidatorIDs []idx.ValidatorID, label string, lag time.Duration) {
	// d.svc.handler.BroadcastTxsAggressive([]*types.Transaction{p.Target, signedTx})
	// d.svc.handler.BroadcastTxsAggressive([]*types.Transaction{signedTx})
	d.svc.txpool.AddLocal(signedTx)
	if p.Target != nil {
		select {
		case d.watchedTxs <- &TxSub{
			Hash:                p.Target.Hash(),
			Label:               "target",
			PredictedValidators: p.ValidatorIDs,
			Print:               true,
			StartTime:           time.Now(),
		}:
		default:
		}
	}
	select {
	case d.watchedTxs <- &TxSub{
		Hash:                signedTx.Hash(),
		Label:               label,
		PredictedValidators: gunValidatorIDs,
		TargetValidators:    p.ValidatorIDs,
		Print:               true,
		StartTime:           time.Now(),
	}:
	default:
	}
	var method [4]byte
	copy(method[:], p.Target.Data()[:4])
	d.methodist.Record(dexter.MethodEvent{method, dexter.Fired, p.Target.Hash()})
	select {
	case d.firedTxChan <- &FiredTx{
		Hash:         signedTx.Hash(),
		StrategyID:   p.StrategyID,
		Time:         time.Now(),
		Plan:         p.Response,
		TargetMethod: method,
	}:
	default:
	}
	d.mu.Lock()
	d.mtts[p.StrategyID] = time.Duration(float64(lag)*mttsAlpha + float64(d.mtts[p.StrategyID])*(1-mttsAlpha))
	d.numFired[p.StrategyID]++
	d.numFiredThisBlock++
	d.mu.Unlock()
}

func (d *Dexter) watchEvents() {
	watchedTxMap := make(map[common.Hash]*TxSub)
	gasAlpha1 := int64(100.0 * gasAlpha)
	gasAlpha2 := int64(100.0 * (1.0 - gasAlpha))
	for {
		select {
		case e := <-d.inEventChan:
			// log.Info("Received event", "lamport", e.Locator().Lamport, "creator", e.Locator().Creator)
			if len(e.Txs()) > 0 && d.lag < 4*time.Second {
				d.advanceEvmState(e)
			}
			interested := false
			interestedGas := make(map[int64]struct{})
			predictedValidators := make(map[common.Hash][]idx.ValidatorID)
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
						predictedValidators[tx.Hash()] = sub.PredictedValidators
						if sub.Label == "target" {
							txLabels[tx.Hash()] = "TARGET"
						} else {
							txLabels[tx.Hash()] = sub.Label
						}
					}
					delete(watchedTxMap, tx.Hash())
					select {
					case d.clearIgnoreTxChan <- tx.Hash():
					default:
					}
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
					log.Info("TX in event block", "gasTip", effectiveGasTip, "creator", e.Locator().Creator, "predicted", predictedValidators[tx.Hash()], "to", tx.To(), "hash", tx.Hash().Hex(), "label", txLabel)
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
		var pool struct {
			Tokens          []common.Address
			Balances        []*big.Int
			LastChangeBlock *big.Int
		}
		if poolType == dexter.BalancerWeightedPool || poolType == dexter.BalancerStablePool {
			pool = d.getPoolTokens(poolId)
		} else {
			pool.Tokens = d.getTokensCurve(common.BytesToAddress(poolId[:20]), poolType)
			pool.Balances = d.getBalancesCurve(common.BytesToAddress(poolId[:20]), poolType)
			log.Info("Gotten poolInfo for curve pool", "tokens", pool.Tokens, "balances", pool.Balances)
		}
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

func (d *Dexter) getSwapFeePercentage(addr *common.Address, t dexter.PoolType) *big.Int {
	evm := d.getReadOnlyEvm()
	var msg types.Message
	if t == dexter.BalancerWeightedPool || t == dexter.BalancerStablePool {
		msg = d.readOnlyMessage(addr, getSwapFeePercentageAbi)
	} else if t == dexter.CurveBasePlainPool {
		msg = d.readOnlyMessage(&cRegistryAddr, curve_registry.GetFees(addr))
	} else if t == dexter.CurveFactoryPlainPool || t == dexter.CurveFactoryMetaPool {
		msg = d.readOnlyMessage(&cFactoryAddr, curve_factory.GetFees(addr))
	}
	gp := new(evmcore.GasPool).AddGas(math.MaxUint64)
	result, err := evmcore.ApplyMessage(evm, msg, gp)
	if err != nil {
		log.Error("getSwapFeePercentage error", "err", err)
		return nil
	}
	var fee *big.Int
	if t == dexter.BalancerWeightedPool || t == dexter.BalancerStablePool {
		fee = new(big.Int).SetBytes(result.ReturnData[:32])
	} else if t == dexter.CurveBasePlainPool {
		fee = curve_registry.UnpackGetFee(result.ReturnData)
	} else if t == dexter.CurveFactoryPlainPool || t == dexter.CurveFactoryMetaPool {
		fee = curve_factory.UnpackGetFee(result.ReturnData)
	}
	return fee
}

func (d *Dexter) getAmplificationParameter(addr common.Address, t dexter.PoolType) *big.Int {
	evm := d.getReadOnlyEvm()
	var msg types.Message
	if t == dexter.BalancerWeightedPool || t == dexter.BalancerStablePool {
		msg = d.readOnlyMessage(&addr, getAmplificationParameterAbi)
	} else if t == dexter.CurveBasePlainPool {
		msg = d.readOnlyMessage(&cRegistryAddr, curve_registry.GetAmplificationParameter(addr))
	} else if t == dexter.CurveFactoryPlainPool {
		msg = d.readOnlyMessage(&cFactoryAddr, curve_factory.GetAmplificationParameter(addr))
	}
	gp := new(evmcore.GasPool).AddGas(math.MaxUint64)
	result, err := evmcore.ApplyMessage(evm, msg, gp)
	if err != nil {
		log.Error("getAmplificationParameter error", "err", err)
		return nil
	}
	amp := new(big.Int).SetBytes(result.ReturnData[:32])
	if t == dexter.CurveBasePlainPool || t == dexter.CurveFactoryPlainPool || t == dexter.CurveFactoryMetaPool {
		amp = amp.Mul(amp, big.NewInt(1e3))
	}
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

func (d *Dexter) getTokensCurve(addr common.Address, t dexter.PoolType) []common.Address {
	evm := d.getReadOnlyEvm()
	return d.getTokensCurveFromEvm(addr, t, evm)
}

func (d *Dexter) getTokensCurveFromEvm(addr common.Address, t dexter.PoolType, evm *vm.EVM) []common.Address {
	var msg types.Message
	if t == dexter.CurveBasePlainPool {
		msg = d.readOnlyMessage(&cRegistryAddr, curve_registry.GetCoins(addr))
	} else if t == dexter.CurveFactoryPlainPool {
		msg = d.readOnlyMessage(&cFactoryAddr, curve_factory.GetCoins(addr))
	} else if t == dexter.CurveFactoryMetaPool {
		msg = d.readOnlyMessage(&cFactoryAddr, curve_factory.GetUnderlyingCoins(addr))
	}
	gp := new(evmcore.GasPool).AddGas(math.MaxUint64)
	result, err := evmcore.ApplyMessage(evm, msg, gp)
	if err != nil {
		log.Error("getTokensCurve error", "err", err)
		return nil
	}
	var tokens []common.Address
	if t == dexter.CurveBasePlainPool {
		tokens = curve_registry.UnpackGetCoins(result.ReturnData)
	} else if t == dexter.CurveFactoryPlainPool {
		tokens = curve_factory.UnpackGetCoins(result.ReturnData)
	} else if t == dexter.CurveFactoryMetaPool {
		tokens = curve_factory.UnpackGetUnderlyingCoins(result.ReturnData)
	}
	var filteredTokens []common.Address
	for _, tok := range tokens {
		if tok == nullAddr {
			// log.Info("Encountered nullAddr")
			continue
		}
		filteredTokens = append(filteredTokens, tok)
	}
	return filteredTokens
}

func (d *Dexter) getBalancesCurve(poolAddr common.Address, t dexter.PoolType) []*big.Int {
	evm := d.getReadOnlyEvm()
	return d.getBalancesCurveFromEvm(poolAddr, t, evm)
}

func (d *Dexter) getBalancesCurveFromEvm(poolAddr common.Address, t dexter.PoolType, evm *vm.EVM) []*big.Int {
	var msg types.Message
	if t == dexter.CurveBasePlainPool {
		msg = d.readOnlyMessage(&cRegistryAddr, curve_registry.GetBalances(poolAddr))
	} else if t == dexter.CurveFactoryPlainPool {
		msg = d.readOnlyMessage(&cFactoryAddr, curve_factory.GetBalances(poolAddr))
	}
	gp := new(evmcore.GasPool).AddGas(math.MaxUint64)
	result, err := evmcore.ApplyMessage(evm, msg, gp)
	if err != nil {
		log.Error("getBalancesCurve error", "err", err)
	}
	// log.Info("getBalancesCurve", "addr", bVaultAddr, "poolId", poolId, "result", result.ReturnData)
	var balances []*big.Int
	if t == dexter.CurveBasePlainPool {
		balances = curve_factory.UnpackGetBalances(result.ReturnData)
	} else if t == dexter.CurveFactoryPlainPool {
		balances = curve_factory.UnpackGetBalances(result.ReturnData)
	}
	var filteredBalances []*big.Int
	zero := big.NewInt(0)
	for _, bal := range balances {
		if bal.Cmp(zero) == 0 {
			continue
		}
		filteredBalances = append(filteredBalances, bal)
	}

	return filteredBalances
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

func (d *Dexter) readOnlyMessage(toAddr *common.Address, data []byte) types.Message {
	var accessList types.AccessList
	return types.NewMessage(ownerAddr, toAddr, 0, new(big.Int), math.MaxUint64, new(big.Int), new(big.Int), new(big.Int), data, accessList, true)
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
	return d.getEvm(statedb, evmStateReader, bs.LastBlock.Idx)
}

func (d *Dexter) getEvm(statedb *state.StateDB, evmStateReader *EvmStateReader, lastBlockIdx idx.Block) *vm.EVM {
	vmConfig := &opera.DefaultVMConfig
	txContext := vm.TxContext{
		Origin:   nullAddr,
		GasPrice: new(big.Int),
	}
	header := d.evmHeader(lastBlockIdx, evmStateReader)
	context := evmcore.NewEVMBlockContext(header, evmStateReader, nil)
	config := d.svc.store.GetRules().EvmChainConfig()
	return vm.NewEVM(context, txContext, statedb, config, *vmConfig)
}

func (d *Dexter) evmHeader(lastBlockIdx idx.Block, reader evmcore.DummyChain) *evmcore.EvmHeader {
	baseFee := d.svc.store.GetRules().Economy.MinGasPrice
	if !d.svc.store.GetRules().Upgrades.London {
		baseFee = nil
	}
	return &evmcore.EvmHeader{
		Number:     utils.U64toBig(uint64(lastBlockIdx + 1)),
		Hash:       common.Hash{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		ParentHash: reader.GetHeader(common.Hash{}, uint64(lastBlockIdx)).Hash,
		Root:       common.Hash{},
		Time:       inter.Timestamp(uint64(time.Now().UnixNano())),
		Coinbase:   common.Address{},
		GasLimit:   math.MaxUint64,
		GasUsed:    0,
		BaseFee:    baseFee,
	}
}

func (d *Dexter) evmBlockWith(txs types.Transactions, lastBlockIdx idx.Block, reader evmcore.DummyChain) *evmcore.EvmBlock {
	return evmcore.NewEvmBlock(d.evmHeader(lastBlockIdx, reader), txs)
}

func (d *Dexter) updateMethods() {
	for {
		time.Sleep(60 * time.Second)
		// var black []dexter.Method
		white, black := d.methodist.GetLists()
		d.svc.txpool.UpdateMethods(white, black)
	}
}
