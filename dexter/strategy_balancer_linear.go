package dexter

import (
	"bytes"
	"container/heap"
	"encoding/json"
	"io/ioutil"
	"math/big"
	"math/rand"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Fantom-foundation/go-opera/contracts/fish5_lite"
	"github.com/Fantom-foundation/go-opera/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

type BalancerLinearStrategy struct {
	Name                string
	ID                  int
	RailgunChan         chan *RailgunPacket
	inPossibleTxsChan   chan *PossibleTx
	inPermUpdatesChan   chan []*PoolUpdate
	cfg                 BalancerLinearStrategyConfig
	poolsInfo           map[common.Address]*PoolInfoFloat
	poolsInfoUpdateChan chan *PoolsInfoUpdateFloat
	interestedPairs     map[common.Address]PoolType
	interestedPools     map[BalPoolId]PoolType
	edgePools           map[EdgeKey][]common.Address
	aggregatePools      map[EdgeKey]*PoolInfoFloat
	routeCache          RouteCache
	subStrategies       []Strategy
	gasPrice            int64
	mu                  sync.RWMutex
}

type BalancerLinearStrategyConfig struct {
	RoutesFileName          string
	PoolToRouteIdxsFileName string
	SelectSecondBest        bool
}

func NewBalancerLinearStrategy(name string, id int, railgun chan *RailgunPacket, cfg BalancerLinearStrategyConfig) Strategy {
	s := &BalancerLinearStrategy{
		Name:                name,
		ID:                  id,
		RailgunChan:         railgun,
		inPossibleTxsChan:   make(chan *PossibleTx, 256),
		inPermUpdatesChan:   make(chan []*PoolUpdate, 256),
		cfg:                 cfg,
		poolsInfo:           make(map[common.Address]*PoolInfoFloat),
		poolsInfoUpdateChan: make(chan *PoolsInfoUpdateFloat),
		interestedPairs:     make(map[common.Address]PoolType),
		interestedPools:     make(map[BalPoolId]PoolType),
		aggregatePools:      make(map[EdgeKey]*PoolInfoFloat),
	}
	s.loadJson()
	return s
}

func (s *BalancerLinearStrategy) SetPoolsInfo(poolsInfo map[common.Address]*PoolInfo) {
	for k, v := range poolsInfo {
		reserves := make(map[common.Address]float64)
		weights := make(map[common.Address]float64)
		scaleFactors := make(map[common.Address]float64)
		for a, r := range v.Reserves {
			scaleFactors[a] = BigIntToFloat(v.ScaleFactors[a])
			weights[a] = BigIntToFloat(v.Weights[a])
			if v.Type == BalancerWeightedPool || v.Type == BalancerStablePool {
				reserves[a] = upScale(BigIntToFloat(r), scaleFactors[a])
			} else {
				reserves[a] = BigIntToFloat(r)
			}
		}

		poolInfo := &PoolInfoFloat{
			Tokens:             v.Tokens,
			Reserves:           reserves,
			Weights:            weights,
			ScaleFactors:       scaleFactors,
			FeeNumerator:       BigIntToFloat(v.FeeNumerator),
			FeeNumeratorBI:     v.FeeNumerator,
			Fee:                BigIntToFloat(v.Fee) / 1e18,
			FeeBI:              v.Fee,
			LastUpdate:         time.Now(),
			AmplificationParam: BigIntToFloat(v.AmplificationParam),
			Type:               v.Type,
		}
		if poolInfo.Type == BalancerStablePool {
			log.Info("Created stable pool", "pi", poolInfo)
		}
		s.poolsInfo[k] = poolInfo
	}
}

func (s *BalancerLinearStrategy) SetEdgePools(edgePools map[EdgeKey][]common.Address) {
	s.edgePools = edgePools
}

func (s *BalancerLinearStrategy) SetGasPrice(gasPrice int64) {
	s.gasPrice = gasPrice
}

func (s *BalancerLinearStrategy) ProcessPossibleTx(t *PossibleTx) {
	s.inPossibleTxsChan <- t
}

func (s *BalancerLinearStrategy) ProcessPermUpdates(us []*PoolUpdate) {
	s.inPermUpdatesChan <- us
}

func (s *BalancerLinearStrategy) GetInterestedPools() (map[common.Address]PoolType, map[BalPoolId]PoolType) {
	return s.interestedPairs, s.interestedPools
}

func (s *BalancerLinearStrategy) AddSubStrategy(sub Strategy) {
	s.subStrategies = append(s.subStrategies, sub)
}

func (s *BalancerLinearStrategy) Start() {
	s.aggregatePools = makeAggregatePoolsFloat(s.edgePools, s.poolsInfo, nil)
	s.routeCache.Scores = s.makeScores()
	go s.runPermUpdater()
	go s.runStrategy()
}

func (s *BalancerLinearStrategy) loadJson() {
	log.Info("Loading routes")
	routeCacheRoutesFile, err := os.Open(s.cfg.RoutesFileName)
	if err != nil {
		log.Info("Error opening routeCacheRoutes", "routeCacheRoutesFileName", s.cfg.RoutesFileName, "err", err)
		return
	}
	defer routeCacheRoutesFile.Close()
	routeCacheRoutesBytes, _ := ioutil.ReadAll(routeCacheRoutesFile)
	var routeCacheJson RouteCacheJson
	json.Unmarshal(routeCacheRoutesBytes, &(routeCacheJson.Routes))
	log.Info("Loaded routes")
	routeCachePoolToRouteIdxsFile, err := os.Open(s.cfg.PoolToRouteIdxsFileName)
	if err != nil {
		log.Info("Error opening routeCachePoolToRouteIdxs", "routeCachePoolToRouteIdxsFileName", s.cfg.PoolToRouteIdxsFileName, "err", err)
		return
	}
	defer routeCachePoolToRouteIdxsFile.Close()
	routeCachePoolToRouteIdxsBytes, _ := ioutil.ReadAll(routeCachePoolToRouteIdxsFile)
	json.Unmarshal(routeCachePoolToRouteIdxsBytes, &(routeCacheJson.PoolToRouteIdxs))
	log.Info("Loaded poolToRouteIdxs")
	routeCache := RouteCache{
		Routes:          make([][]*Leg, len(routeCacheJson.Routes)),
		PoolToRouteIdxs: make(map[PoolKey][]uint),
		LastFiredTime:   make([]time.Time, len(routeCacheJson.Routes)),
	}
	for i, routeJson := range routeCacheJson.Routes {
		route := make([]*Leg, len(routeJson))
		routeCache.LastFiredTime[i] = time.Now()
		for x, leg := range routeJson {
			poolAddr := common.HexToAddress(leg.PoolAddr)
			var poolId BalPoolId
			copy(poolId[:], common.FromHex(leg.PoolId))
			var t PoolType
			if leg.ExchangeType == "balancerWeightedPool" {
				t = BalancerWeightedPool
				s.interestedPools[poolId] = t
			} else if leg.ExchangeType == "balancerStablePool" {
				t = BalancerStablePool
				s.interestedPools[poolId] = t
			} else {
				t = UniswapV2Pair
				s.interestedPairs[poolAddr] = t
			}
			route[x] = &Leg{
				From:     common.HexToAddress(leg.From),
				To:       common.HexToAddress(leg.To),
				PoolAddr: poolAddr,
				Type:     t,
				PoolId:   poolId,
			}
		}
		routeCache.Routes[i] = route
	}
	for strKey, routeIdxs := range routeCacheJson.PoolToRouteIdxs {
		parts := strings.Split(strKey, "_")
		key := poolKeyFromStrs(parts[0], parts[1], parts[2])
		routeCache.PoolToRouteIdxs[key] = routeIdxs
	}
	log.Info("Processed route cache", "name", s.Name)
	s.routeCache = routeCache
}

func (s *BalancerLinearStrategy) getRouteAmountOutBalancer(
	route []*Leg, amountIn float64, poolsInfoOverride map[common.Address]*PoolInfoFloat, debug bool) float64 {
	var amountOut float64
	if debug {
		log.Info("getRouteAmountOutBalancer", "route", route)
	}
	for i, leg := range route {
		s.mu.RLock()
		poolInfo := getPoolInfoFloat(s.poolsInfo, poolsInfoOverride, leg.PoolAddr)
		s.mu.RUnlock()
		reserveFrom, reserveTo := poolInfo.Reserves[leg.From], poolInfo.Reserves[leg.To]
		if leg.Type == UniswapV2Pair {
			amountOut = getAmountOutUniswapFloat(amountIn, reserveFrom, reserveTo, poolInfo.FeeNumerator)
			if debug {
				log.Info("Leg: uniswap", "i", i, "reserveFrom", reserveFrom, "reserveTo", reserveTo, "feeNumerator", poolInfo.FeeNumerator, "amountIn", amountIn, "amountOut", amountOut)
			}
		} else if leg.Type == BalancerWeightedPool {
			weightFrom, weightTo := poolInfo.Weights[leg.From], poolInfo.Weights[leg.To]
			amountOut = getAmountOutBalancer(amountIn, reserveFrom, reserveTo, weightFrom, weightTo, poolInfo.Fee, poolInfo.ScaleFactors[leg.From], poolInfo.ScaleFactors[leg.To])
			if debug {
				log.Info("Leg: balancerWeighted", "i", i, "reserveFrom", reserveFrom, "reserveTo", reserveTo, "feeNumerator", poolInfo.Fee, "weightFrom", weightFrom, "weightTo", weightTo, "amountIn", amountIn, "amountOut", amountOut)
			}
		} else if leg.Type == BalancerStablePool {
			amountOut = getAmountOutBalancerStable(
				amountIn, poolInfo.Fee, poolInfo.AmplificationParam, poolInfo.Reserves, leg.From, leg.To, poolInfo.ScaleFactors[leg.From], poolInfo.ScaleFactors[leg.To])
			if debug {
				log.Info("Leg: balancerStable", "i", i, "reserveFrom", reserveFrom, "reserveTo", reserveTo, "feeNumerator", poolInfo.Fee, "amountIn", amountIn, "amountOut", amountOut)
				for addr, bal := range poolInfo.Reserves {
					log.Info("Debug balancerStable reserves:", "addr", addr, "bal", bal)
				}
				log.Info("Amp", "amp", poolInfo.AmplificationParam)
			}
		}
		amountIn = amountOut
	}
	return amountOut
}

func calcStableInvariant(amp float64, balances map[common.Address]float64) float64 {
	ampPrecision := 1e3
	sum := 0.
	numToks := float64(len(balances))
	for _, bal := range balances {
		sum += bal
	}
	if sum == 0 {
		return 0
	}
	prevInv := 0.
	inv := sum
	ampTimesTotal := amp * numToks
	for i := 0; i < 255; i++ {
		P_D := 1.
		for _, bal := range balances {
			P_D = P_D * bal * numToks / inv
		}
		P_D *= inv
		prevInv = inv
		inv = (numToks*inv*inv + ampTimesTotal*sum*P_D/ampPrecision) / (inv*(numToks+1) + (ampTimesTotal-ampPrecision)*P_D/ampPrecision)
		if inv > prevInv {
			if inv-prevInv <= 1 {
				break
			}
		} else if prevInv-inv <= 1 {
			break
		}
	}
	return inv
}

func calcCurveInvariant(amp float64, balances map[common.Address]float64) float64 {
	ampPrecision := 1e3
	sum := 0.
	numToks := float64(len(balances))
	for _, bal := range balances {
		sum += bal
	}
	if sum == 0 {
		return 0
	}
	prevInv := 0.
	inv := sum
	ampTimesTotal := amp * numToks
	for i := 0; i < 255; i++ {
		D_P := inv
		for _, bal := range balances {
			D_P = D_P * inv / (numToks * bal)
		}
		prevInv = inv
		inv = (ampTimesTotal*sum/ampPrecision + D_P*numToks) * inv / ((ampTimesTotal-ampPrecision)*inv/ampPrecision + (numToks+1)*D_P)
		if inv > prevInv {
			if inv-prevInv <= 1 {
				break
			}
		} else if prevInv-inv <= 1 {
			break
		}
	}
	return inv
}

func getTokenBalanceGivenInvAndBalances(amp, inv float64, balances map[common.Address]float64, balanceIn float64, tokenIn, tokenOut common.Address) float64 {
	ampPrecision := 1e3
	numToks := float64(len(balances))
	ampTimesTotal := amp * numToks
	sum := 0.
	P_D := 1.
	for addr, bal := range balances {
		if bytes.Compare(addr.Bytes(), tokenIn.Bytes()) != 0 {
			P_D = P_D * bal * numToks / inv
			sum += bal
		}
	}
	P_D = P_D * balanceIn * numToks
	sum += balanceIn
	sum -= balances[tokenOut]
	c := inv * inv * balances[tokenOut] / (ampTimesTotal * P_D / ampPrecision)
	b := sum + (inv / ampTimesTotal * ampPrecision)
	prevBal := 0.
	currentBal := ((inv*inv+c)-1)/(inv+b) + 1
	for i := 0; i < 255; i++ {
		prevBal = currentBal
		currentBal = (currentBal*currentBal + c) / (2*currentBal + b - inv)
		if currentBal > prevBal {
			if currentBal-prevBal <= 1 {
				break
			}
		} else if prevBal-currentBal <= 1 {
			break
		}
	}
	return currentBal
}

func getTokenBalanceGivenInvAndBalancesCurve(amp, inv float64, balances map[common.Address]float64, balanceIn float64, tokenIn, tokenOut common.Address) float64 {
	ampPrecision := 1e3
	numToks := float64(len(balances))
	ampTimesTotal := amp * numToks
	sum := 0.
	_x := 0.
	c := inv
	for addr, bal := range balances {
		if bytes.Compare(addr.Bytes(), tokenIn.Bytes()) == 0 {
			_x = balanceIn
		} else if bytes.Compare(addr.Bytes(), tokenOut.Bytes()) != 0 {
			_x = bal
		} else {
			continue
		}
		sum += _x
		c = c * inv / (_x * numToks)
	}
	c = c * inv * ampPrecision / (ampTimesTotal * numToks)
	b := sum + (inv / ampTimesTotal * ampPrecision)
	prevBal := 0.
	currentBal := inv
	for i := 0; i < 255; i++ {
		prevBal = currentBal
		currentBal = (currentBal*currentBal + c) / (2*currentBal + b - inv)
		if currentBal > prevBal {
			if currentBal-prevBal <= 1 {
				break
			}
		} else if prevBal-currentBal <= 1 {
			break
		}
	}
	return currentBal
}

func getAmountOutBalancerStable(amountIn, fee, amp float64, balances map[common.Address]float64, tokenIn, tokenOut common.Address, scaleIn, scaleOut float64) float64 {
	amountIn = amountIn * (1 - fee)
	amountIn = upScale(amountIn, scaleIn)
	inv := calcStableInvariant(amp, balances)
	finalOut := getTokenBalanceGivenInvAndBalances(amp, inv, balances, balances[tokenIn]+amountIn, tokenIn, tokenOut)
	// fmt.Printf("balOut: %v, finalOut: %v\n", balances[tokenOut], finalOut)
	return downScale(balances[tokenOut]-finalOut-1, scaleOut)
}

func (s *BalancerLinearStrategy) getAmountOutCurveMeta(
	amountIn, scaleIn, scaleOut float64, tokenIn, tokenOut, metaAddr, baseAddr common.Address,
	underlyingBalances map[common.Address]float64, poolsInfoOverride map[common.Address]*PoolInfoFloat) float64 {
	s.mu.RLock()
	basePoolInfo := getPoolInfoFloat(s.poolsInfo, poolsInfoOverride, baseAddr)
	metaPoolInfo := getPoolInfoFloat(s.poolsInfo, poolsInfoOverride, metaAddr)
	s.mu.RUnlock()
	fee := metaPoolInfo.Fee
	amp := metaPoolInfo.AmplificationParam
	balances := metaPoolInfo.Reserves
	basePoolInv := calcStableInvariant(basePoolInfo.AmplificationParam, basePoolInfo.Reserves)
	// fmt.Printf("basePoolInv: %v, basePoolInfo.MetaTokenSupply: %v\n", basePoolInv, basePoolInfo.MetaTokenSupply)
	virtualP := basePoolInv / basePoolInfo.MetaTokenSupply
	tokens := make([]common.Address, len(balances))
	i := 0
	for addr := range balances {
		tokens[i] = addr
		i++
	}
	balances[tokens[len(tokens)-1]] = virtualP * balances[tokens[len(tokens)-1]]
	balanceIn := 0.
	baseTokenIn := tokens[0]
	baseTokenOut := tokens[0]
	metaTokenIn := tokens[0]
	metaTokenOut := tokens[0]
	if bytes.Compare(tokenIn.Bytes(), tokens[0].Bytes()) != 0 {
		baseTokenIn = tokenIn
		metaTokenIn = tokens[1]
	} else {
		baseTokenOut = tokenOut
		metaTokenOut = tokens[1]
	}
	if bytes.Compare(tokenIn.Bytes(), tokens[0].Bytes()) == 0 {
		balanceIn = balances[metaTokenIn] + upScale(amountIn, scaleIn)
	} else {
		if bytes.Compare(tokenOut.Bytes(), tokens[0].Bytes()) == 0 {
			inv0 := calcStableInvariant(basePoolInfo.AmplificationParam, basePoolInfo.Reserves)
			newBasePoolBals := make(map[common.Address]float64)
			for addr, amount := range basePoolInfo.Reserves {
				newBasePoolBals[addr] = amount
			}
			newBasePoolBals[baseTokenIn] += amountIn
			inv1 := calcStableInvariant(basePoolInfo.AmplificationParam, newBasePoolBals)
			balanceIn = (inv1 - inv0) * basePoolInfo.MetaTokenSupply / inv0
			balanceIn -= balanceIn * basePoolInfo.Fee / 2
			balanceIn += balances[tokens[len(tokens)]]
		} else {
			return getAmountOutBalancerStable(
				amountIn, basePoolInfo.Fee, basePoolInfo.AmplificationParam, basePoolInfo.Reserves, tokenIn, tokenOut, scaleIn, scaleOut)
		}
	}
	// for addr, bal := range balances {
	// 	fmt.Printf("addr: %v, bal: %v\n", addr, bal)
	// }
	// fmt.Printf("virtual Price: %v, balanceIn: %v\n", virtualP, balanceIn)
	inv := calcCurveInvariant(amp, balances)
	balanceOut := getTokenBalanceGivenInvAndBalancesCurve(
		amp, inv, balances, balanceIn, metaTokenIn, metaTokenOut)
	// fmt.Printf("metaIn: %v, metaOut: %v\n", metaTokenIn, metaTokenOut)
	// fmt.Printf("balOut: %v, bals: %v\n", balanceOut, balances[metaTokenOut])
	amountOut := balances[metaTokenOut] - balanceOut - 1
	amountOut = amountOut - fee*amountOut
	// fmt.Printf("amountOut: %v\n", amountOut)
	if bytes.Compare(tokenOut.Bytes(), tokens[0].Bytes()) == 0 {
		amountOut = downScale(amountOut, scaleOut)
	} else {
		inv0 := calcStableInvariant(basePoolInfo.AmplificationParam, basePoolInfo.Reserves)
		inv1 := inv0 - amountOut*inv0/basePoolInfo.MetaTokenSupply
		// fmt.Printf("inv0: %v, inv1: %v\n", inv0, inv1)
		// fmt.Printf("amp: %v, inv: %v, baseTokenOut: %s\n", basePoolInfo.AmplificationParam, inv1, baseTokenOut)
		// for addr, bal := range basePoolInfo.Reserves {
		// 	fmt.Printf("addr: %v, bal: %v\n", addr, bal)
		// }
		newBalanceOut := getTokenBalanceGivenInvAndBalancesCurve( // Sketch call but should function
			basePoolInfo.AmplificationParam, inv1, basePoolInfo.Reserves, 0., common.HexToAddress("0x00"), baseTokenOut)
		fee := basePoolInfo.Fee * float64(len(basePoolInfo.Tokens)) / (4 * float64((len(basePoolInfo.Tokens))-1))
		balancesReduced := make(map[common.Address]float64)
		for addr, bal := range basePoolInfo.Reserves {
			amountInExpected := 0.
			if bytes.Compare(addr.Bytes(), baseTokenOut.Bytes()) == 0 {
				amountInExpected = bal*inv1/inv0 - newBalanceOut
			} else {
				amountInExpected = bal - bal*inv1/inv0
			}
			balancesReduced[addr] = bal - fee*amountInExpected
		}
		baseTokBalOut := getTokenBalanceGivenInvAndBalancesCurve( // Sketch call but should function
			basePoolInfo.AmplificationParam, inv1, balancesReduced, 0., common.HexToAddress("0x00"), baseTokenOut)
		amountOut = balancesReduced[baseTokenOut] - baseTokBalOut
		amountOut -= 1
	}
	return downScale(amountOut, scaleOut)
}

func (s *BalancerLinearStrategy) makeScores() []float64 {
	scores := make([]float64, len(s.routeCache.Routes))
	for i, route := range s.routeCache.Routes {
		scores[i] = s.getScore(route, nil)
	}
	return scores
}

func (s *BalancerLinearStrategy) getScore(route []*Leg, poolsInfoOverride map[common.Address]*PoolInfoFloat) float64 {
	unitPrice := s.getRouteAmountOutBalancer(route, unitIn, poolsInfoOverride, false) / unitIn
	return unitPrice
	// Spot price gradient method
	// score := 1.0
	// for _, leg := range route {
	// 	s.mu.RLock()
	// 	poolInfo := getPoolInfoFloat(s.poolsInfo, poolsInfoOverride, leg.PoolAddr)
	// 	s.mu.RUnlock()
	// 	reserveFrom, reserveTo := poolInfo.Reserves[leg.From], poolInfo.Reserves[leg.To]
	// 	if leg.Type == UniswapV2Pair {
	// 		score = score * (reserveTo * poolInfo.FeeNumerator) / (reserveFrom * 1e6)
	// 	} else if leg.Type == BalancerWeightedPool {
	// 		weightFrom, weightTo := poolInfo.Weights[leg.From], poolInfo.Weights[leg.To]
	// 		scaleFrom, scaleTo := poolInfo.ScaleFactors[leg.From], poolInfo.ScaleFactors[leg.To]
	// 		score = score * (downScale(reserveTo, scaleTo) * weightFrom) / (downScale(reserveFrom, scaleFrom) * weightTo) * (1 / (1 - poolInfo.Fee))
	// 	} else if leg.Type == BalancerStablePool {
	// 		scaleFrom, scaleTo := poolInfo.ScaleFactors[leg.From], poolInfo.ScaleFactors[leg.To]
	// 		score = score * (downScale(reserveTo, scaleTo) / downScale(reserveFrom, scaleFrom)) * (1 / (1 - poolInfo.Fee))
	// 	}
	// }
	// return score
}

func (s *BalancerLinearStrategy) refreshScoresForPools(
	keys map[PoolKey]struct{}, poolsInfoOverride map[common.Address]*PoolInfoFloat, start time.Time) {
	var allRouteIdxs []uint
	for key, _ := range keys {
		if routeIdxs, ok := s.routeCache.PoolToRouteIdxs[key]; ok {
			allRouteIdxs = append(allRouteIdxs, routeIdxs...)
		}
	}
	sort.Slice(allRouteIdxs, func(a, b int) bool { return a < b })
	allRouteIdxs = uniq(allRouteIdxs)
	// log.Info("Perm updater done uniq", "t", utils.PrettyDuration(time.Now().Sub(start)), "keys", len(keys))
	for _, routeIdx := range allRouteIdxs {
		route := s.routeCache.Routes[routeIdx]
		s.routeCache.Scores[routeIdx] = s.getScore(route, poolsInfoOverride)
	}
}

func (s *BalancerLinearStrategy) runPermUpdater() {
	for {
		poolsInfoUpdates := make(map[common.Address]*PoolInfoFloat)
		refreshKeys := make(map[PoolKey]struct{})
		us := <-s.inPermUpdatesChan
		start := time.Now()
	loop:
		for {
			select {
			case u2 := <-s.inPermUpdatesChan:
				us = append(us, u2...)
			default:
				break loop
			}
		}
		for _, u := range us {
			s.mu.RLock()
			poolInfo, ok := s.poolsInfo[u.Addr]
			s.mu.RUnlock()
			if !ok {
				continue
			}
			reserves := make(map[common.Address]float64)
			for a, r := range u.Reserves {
				if poolInfo.Type == BalancerWeightedPool || poolInfo.Type == BalancerStablePool {
					// log.Info("Updating balancer pool", "addr", u.Addr, "token", a, "from", downScale(poolInfo.Reserves[a], poolInfo.ScaleFactors[a]), "to", BigIntToFloat(r))
					reserves[a] = upScale(BigIntToFloat(r), poolInfo.ScaleFactors[a])
				} else {
					reserves[a] = BigIntToFloat(r)
				}
			}
			poolsInfoUpdates[u.Addr] = &PoolInfoFloat{
				Reserves:           reserves,
				Tokens:             poolInfo.Tokens,
				Weights:            poolInfo.Weights,
				ScaleFactors:       poolInfo.ScaleFactors,
				AmplificationParam: poolInfo.AmplificationParam,
				Fee:                poolInfo.Fee,
				FeeNumerator:       poolInfo.FeeNumerator,
				FeeBI:              poolInfo.FeeBI,
				FeeNumeratorBI:     poolInfo.FeeNumeratorBI,
				LastUpdate:         time.Now(),
				Type:               poolInfo.Type,
			}
			for _, token0 := range poolInfo.Tokens {
				for _, token1 := range poolInfo.Tokens {
					if token0 == token1 {
						continue
					}
					key := poolKeyFromAddrs(token0, token1, u.Addr)
					refreshKeys[key] = struct{}{}
				}
			}
		}
		if len(poolsInfoUpdates) > 0 {
			// poolToRouteIdxs := s.sortPoolToRouteIdxMap(scores, start)
			update := &PoolsInfoUpdateFloat{
				PoolsInfoUpdates: poolsInfoUpdates,
				// PoolToRouteIdxs:  poolToRouteIdxs,
				// Scores: make([]float64, len(scores)),
			}
			// copy(update.Scores, scores)
			s.poolsInfoUpdateChan <- update
			s.refreshScoresForPools(refreshKeys, poolsInfoUpdates, start)
			// log.Info("Perm updater done computing updates", "t", utils.PrettyDuration(time.Now().Sub(start)), "queue", len(s.inPermUpdatesChan))
		}
	}
}

func (s *BalancerLinearStrategy) runStrategy() {
	for {
		select {
		case update := <-s.poolsInfoUpdateChan:
			s.mu.Lock()
			// s.routeCache.PoolToRouteIdxs = update.PoolToRouteIdxs
			// s.routeCache.Scores = update.Scores
			for poolAddr, poolInfo := range update.PoolsInfoUpdates {
				s.poolsInfo[poolAddr] = poolInfo
			}
			s.mu.Unlock()
		case p := <-s.inPossibleTxsChan:
			s.processPotentialTx(p)
		}
	}
}

func (s *BalancerLinearStrategy) processPotentialTx(ptx *PossibleTx) {
	// start := time.Now()
	poolsInfoOverride := make(map[common.Address]*PoolInfoFloat)
	var updatedKeys []PoolKey
	for _, u := range ptx.Updates {
		s.mu.RLock()
		poolInfo, ok := s.poolsInfo[u.Addr]
		s.mu.RUnlock()
		if !ok {
			continue
		}
		reserves := make(map[common.Address]float64)
		for a, r := range u.Reserves {
			if poolInfo.Type == BalancerWeightedPool || poolInfo.Type == BalancerStablePool {
				reserves[a] = upScale(BigIntToFloat(r), poolInfo.ScaleFactors[a])
				// log.Info("Balancer update", "hash", ptx.Tx.Hash().Hex(), "pool", u.Addr, "token", a, "old reserve", poolInfo.Reserves[a], "new reserve", reserves[a])
			} else {
				reserves[a] = BigIntToFloat(r)
			}
		}
		poolsInfoOverride[u.Addr] = &PoolInfoFloat{
			Reserves:           reserves,
			Tokens:             poolInfo.Tokens,
			Weights:            poolInfo.Weights,
			ScaleFactors:       poolInfo.ScaleFactors,
			AmplificationParam: poolInfo.AmplificationParam,
			Fee:                poolInfo.Fee,
			FeeNumerator:       poolInfo.FeeNumerator,
			FeeBI:              poolInfo.FeeBI,
			FeeNumeratorBI:     poolInfo.FeeNumeratorBI,
			LastUpdate:         time.Now(),
			Type:               poolInfo.Type,
		}
		for _, token0 := range poolInfo.Tokens {
			for _, token1 := range poolInfo.Tokens {
				if token0 == token1 {
					continue
				}
				updatedKeys = append(updatedKeys, poolKeyFromAddrs(token0, token1, u.Addr))
			}
		}
	}
	var pop Population
	candidateRoutes := 0
	// FIXME TODO: Combine the route idxs that show up in multiple keys.
	for _, key := range updatedKeys {
		pop = append(pop, s.getProfitableRoutes(key, poolsInfoOverride)...)
		candidateRoutes += len(s.routeCache.PoolToRouteIdxs[key])
	}
	// log.Info("strategy_balancer_linear full routes", "profitable", len(pop), "/", candidateRoutes, "t", utils.PrettyDuration(time.Now().Sub(ptx.StartTime)), "hash", ptx.Tx.Hash().Hex(), "gasPrice", ptx.Tx.GasPrice())
	if len(pop) < 2 {
		return
	}
	plan := s.getMostProfitablePath(pop, poolsInfoOverride, ptx.Tx.GasPrice())
	if plan == nil {
		return
	}
	s.routeCache.LastFiredTime[plan.RouteIdx] = time.Now()
	log.Info("strategy_balancer_linear final route", "strategy", s.Name, "profitable", len(pop), "/", candidateRoutes, "t", utils.PrettyDuration(time.Now().Sub(ptx.StartTime)), "hash", ptx.Tx.Hash().Hex(), "gasPrice", ptx.Tx.GasPrice())
	s.RailgunChan <- &RailgunPacket{
		Type:         SwapSinglePath,
		StrategyID:   s.ID,
		Target:       ptx.Tx,
		Response:     plan,
		ValidatorIDs: ptx.ValidatorIDs,
		StartTime:    ptx.StartTime,
	}
}

func (s *BalancerLinearStrategy) getProfitableRoutes(key PoolKey, poolsInfoOverride map[common.Address]*PoolInfoFloat) Population {
	var pop Population
	now := time.Now()
	routeIdxs, ok := s.routeCache.PoolToRouteIdxs[key]
	if !ok {
		return pop
	}
	h := RouteIdxHeap{s.routeCache.Scores, routeIdxs}
	heap.Init(&h)
	for routeIdx := heap.Pop(&h).(uint); h.Len() > 0; routeIdx = heap.Pop(&h).(uint) {
		if now.Sub(s.routeCache.LastFiredTime[routeIdx]) < 4*time.Second {
			continue
		}
		route := s.routeCache.Routes[routeIdx]
		amountIn := unitIn
		amountOut := s.getRouteAmountOutBalancer(route, amountIn, poolsInfoOverride, false)
		if amountOut < amountIn {
			break
		}
		pop = append(pop, Candidate{
			DiscreteGene:   routeIdx,
			ContinuousGene: amountIn,
			Fitness:        amountOut - amountIn,
		})
		for _, in := range inCandidates {
			amountIn := in + rand.NormFloat64()*in/4
			amountOut := s.getRouteAmountOutBalancer(route, amountIn, poolsInfoOverride, false)
			if amountOut < amountIn {
				break
			}
			pop = append(pop, Candidate{
				DiscreteGene:   routeIdx,
				ContinuousGene: amountIn,
				Fitness:        amountOut - amountIn,
			})
		}
	}
	return pop
}

func (s *BalancerLinearStrategy) getMostProfitablePath(pop Population, poolsInfoOverride map[common.Address]*PoolInfoFloat, gasPrice *big.Int) *Plan {
	start := time.Now()
	// log.Info("Initial population", "len", len(pop))
	// sort.Sort(pop)
	// if len(pop) < 10 {
	// 	pop.Print()
	// } else {
	// 	pop[:10].Print()
	// }
	// log.Info("Performing evolution on population", "size", len(pop))
	for i := 0; i < 5; i++ {
		pop = NextGeneration(pop, 1000, func(c Candidate) float64 {
			route := s.routeCache.Routes[c.DiscreteGene]
			amountOut := s.getRouteAmountOutBalancer(route, c.ContinuousGene, poolsInfoOverride, false)
			return amountOut - c.ContinuousGene
		})
		// sort.Sort(pop)
		// winner := pop[0]
		// winnerProfit := s.getRouteAmountOutBalancer(s.routeCache.Routes[winner.DiscreteGene], winner.ContinuousGene, poolsInfoOverride, false) - winner.ContinuousGene
		// log.Info("Found best candidate", "iteration", i, "route", winner.DiscreteGene, "amountIn", winner.ContinuousGene/1e18, "profit", winner.Fitness/1e18, "profit2", winnerProfit/1e18, "time", utils.PrettyDuration(time.Now().Sub(start)))
		// if i%3 == 0 {
		// 	pop[:10].Print()
		// }
	}
	sort.Sort(pop)
	winner := pop[0]
	route := s.routeCache.Routes[winner.DiscreteGene]
	// gas := estimateFishGasFloat(5, len(route), gasPrice)
	gas := estimateFishBalancerGas(route) * BigIntToFloat(gasPrice)
	// bestAmountOut := s.getRouteAmountOutBalancer(route, winner.ContinuousGene, poolsInfoOverride, false)
	// log.Info("Best route", "routeIdx", winner.DiscreteGene, "bestAmountIn", winner.ContinuousGene/1e18, "bestAmountOut", bestAmountOut/1e18, "bestGas", gas)
	netProfit := winner.Fitness - gas
	if netProfit < 0 {
		return nil
	}
	log.Info("Found best final candidate", "route", winner.DiscreteGene, "amountIn", winner.ContinuousGene/1e18, "profit", winner.Fitness/1e18, "time", utils.PrettyDuration(time.Now().Sub(start)))
	// s.getRouteAmountOutBalancer(route, winner.ContinuousGene, poolsInfoOverride, true)
	return s.makePlan(winner.DiscreteGene, gas, winner.ContinuousGene, netProfit, gasPrice, poolsInfoOverride)
}

func estimateFishBalancerGas(route []*Leg) float64 {
	numUniswapSwaps := 0
	numBalancerSwaps := 0
	for _, leg := range route {
		if leg.Type == UniswapV2Pair || leg.Type == SolidlyVolatilePool {
			numUniswapSwaps++
		} else {
			numBalancerSwaps++
		}
	}
	return float64(GAS_INITIAL + GAS_TRANSFER + numUniswapSwaps*GAS_SWAP + numBalancerSwaps*(GAS_ESTIMATE_BALANCER+GAS_SWAP_BALANCER+GAS_TRANSFER))
}

func (s *BalancerLinearStrategy) makePlan(routeIdx uint, gasCost, amountIn, netProfit float64, gasPrice *big.Int, poolsInfoOverride map[common.Address]*PoolInfoFloat) *Plan {
	route := s.routeCache.Routes[routeIdx]
	minProfit := convertFloat(wftm, route[0].From, gasCost, s.aggregatePools)
	startAmountIn := convertFloat(wftm, route[0].From, startTokensInContractFloat, s.aggregatePools)
	plan := &Plan{
		GasPrice:  gasPrice,
		GasCost:   FloatToBigInt(gasCost),
		NetProfit: FloatToBigInt(netProfit),
		MinProfit: FloatToBigInt(minProfit),
		AmountIn:  FloatToBigInt(startAmountIn),
		Path:      make([]fish5_lite.LinearSwapCommand, len(route)),
		RouteIdx:  routeIdx,
		Reserves:  make([]ReserveInfo, 0, len(route)*2),
	}
	for i, leg := range route {
		s.mu.RLock()
		poolInfo := s.poolsInfo[leg.PoolAddr] // No need to use override as we don't look up reserves
		predictedPoolInfo := getPoolInfoFloat(s.poolsInfo, poolsInfoOverride, leg.PoolAddr)
		s.mu.RUnlock()
		fromReserveInfo := ReserveInfo{
			Token: leg.From,
			Type:  leg.Type,
		}
		toReserveInfo := ReserveInfo{
			Token: leg.To,
			Type:  leg.Type,
		}
		if leg.Type == UniswapV2Pair || leg.Type == SolidlyVolatilePool {
			fromToken0 := bytes.Compare(poolInfo.Tokens[0].Bytes(), leg.From.Bytes()) == 0
			plan.Path[i] = fish5_lite.LinearSwapCommand{
				Token0:       poolInfo.Tokens[0],
				Token1:       poolInfo.Tokens[1],
				FeeNumerator: poolInfo.FeeNumeratorBI,
				FromToken0:   fromToken0,
				PoolType:     uint8(leg.Type),
			}
			copy(plan.Path[i].PoolId[:], leg.PoolAddr.Bytes())
			copy(fromReserveInfo.PoolId[:], leg.PoolAddr.Bytes())
			copy(toReserveInfo.PoolId[:], leg.PoolAddr.Bytes())
			fromReserveInfo.Original = FloatToBigInt(poolInfo.Reserves[leg.From])
			fromReserveInfo.Predicted = FloatToBigInt(predictedPoolInfo.Reserves[leg.From])
			toReserveInfo.Original = FloatToBigInt(poolInfo.Reserves[leg.To])
			toReserveInfo.Predicted = FloatToBigInt(predictedPoolInfo.Reserves[leg.To])
		} else {
			if leg.Type == BalancerStablePool {
				log.Info("Stable pool leg", "from", leg.From.Hex(), "to", leg.To.Hex(), "poolId", leg.PoolId)
			}
			plan.Path[i] = fish5_lite.LinearSwapCommand{
				Token0:     leg.From,
				Token1:     leg.To,
				FromToken0: true,
				PoolType:   uint8(leg.Type),
			}
			plan.Path[i].FeeNumerator = poolInfo.FeeBI
			plan.Path[i].PoolId = leg.PoolId
			fromReserveInfo.PoolId = leg.PoolId
			toReserveInfo.PoolId = leg.PoolId
			fromReserveInfo.Original = FloatToBigInt(downScale(poolInfo.Reserves[leg.From], poolInfo.ScaleFactors[leg.From]))
			fromReserveInfo.Predicted = FloatToBigInt(downScale(predictedPoolInfo.Reserves[leg.From], poolInfo.ScaleFactors[leg.From]))
			toReserveInfo.Original = FloatToBigInt(downScale(poolInfo.Reserves[leg.To], poolInfo.ScaleFactors[leg.To]))
			toReserveInfo.Predicted = FloatToBigInt(downScale(predictedPoolInfo.Reserves[leg.To], poolInfo.ScaleFactors[leg.To]))
		}
		plan.Reserves = append(plan.Reserves, fromReserveInfo, toReserveInfo)
	}
	return plan
}

// func (s *BalancerLinearStrategy) getRouteOptimalAmountIn(route []*Leg, poolsInfoOverride map[common.Address]*PoolInfo) *big.Int {
// 	s.mu.RLock()
// 	startPoolInfo := getPoolInfo(s.poolsInfo, poolsInfoOverride, route[0].PoolAddr)
// 	s.mu.RUnlock()
// 	leftAmount, rightAmount := new(big.Int), new(big.Int)
// 	if bytes.Compare(startPoolInfo.Token0.Bytes(), route[0].From.Bytes()) == 0 {
// 		leftAmount.Set(startPoolInfo.Reserves[0])
// 		rightAmount.Set(startPoolInfo.Reserves[1])
// 	} else {
// 		leftAmount.Set(startPoolInfo.Reserves[1])
// 		rightAmount.Set(startPoolInfo.Reserves[0])
// 	}
// 	r1 := startPoolInfo.FeeNumerator
// 	tenToSix := big.NewInt(int64(1e6))
// 	for _, leg := range route[1:] {
// 		s.mu.RLock()
// 		poolInfo := getPoolInfo(s.poolsInfo, poolsInfoOverride, leg.PoolAddr)
// 		s.mu.RUnlock()
// 		var reserveFrom, reserveTo *big.Int
// 		if bytes.Compare(poolInfo.Token0.Bytes(), leg.From.Bytes()) == 0 {
// 			reserveFrom, reserveTo = poolInfo.Reserves[0], poolInfo.Reserves[1]
// 		} else {
// 			reserveTo, reserveFrom = poolInfo.Reserves[0], poolInfo.Reserves[1]
// 		}
// 		legFee := poolInfo.FeeNumerator
// 		den := new(big.Int).Mul(rightAmount, legFee)
// 		den = den.Div(den, tenToSix)
// 		den = den.Add(den, reserveFrom)
// 		leftAmount = leftAmount.Mul(leftAmount, reserveFrom)
// 		leftAmount = leftAmount.Div(leftAmount, den)
// 		rightAmount = rightAmount.Mul(rightAmount, reserveTo)
// 		rightAmount = rightAmount.Mul(rightAmount, legFee)
// 		rightAmount = rightAmount.Div(rightAmount, tenToSix)
// 		rightAmount = rightAmount.Div(rightAmount, den)
// 	}
// 	amountIn := new(big.Int).Mul(rightAmount, leftAmount)
// 	amountIn = amountIn.Mul(amountIn, r1)
// 	amountIn = amountIn.Div(amountIn, tenToSix)
// 	amountIn = amountIn.Sqrt(amountIn)
// 	amountIn = amountIn.Sub(amountIn, leftAmount)
// 	amountIn = amountIn.Mul(amountIn, tenToSix)
// 	amountIn = amountIn.Div(amountIn, r1)
// 	if amountIn.Cmp(MaxAmountIn) == 1 { // amountIn > MaxAmountIn
// 		return MaxAmountIn
// 	}
// 	return amountIn
// }

// func (s *BalancerLinearStrategy) spotPriceAfterSwapExactTokensForTokens(
// 	amount *big.Int, poolPoolData ) *big.Int {
// 	s.mu.RLock()
// 	startPoolInfo := getPoolInfo(s.poolsInfo, poolsInfoOverride, route[0].PoolAddr)
// 	s.mu.RUnlock()
// 	leftAmount, rightAmount := new(big.Int), new(big.Int)
// 	if bytes.Compare(startPoolInfo.Token0.Bytes(), route[0].From.Bytes()) == 0 {
// 		leftAmount.Set(startPoolInfo.Reserves[0])
// 		rightAmount.Set(startPoolInfo.Reserves[1])
// 	} else {
// 		leftAmount.Set(startPoolInfo.Reserves[1])
// 		rightAmount.Set(startPoolInfo.Reserves[0])
// 	}
// 	r1 := startPoolInfo.FeeNumerator
// 	tenToSix := big.NewInt(int64(1e6))
// 	for _, leg := range route[1:] {
// 		s.mu.RLock()
// 		poolInfo := getPoolInfo(s.poolsInfo, poolsInfoOverride, leg.PoolAddr)
// 		s.mu.RUnlock()
// 		var reserveFrom, reserveTo *big.Int
// 		if bytes.Compare(poolInfo.Token0.Bytes(), leg.From.Bytes()) == 0 {
// 			reserveFrom, reserveTo = poolInfo.Reserves[0], poolInfo.Reserves[1]
// 		} else {
// 			reserveTo, reserveFrom = poolInfo.Reserves[0], poolInfo.Reserves[1]
// 		}
// 		legFee := poolInfo.FeeNumerator
// 		den := new(big.Int).Mul(rightAmount, legFee)
// 		den = den.Div(den, tenToSix)
// 		den = den.Add(den, reserveFrom)
// 		leftAmount = leftAmount.Mul(leftAmount, reserveFrom)
// 		leftAmount = leftAmount.Div(leftAmount, den)
// 		rightAmount = rightAmount.Mul(rightAmount, reserveTo)
// 		rightAmount = rightAmount.Mul(rightAmount, legFee)
// 		rightAmount = rightAmount.Div(rightAmount, tenToSix)
// 		rightAmount = rightAmount.Div(rightAmount, den)
// 	}
// 	amountIn := new(big.Int).Mul(rightAmount, leftAmount)
// 	amountIn = amountIn.Mul(amountIn, r1)
// 	amountIn = amountIn.Div(amountIn, tenToSix)
// 	amountIn = amountIn.Sqrt(amountIn)
// 	amountIn = amountIn.Sub(amountIn, leftAmount)
// 	amountIn = amountIn.Mul(amountIn, tenToSix)
// 	amountIn = amountIn.Div(amountIn, r1)
// 	if amountIn.Cmp(MaxAmountIn) == 1 { // amountIn > MaxAmountIn
// 		return MaxAmountIn
// 	}
// 	return amountIn
// }

// func (s *BalancerLinearStrategy) spotPriceAfterSwapExactTokenInForTokenOut(
// 	amount *big.Int, poolInfo *PoolInfo, fromToken, toToken common.Address) *big.Int {
// 	balanceIn := BigIntToFloat(poolInfo.Reserves[fromToken])
// 	balanceOut := BigIntToFloat(poolInfo.Reserves[toToken])
// 	weightIn := BigIntToFloat(poolInfo.Weights[fromToken])
// 	weightOut := BigIntToFloat(poolInfo.Weights[toToken])
// 	amountIn := BigIntToFloat(amount)
// 	fee := BigIntToFloat(poolInfo.Fee) / math.Pow(10, 18)
// 	spotPrice := -(balanceIn * weightOut) /
// 		(balanceOut * (-1 + fee) *
// 			math.Pow(balanceIn/(amountIn+balanceIn-amountIn*fee), (weightIn+weightOut)/weightOut) * weightIn)
// 	return FloatToBigInt(spotPrice * math.Pow(10, 18))
// }

// func (s *BalancerLinearStrategy) derivativeSpotPriceAfterSwapExactTokenInForTokenOut(
// 	amount *big.Int, poolInfo *PoolInfo, fromToken, toToken common.Address) *big.Int {
// 	balanceIn := BigIntToFloat(poolInfo.Reserves[fromToken]) / math.Pow(10, 18)
// 	balanceOut := BigIntToFloat(poolInfo.Reserves[toToken]) / math.Pow(10, 18)
// 	weightIn := BigIntToFloat(poolInfo.Weights[fromToken]) / math.Pow(10, 18)
// 	weightOut := BigIntToFloat(poolInfo.Weights[toToken]) / math.Pow(10, 18)
// 	amountIn := BigIntToFloat(amount)
// 	fee := BigIntToFloat(poolInfo.Fee) / math.Pow(10, 18)
// 	// fmt.Printf("balIn: %v, balOut: %v, weightIn: %v, weightOut: %v\n", balanceIn, balanceOut, weightIn, weightOut)
// 	// fmt.Printf("amountIn: %v, fee: %v\n", amountIn, fee)
// 	derivative := (weightIn + weightOut) /
// 		(balanceOut * math.Pow(balanceIn/(amountIn+balanceIn-amountIn*fee), weightIn/weightOut) * weightIn)
// 	return FloatToBigInt(derivative)
// }

// func (s *BalancerLinearStrategy) getBestPaths(
// 	paths, totalSwapAmount, inputDecimals, outputDecimals, maxPools) *big.Int {
// 	balanceIn := BigIntToFloat(poolInfo.Reserves[fromToken]) / math.Pow(10, 18)
// 	balanceOut := BigIntToFloat(poolInfo.Reserves[toToken]) / math.Pow(10, 18)
// 	weightIn := BigIntToFloat(poolInfo.Weights[fromToken]) / math.Pow(10, 18)
// 	weightOut := BigIntToFloat(poolInfo.Weights[toToken]) / math.Pow(10, 18)
// 	amountIn := BigIntToFloat(amount)
// 	fee := BigIntToFloat(poolInfo.Fee) / math.Pow(10, 18)
// 	// fmt.Printf("balIn: %v, balOut: %v, weightIn: %v, weightOut: %v\n", balanceIn, balanceOut, weightIn, weightOut)
// 	// fmt.Printf("amountIn: %v, fee: %v\n", amountIn, fee)
// 	derivative := (weightIn + weightOut) /
// 		(balanceOut * math.Pow(balanceIn/(amountIn+balanceIn-amountIn*fee), weightIn/weightOut) * weightIn)
// 	return FloatToBigInt(derivative)
// }
