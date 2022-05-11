package dexter

import (
	"math/big"

	"github.com/Fantom-foundation/go-opera/contracts/fish3_lite"
	"github.com/Fantom-foundation/lachesis-base/inter/idx"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

var (
	startTokensIn = new(big.Int).Exp(big.NewInt(10), big.NewInt(19), nil)
	MaxAmountIn   = new(big.Int).Mul(big.NewInt(2021), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
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

type PairKey [60]byte

type PossibleTx struct {
	Tx             *types.Transaction
	Updates        []PairUpdate
	AvoidPairAddrs []*common.Address
	ValidatorIDs   []idx.ValidatorID
}

type PairUpdate struct {
	Addr     common.Address
	Reserves []*big.Int
}

type PoolInfo struct {
	Address      common.Address
	PoolType     string
	SwapFee      float64
	Reserves     []*big.Int
	Token0       common.Address
	Token1       common.Address
	FeeNumerator *big.Int
}

type PairInfo struct {
	Reserves     []*big.Int
	Token0       common.Address
	Token1       common.Address
	FeeNumerator *big.Int
}

type PairsInfoUpdate struct {
	PairsInfoUpdates map[common.Address]*PairInfo
	PairToRouteIdxs  map[PairKey][]uint
}

type Strategy interface {
	ProcessPossibleTx(ptx *PossibleTx)
	ProcessPermUpdates(us []*PairUpdate)
	GetInterestedPairs() map[common.Address]struct{}
	SetPairsInfo(pairsInfo map[common.Address]*PairInfo)
	Start()
	AddSubStrategy(Strategy)
}

type Plan struct {
	AmountIn  *big.Int
	GasPrice  *big.Int
	GasCost   *big.Int
	NetProfit *big.Int
	Path      []fish3_lite.SwapCommand
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

// Get the PairInfo from pairsInfoOverride first, and pairsInfo if not found in pairsInfoOverride.
// If PairsInfo is protected with a mutex, you must hold it while calling this.
// TODO: Check calls to this function to make sure they're mutexed.
func getPairInfo(pairsInfo, pairsInfoOverride map[common.Address]*PairInfo, pairAddr common.Address) *PairInfo {
	if pairsInfoOverride != nil {
		if pairInfo, ok := pairsInfoOverride[pairAddr]; ok {
			return pairInfo
		}
	}
	pairInfo := pairsInfo[pairAddr]
	return pairInfo
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
