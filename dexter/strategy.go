package dexter

import (
	"bytes"
	"math/big"

	"github.com/Fantom-foundation/go-opera/contracts/fish4_lite"
	"github.com/Fantom-foundation/lachesis-base/inter/idx"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

var (
	startTokensIn = new(big.Int).Exp(big.NewInt(10), big.NewInt(19), nil)
	MaxAmountIn   = new(big.Int).Mul(big.NewInt(2547), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
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

type FishCallType int

const (
	SwapSinglePath FishCallType = iota
)

type PoolKey [60]byte
type EdgeKey [40]byte

type PossibleTx struct {
	Tx             *types.Transaction
	Updates        []PoolUpdate
	AvoidPoolAddrs []*common.Address
	ValidatorIDs   []idx.ValidatorID
}

type PoolUpdate struct {
	Addr     common.Address
	Reserves []*big.Int
}

type PoolInfo struct {
	Reserves     map[common.Address]*big.Int
	Tokens       []common.Address
	Weights      []*big.Int
	FeeNumerator *big.Int
}

type PoolsInfoUpdate struct {
	PoolsInfoUpdates map[common.Address]*PoolInfo
	PoolToRouteIdxs  map[PoolKey][]uint
	AggregatePools   map[EdgeKey]*PoolInfo
}

type Strategy interface {
	ProcessPossibleTx(ptx *PossibleTx)
	ProcessPermUpdates(us []*PoolUpdate)
	GetInterestedPools() map[common.Address]struct{}
	SetPoolsInfo(poolsInfo map[common.Address]*PoolInfo)
	SetEdgePools(edgePools map[EdgeKey][]common.Address)
	Start()
	AddSubStrategy(Strategy)
}

type Plan struct {
	AmountIn  *big.Int
	GasPrice  *big.Int
	GasCost   *big.Int
	NetProfit *big.Int
	MinProfit *big.Int
	Path      []fish4_lite.LinearSwapCommand
}

type LegacyLegJson struct {
	From         string `json:from`
	To           string `json:to`
	PairAddr     string `json:pairAddr`
	ExchangeType string `json:exchangeType`
}

type LegacyRouteCacheJson struct {
	Routes          [][]LegacyLegJson `json:routes`
	PoolToRouteIdxs map[string][]uint `json:pairToRouteIdxs`
}

type LegJson struct {
	From         string `json:from`
	To           string `json:to`
	PoolAddr     string `json:poolAddr`
	ExchangeType string `json:exchangeType`
}

type RouteCacheJson struct {
	Routes          [][]LegJson       `json:routes`
	PoolToRouteIdxs map[string][]uint `json:pairToRouteIdxs`
}

type RouteCache struct {
	Routes          [][]*Leg
	PoolToRouteIdxs map[PoolKey][]uint
	Scores          []uint64
}

type Leg struct {
	From         common.Address
	To           common.Address
	PoolAddr     common.Address
	ExchangeType string
}

type RailgunPacket struct {
	Type         FishCallType
	Target       *types.Transaction
	StrategyID   int
	Response     *Plan
	ValidatorIDs []idx.ValidatorID
}

func estimateFishGas(numTransfers, numSwaps int, gasPrice *big.Int) *big.Int {
	gas := GAS_INITIAL + (GAS_TRANSFER * numTransfers) + (GAS_SWAP * numSwaps)
	return new(big.Int).Mul(gasPrice, big.NewInt(int64(gas)))
}

func estimateFailureCost(gasPrice *big.Int) *big.Int {
	return new(big.Int).Mul(gasPrice, big.NewInt(GAS_FAIL*1))
}

func getAmountOutUniswap(amountIn, reserveIn, reserveOut, feeNumerator *big.Int) *big.Int {
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

// Get the PoolInfo from poolsInfoOverride first, and poolsInfo if not found in poolsInfoOverride.
// If PoolsInfo is protected with a mutex, you must hold it while calling this.
// TODO: Check calls to this function to make sure they're mutexed.
func getPoolInfo(poolsInfo, poolsInfoOverride map[common.Address]*PoolInfo, poolAddr common.Address) *PoolInfo {
	if poolsInfoOverride != nil {
		if poolInfo, ok := poolsInfoOverride[poolAddr]; ok {
			return poolInfo
		}
	}
	poolInfo := poolsInfo[poolAddr]
	return poolInfo
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

func poolKeyFromStrs(from, to, poolAddr string) PoolKey {
	return poolKeyFromAddrs(common.HexToAddress(from), common.HexToAddress(to), common.HexToAddress(poolAddr))
}

func poolKeyFromAddrs(from, to, poolAddr common.Address) PoolKey {
	var key PoolKey
	copy(key[:20], from[:])
	copy(key[20:], to[:])
	copy(key[40:], poolAddr[:])
	return key
}

func MakeEdgeKey(a, b common.Address) (key EdgeKey) {
	if bytes.Compare(a.Bytes(), b.Bytes()) < 0 {
		copy(key[:20], a[:])
		copy(key[20:], b[:])
	} else {
		copy(key[:20], b[:])
		copy(key[20:], a[:])
	}
	return key
}

func refreshAggregatePool(key EdgeKey, pools []common.Address, poolsInfo, poolsInfoOverride map[common.Address]*PoolInfo) *PoolInfo {
	token0 := common.BytesToAddress(key[:20])
	token1 := common.BytesToAddress(key[20:])
	agg := &PoolInfo{
		Reserves: map[common.Address]*big.Int{token0: big.NewInt(0), token1: big.NewInt(0)},
		Tokens:   []common.Address{token0, token1},
	}
	for _, poolAddr := range pools {
		pi := getPoolInfo(poolsInfo, poolsInfoOverride, poolAddr)
		agg.Reserves[Tokens[0]].Add(agg.Reserves[Tokens[0]], pi.Reserves[Tokens[0]])
		agg.Reserves[Tokens[1]].Add(agg.Reserves[Tokens[1]], pi.Reserves[Tokens[1]])
	}
	return agg
}

func makeAggregatePools(edgePools map[EdgeKey][]common.Address, poolsInfo, poolsInfoOverride map[common.Address]*PoolInfo) map[EdgeKey]*PoolInfo {
	aggs := make(map[EdgeKey]*PoolInfo)
	for key, pools := range edgePools {
		aggs[key] = refreshAggregatePool(key, pools, poolsInfo, poolsInfoOverride)
	}
	return aggs
}

func convert(fromToken, toToken common.Address, amount *big.Int, aggregatePools map[EdgeKey]*PoolInfo) *big.Int {
	if bytes.Compare(fromToken.Bytes(), toToken.Bytes()) == 0 {
		return amount
	}
	key := MakeEdgeKey(fromToken, toToken)
	pool, ok := aggregatePools[key]
	if !ok {
		//log.Error("Could not find aggregate pool", "key", key, "len", len(aggregatePools), "fromToken", fromToken, "toToken", toToken)
		return big.NewInt(0)
	}
	if bytes.Compare(fromToken.Bytes(), pool.Tokens[0].Bytes()) == 0 {
		x := big.NewInt(0).Mul(amount, pool.Reserves[1])
		return x.Div(x, pool.Reserves[0])
	} else {
		x := big.NewInt(0).Mul(amount, pool.Reserves[0])
		return x.Div(x, pool.Reserves[1])
	}
}
