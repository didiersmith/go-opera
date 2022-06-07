package dexter

import (
	"bytes"
	"container/heap"
	"encoding/json"
	"io/ioutil"
	"math"
	"math/big"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Fantom-foundation/go-opera/contracts/fish5_lite"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

var (
	wftm        = common.HexToAddress("0x21be370D5312f44cB42ce377BC9b8a0cEF1A4C83")
	poolCoolOff = 2 * time.Second
)

type LinearStrategy struct {
	Name                string
	ID                  int
	RailgunChan         chan *RailgunPacket
	inPossibleTxsChan   chan *PossibleTx
	inPermUpdatesChan   chan []*PoolUpdate
	cfg                 LinearStrategyConfig
	poolsInfo           map[common.Address]*PoolInfoFloat
	poolsInfoUpdateChan chan *PoolsInfoUpdateFloat
	interestedPairs     map[common.Address]PoolType
	edgePools           map[EdgeKey][]common.Address
	aggregatePools      map[EdgeKey]*PoolInfoFloat
	routeCache          RouteCache
	subStrategies       []Strategy
	gasPrice            int64
	mu                  sync.RWMutex
}

type LinearStrategyConfig struct {
	RoutesFileName          string
	PoolToRouteIdxsFileName string
	SelectSecondBest        bool
}

func NewLinearStrategy(name string, id int, railgun chan *RailgunPacket, cfg LinearStrategyConfig) Strategy {
	s := &LinearStrategy{
		Name:                name,
		ID:                  id,
		RailgunChan:         railgun,
		inPossibleTxsChan:   make(chan *PossibleTx, 256),
		inPermUpdatesChan:   make(chan []*PoolUpdate, 256),
		cfg:                 cfg,
		poolsInfo:           make(map[common.Address]*PoolInfoFloat),
		poolsInfoUpdateChan: make(chan *PoolsInfoUpdateFloat, 32),
		interestedPairs:     make(map[common.Address]PoolType),
		aggregatePools:      make(map[EdgeKey]*PoolInfoFloat),
	}
	s.loadJson()
	return s
}

func (s *LinearStrategy) SetPoolsInfo(poolsInfo map[common.Address]*PoolInfo) {
	for k, v := range poolsInfo {
		reserves := make(map[common.Address]float64)
		for a, r := range v.Reserves {
			reserves[a] = BigIntToFloat(r)
		}
		poolInfo := &PoolInfoFloat{
			Tokens:         v.Tokens,
			Reserves:       reserves,
			FeeNumerator:   BigIntToFloat(v.FeeNumerator),
			FeeNumeratorBI: v.FeeNumerator,
			LastUpdate:     time.Now(),
			Type:           UniswapV2Pair,
		}
		s.poolsInfo[k] = poolInfo
	}
}

func (s *LinearStrategy) SetEdgePools(edgePools map[EdgeKey][]common.Address) {
	s.edgePools = edgePools
}

func (s *LinearStrategy) SetGasPrice(gasPrice int64) {
	s.gasPrice = gasPrice
}

func (s *LinearStrategy) ProcessPossibleTx(t *PossibleTx) {
	s.inPossibleTxsChan <- t
}

func (s *LinearStrategy) ProcessPermUpdates(us []*PoolUpdate) {
	s.inPermUpdatesChan <- us
}

func (s *LinearStrategy) GetInterestedPools() (map[common.Address]PoolType, map[BalPoolId]PoolType) {
	return s.interestedPairs, nil
}

func (s *LinearStrategy) AddSubStrategy(sub Strategy) {
	s.subStrategies = append(s.subStrategies, sub)
}

func (s *LinearStrategy) Start() {
	s.aggregatePools = makeAggregatePoolsFloat(s.edgePools, s.poolsInfo, nil)
	s.routeCache.Scores = s.makeScores()
	log.Info("Scores", "head", s.routeCache.Scores[:10], "tail", s.routeCache.Scores[len(s.routeCache.Scores)-10:])
	fromToken := common.HexToAddress("0x04068da6c83afcfa0e13ba15a6696662335d5b75")
	log.Info("1 usdc -> wftm", "amount", convertFloat(fromToken, wftm, 1e6, s.aggregatePools))
	go s.runPermUpdater()
	go s.runStrategy()
}

func (s *LinearStrategy) loadJson() {
	log.Info("Loading routes")
	routeCacheRoutesFile, err := os.Open(s.cfg.RoutesFileName)
	if err != nil {
		log.Info("Error opening routeCacheRoutes", "routeCacheRoutesFileName", s.cfg.RoutesFileName, "err", err)
		return
	}
	defer routeCacheRoutesFile.Close()
	routeCacheRoutesBytes, _ := ioutil.ReadAll(routeCacheRoutesFile)
	var routeCacheJson LegacyRouteCacheJson
	json.Unmarshal(routeCacheRoutesBytes, &(routeCacheJson.Routes))
	log.Info("Loaded routes", "len", len(routeCacheJson.Routes))

	routeCachePoolToRouteIdxsFile, err := os.Open(s.cfg.PoolToRouteIdxsFileName)
	if err != nil {
		log.Info("Error opening routeCachePoolToRouteIdxs", "routeCachePoolToRouteIdxsFileName", s.cfg.PoolToRouteIdxsFileName, "err", err)
		return
	}
	defer routeCachePoolToRouteIdxsFile.Close()
	routeCachePoolToRouteIdxsBytes, _ := ioutil.ReadAll(routeCachePoolToRouteIdxsFile)
	json.Unmarshal(routeCachePoolToRouteIdxsBytes, &(routeCacheJson.PoolToRouteIdxs))
	log.Info("Loaded poolToRouteIdxs", "len", len(routeCacheJson.PoolToRouteIdxs))

	routeCache := RouteCache{
		Routes:          make([][]*Leg, len(routeCacheJson.Routes)),
		PoolToRouteIdxs: make(map[PoolKey][]uint),
	}
	for i, routeJson := range routeCacheJson.Routes {
		route := make([]*Leg, len(routeJson))
		// log.Info("Route", "routeJson", routeJson)
		for x, leg := range routeJson {
			poolAddr := common.HexToAddress(leg.PairAddr)
			t := UniswapV2Pair
			if leg.ExchangeName == "Solidly" {
				t = SolidlyVolatilePool
			}
			s.interestedPairs[poolAddr] = t
			route[x] = &Leg{
				From:     common.HexToAddress(leg.From),
				To:       common.HexToAddress(leg.To),
				PoolAddr: poolAddr,
				Type:     t,
			}
		}
		routeCache.Routes[i] = route
	}
	for strKey, routeIdxs := range routeCacheJson.PoolToRouteIdxs {
		parts := strings.Split(strKey, "_")
		key := poolKeyFromStrs(parts[0], parts[1], parts[2])
		routeCache.PoolToRouteIdxs[key] = routeIdxs
	}
	log.Info("Processed route cache", "name", s.Name, "len(routes)", len(routeCache.Routes), "len(PoolToRouteIdxs)", len(routeCache.PoolToRouteIdxs))
	s.routeCache = routeCache
}

func (s *LinearStrategy) runPermUpdater() {
	for {
		aggregatePoolUpdates := make(map[EdgeKey]*PoolInfoFloat)
		poolsInfoUpdates := make(map[common.Address]*PoolInfoFloat)
		refreshKeys := make(map[PoolKey]struct{})
		us := <-s.inPermUpdatesChan
		// start := time.Now()
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
				reserves[a] = BigIntToFloat(r)
			}
			poolsInfoUpdates[u.Addr] = &PoolInfoFloat{
				Reserves:       reserves,
				Tokens:         poolInfo.Tokens,
				FeeNumerator:   poolInfo.FeeNumerator,
				FeeNumeratorBI: poolInfo.FeeNumeratorBI,
				LastUpdate:     time.Now(),
			}
			key0 := poolKeyFromAddrs(poolInfo.Tokens[0], poolInfo.Tokens[1], u.Addr)
			key1 := poolKeyFromAddrs(poolInfo.Tokens[1], poolInfo.Tokens[0], u.Addr)
			refreshKeys[key0] = struct{}{}
			refreshKeys[key1] = struct{}{}
			aggKey := MakeEdgeKey(poolInfo.Tokens[0], poolInfo.Tokens[1])
			s.mu.RLock()
			aggregatePoolUpdates[aggKey] = refreshAggregatePoolFloat(aggKey, s.edgePools[aggKey], s.poolsInfo, poolsInfoUpdates)
			s.mu.RUnlock()
		}
		if len(poolsInfoUpdates) > 0 {
			update := &PoolsInfoUpdateFloat{
				PoolsInfoUpdates: poolsInfoUpdates,
			}
			// log.Info("Linear Perm updater done computing updates", "t", utils.PrettyDuration(time.Now().Sub(start)), "queue", len(s.inPermUpdatesChan), "name", s.Name)
			s.poolsInfoUpdateChan <- update
			s.refreshScoresForPools(refreshKeys, poolsInfoUpdates)
		}
	}
}

func (s *LinearStrategy) runStrategy() {
	for {
		select {
		case update := <-s.poolsInfoUpdateChan:
			s.mu.Lock()
			// s.routeCache.PoolToRouteIdxs = update.PoolToRouteIdxs
			for poolAddr, poolInfo := range update.PoolsInfoUpdates {
				s.poolsInfo[poolAddr] = poolInfo
				// if s.ID == 0 {
				// 	log.Info("Received update", "poolAddr", poolAddr, "reserves", poolInfo.Reserves, "feeNumerator", poolInfo.FeeNumerator)
				// }
			}
			for edgeKey, poolInfo := range update.AggregatePools {
				s.aggregatePools[edgeKey] = poolInfo
			}
			s.mu.Unlock()
			// s.triggerOnBlockState(update.PoolsInfoUpdates)
		case p := <-s.inPossibleTxsChan:
			s.processPotentialTx(p)
		}
	}
}

// func (s *LinearStrategy) sortPoolToRouteIdxMap(scores []uint64) map[PoolKey][]uint {
// 	poolToRouteIdxs := make(map[PoolKey][]uint, len(s.routeCache.PoolToRouteIdxs))
// 	s.mu.RLock()
// 	for poolKey, routeIdxs := range s.routeCache.PoolToRouteIdxs {
// 		newRouteIdxs := make([]uint, len(routeIdxs))
// 		copy(newRouteIdxs, routeIdxs)
// 		poolToRouteIdxs[poolKey] = newRouteIdxs
// 	}
// 	s.mu.RUnlock()
// 	for _, routeIdxs := range poolToRouteIdxs {
// 		sort.Slice(routeIdxs, func(a, b int) bool {
// 			return scores[routeIdxs[a]] > scores[routeIdxs[b]]
// 		})
// 	}
// 	return poolToRouteIdxs
// }

func (s *LinearStrategy) getScore(route []*Leg, poolsInfoOverride map[common.Address]*PoolInfoFloat) float64 {
	// score := 1.0
	// for _, leg := range route {
	// 	s.mu.RLock()
	// 	poolInfo := getPoolInfoFloat(s.poolsInfo, poolsInfoOverride, leg.PoolAddr)
	// 	s.mu.RUnlock()
	// 	reserveFrom, reserveTo := poolInfo.Reserves[leg.From], poolInfo.Reserves[leg.To]
	// 	score = score * (reserveTo * poolInfo.FeeNumerator) / (reserveFrom * 1e6)
	// }
	// return score
	amountIn := convertFloat(wftm, route[0].From, startInFloat, s.aggregatePools)
	amountOut := s.getRouteAmountOut(route, amountIn, poolsInfoOverride, false)
	amountOut = convertFloat(route[0].From, wftm, amountOut, s.aggregatePools)
	return amountOut
	// amountOut.Div(amountOut, big.NewInt(1e6))
	// return BigIntToFloat(amountOut)
}

func (s *LinearStrategy) makeScores() []float64 {
	scores := make([]float64, len(s.routeCache.Routes))
	for i, route := range s.routeCache.Routes {
		scores[i] = s.getScore(route, nil)
	}
	return scores
}

func (s *LinearStrategy) refreshScoresForPools(
	keys map[PoolKey]struct{}, poolsInfoOverride map[common.Address]*PoolInfoFloat) {
	var allRouteIdxs []uint
	for key, _ := range keys {
		if routeIdxs, ok := s.routeCache.PoolToRouteIdxs[key]; ok {
			allRouteIdxs = append(allRouteIdxs, routeIdxs...)
		}
	}
	sort.Slice(allRouteIdxs, func(a, b int) bool { return a < b })
	allRouteIdxs = uniq(allRouteIdxs)
	for _, routeIdx := range allRouteIdxs {
		route := s.routeCache.Routes[routeIdx]
		s.routeCache.Scores[routeIdx] = s.getScore(route, poolsInfoOverride)
	}
}

func (s *LinearStrategy) getRouteAmountOut(route []*Leg, amountIn float64, poolsInfoOverride map[common.Address]*PoolInfoFloat, debug bool) float64 {
	var amountOut float64
	if debug {
		log.Info("getRouteAmountOut", "amountIn", amountIn, "route", route)
	}
	for _, leg := range route {
		s.mu.RLock()
		poolInfo := getPoolInfoFloat(s.poolsInfo, poolsInfoOverride, leg.PoolAddr)
		s.mu.RUnlock()
		reserveFrom, reserveTo := poolInfo.Reserves[leg.From], poolInfo.Reserves[leg.To]
		amountOut = getAmountOutUniswapFloat(amountIn, reserveFrom, reserveTo, poolInfo.FeeNumerator)
		if debug {
			log.Info("Leg", "amountIn", amountIn, "amountOut", amountOut, "reserveFrom", reserveFrom, "reserveTo", reserveTo, "feeNumerator", poolInfo.FeeNumerator)
		}
		amountIn = amountOut
	}
	return amountOut
}

func (s *LinearStrategy) processPotentialTx(ptx *PossibleTx) {
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
			reserves[a] = BigIntToFloat(r)
		}
		tokens := []common.Address{poolInfo.Tokens[0], poolInfo.Tokens[1]}
		poolsInfoOverride[u.Addr] = &PoolInfoFloat{
			Reserves:       reserves,
			Tokens:         tokens,
			FeeNumerator:   poolInfo.FeeNumerator,
			FeeNumeratorBI: poolInfo.FeeNumeratorBI,
		}
		updatedKeys = append(updatedKeys,
			poolKeyFromAddrs(poolInfo.Tokens[0], poolInfo.Tokens[1], u.Addr),
			poolKeyFromAddrs(poolInfo.Tokens[1], poolInfo.Tokens[0], u.Addr))
	}
	// log.Info("Finished processing tx", "hash", tx.Hash().Hex(), "t", utils.PrettyDuration(time.Now().Sub(start)), "updatedKeys", len(updatedKeys))
	// for poolAddr, poolInfo := range poolsInfoOverride {
	// 	log.Info("dexter.PoolInfo override", "poolAddr", poolAddr, "reserve0", poolInfo.reserve0, "reserve1", poolInfo.reserve1, "info", *poolInfo)
	// }
	var allProfitableRoutes []uint
	candidateRoutes := 0
	for _, key := range updatedKeys {
		profitableRoutes := s.getProfitableRoutes(key, poolsInfoOverride)
		allProfitableRoutes = append(allProfitableRoutes, profitableRoutes...)
		candidateRoutes += len(s.routeCache.PoolToRouteIdxs[key])
	}
	sort.Slice(allProfitableRoutes, func(a, b int) bool { return allProfitableRoutes[a] < allProfitableRoutes[b] })
	allProfitableRoutes = uniq(allProfitableRoutes)
	if len(ptx.AvoidPoolAddrs) > 0 {
		allProfitableRoutes = s.filterRoutesAvoidPools(allProfitableRoutes, ptx.AvoidPoolAddrs)
	}
	// log.Info("Computed route", "strategy", s.Name, "profitable", len(allProfitableRoutes), "/", candidateRoutes, "t", utils.PrettyDuration(time.Now().Sub(start)), "hash", ptx.Tx.Hash().Hex(), "gasPrice", ptx.Tx.GasPrice(), "size", ptx.Tx.Size(), "avoiding", len(ptx.AvoidPoolAddrs))
	if len(allProfitableRoutes) == 0 {
		return
	}
	plan := s.getMostProfitablePath(allProfitableRoutes, poolsInfoOverride, ptx.Tx.GasPrice())
	if plan == nil {
		return
	}
	// for i, leg := range plan.Path {
	// 	poolInfo := getPoolInfo(s.poolsInfo, poolsInfoOverride, leg.Pair)
	// 	origPoolInfo := s.poolsInfo[leg.Pair]
	// 	log.Info("Leg", "i", i, "pair", leg.Pair, "reserves", poolInfo.Reserves, "origReserves", origPoolInfo.Reserves)
	// }
	s.RailgunChan <- &RailgunPacket{
		Type:         SwapSinglePath,
		StrategyID:   s.ID,
		Target:       ptx.Tx,
		Response:     plan,
		ValidatorIDs: ptx.ValidatorIDs,
		StartTime:    ptx.StartTime,
	}
}

// func (s *LinearStrategy) triggerOnBlockState(updates map[common.Address]*PoolInfo) {
// 	start := time.Now()
// 	var updatedKeys []PoolKey
// 	for poolAddr, _ := range updates {
// 		s.mu.RLock()
// 		poolInfo := s.poolsInfo[poolAddr]
// 		s.mu.RUnlock()
// 		updatedKeys = append(updatedKeys,
// 			poolKeyFromAddrs(poolInfo.Tokens[0], poolInfo.Tokens[1], poolAddr),
// 			poolKeyFromAddrs(poolInfo.Tokens[1], poolInfo.Tokens[0], poolAddr))
// 	}
// 	var allProfitableRoutes []uint
// 	candidateRoutes := 0
// 	for _, key := range updatedKeys {
// 		profitableRoutes := s.getProfitableRoutes(key, nil)
// 		allProfitableRoutes = append(allProfitableRoutes, profitableRoutes...)
// 		candidateRoutes += len(s.routeCache.PoolToRouteIdxs[key])
// 	}
// 	sort.Slice(allProfitableRoutes, func(a, b int) bool { return allProfitableRoutes[a] < allProfitableRoutes[b] })
// 	allProfitableRoutes = uniq(allProfitableRoutes)
// 	if len(allProfitableRoutes) == 0 {
// 		return
// 	}
// 	plan := s.getMostProfitablePath(allProfitableRoutes, nil, big.NewInt(s.gasPrice))
// 	if plan == nil {
// 		return
// 	}
// 	s.RailgunChan <- &RailgunPacket{
// 		Type:         SwapSinglePath,
// 		StrategyID:   s.ID,
// 		Target:       nil,
// 		Response:     plan,
// 		ValidatorIDs: nil,
// 	}
// }

func (s *LinearStrategy) getProfitableRoutes(key PoolKey, poolsInfoOverride map[common.Address]*PoolInfoFloat) []uint {
	var ret []uint
	if routeIdxs, ok := s.routeCache.PoolToRouteIdxs[key]; ok {
		h := RouteIdxHeap{s.routeCache.Scores, routeIdxs}
		heap.Init(&h)
		// sort.Slice(routeIdxs, func(a, b int) bool {
		// 	return s.routeCache.Scores[routeIdxs[a]] > s.routeCache.Scores[routeIdxs[b]]
		// })
		// for _, routeIdx := range routeIdxs {
		for routeIdx := heap.Pop(&h).(uint); h.Len() > 0; routeIdx = heap.Pop(&h).(uint) {
			route := s.routeCache.Routes[routeIdx]
			amountIn := convertFloat(wftm, route[0].From, startInFloat, s.aggregatePools)
			amountOut := s.getRouteAmountOut(route, amountIn, poolsInfoOverride, false)
			// if s.ID > 0 {
			// 	log.Info("getProfitableRoutes", "Score", h.Scores[routeIdx], "amountIn", amountIn, "amountOut", amountOut)
			// }
			if amountOut < amountIn {
				break
			}
			ret = append(ret, routeIdx)
		}
	}
	return ret
}

func (s *LinearStrategy) filterRoutesAvoidPools(routeIdxs []uint, avoidPools []*common.Address) []uint {
	var filteredIdxs []uint
	for _, routeIdx := range routeIdxs {
		route := s.routeCache.Routes[routeIdx]
		found := false
		for _, leg := range route {
			if found {
				break
			}
			for _, poolAddr := range avoidPools {
				if bytes.Compare(poolAddr.Bytes(), leg.PoolAddr.Bytes()) == 0 {
					found = true
					break
				}
			}
		}
		if !found {
			filteredIdxs = append(filteredIdxs, routeIdx)
		}
	}
	return filteredIdxs
}

// TODO: Update this to make a sorted list of candidates and select the second best if cfg.SelectSecondBest
func (s *LinearStrategy) getMostProfitablePath(routeIdxs []uint, poolsInfoOverride map[common.Address]*PoolInfoFloat, gasPrice *big.Int) *Plan {
	var maxProfit, bestAmountIn, bestGas float64
	var bestRouteIdx uint
	for _, routeIdx := range routeIdxs {
		route := s.routeCache.Routes[routeIdx]
		amountIn := s.getRouteOptimalAmountIn(route, poolsInfoOverride)
		if amountIn < 0 {
			log.Info("WARNING: Negative amountIn for route", "routeIdx", routeIdx, "amountIn", amountIn)
			continue
		}
		amountOut := s.getRouteAmountOut(route, amountIn, poolsInfoOverride, false)
		if amountOut < amountIn {
			continue
		}
		profit := amountOut - amountIn
		// profit := new(big.Int).Sub(amountOut, amountIn)
		profit = convertFloat(route[0].From, wftm, profit, s.aggregatePools) // -> wftm
		gas := estimateFishGasFloat(1, len(route), gasPrice)
		netProfit := profit - gas
		// netProfitSubFailures := new(big.Int).Sub(netProfit, estimateFailureCost(gasPrice))
		if netProfit < maxProfit {
			continue
		}
		// if netProfitSubFailures.Cmp(maxProfit) == -1 {
		// 	continue
		// }
		// log.Info("New most profitable route", "amountIn", amountIn, "amountOut", amountOut, "profit", profit, "netProfit", netProfit, "gas", gas, "maxProfit", maxProfit, "routeIdx", routeIdx)
		maxProfit = netProfit
		bestAmountIn = amountIn
		bestGas = gas
		bestRouteIdx = routeIdx
	}
	if maxProfit == 0 {
		return nil
	}
	// log.Info("Best route", "strategy", s.Name, "routeIdx", bestRouteIdx, "bestAmountIn", bestAmountIn, "bestAmountOut", bestAmountOut, "bestGas", bestGas, "maxProfit", maxProfit)
	return s.makePlan(bestRouteIdx, bestGas, bestAmountIn, maxProfit, gasPrice)
}

func (s *LinearStrategy) makePlan(routeIdx uint, gasCost, amountIn, netProfit float64, gasPrice *big.Int) *Plan {
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
	}
	for i, leg := range route {
		s.mu.RLock()
		poolInfo := s.poolsInfo[leg.PoolAddr] // No need to use override as we don't look up reserves
		s.mu.RUnlock()
		fromToken0 := bytes.Compare(poolInfo.Tokens[0].Bytes(), leg.From.Bytes()) == 0
		plan.Path[i] = fish5_lite.LinearSwapCommand{
			Token0:       poolInfo.Tokens[0],
			Token1:       poolInfo.Tokens[1],
			FeeNumerator: poolInfo.FeeNumeratorBI,
			FromToken0:   fromToken0,
			PoolType:     uint8(leg.Type),
		}
		copy(plan.Path[i].PoolId[:], leg.PoolAddr.Bytes())
	}
	return plan
}

func (s *LinearStrategy) getRouteOptimalAmountIn(route []*Leg, poolsInfoOverride map[common.Address]*PoolInfoFloat) float64 {
	s.mu.RLock()
	startPoolInfo := getPoolInfoFloat(s.poolsInfo, poolsInfoOverride, route[0].PoolAddr)
	s.mu.RUnlock()
	leftAmount, rightAmount := 0.0, 0.0
	leftAmount = startPoolInfo.Reserves[route[0].From]
	rightAmount = startPoolInfo.Reserves[route[0].To]
	r1 := startPoolInfo.FeeNumerator
	for _, leg := range route[1:] {
		s.mu.RLock()
		poolInfo := getPoolInfoFloat(s.poolsInfo, poolsInfoOverride, leg.PoolAddr)
		s.mu.RUnlock()
		reserveFrom, reserveTo := poolInfo.Reserves[leg.From], poolInfo.Reserves[leg.To]
		legFee := poolInfo.FeeNumerator
		den := legFee*rightAmount/1e6 + reserveFrom
		// den := new(big.Int).Mul(rightAmount, legFee)
		// den = den.Div(den, tenToSix)
		// den = den.Add(den, reserveFrom)
		leftAmount = leftAmount * reserveFrom / den
		// leftAmount = leftAmount.Mul(leftAmount, reserveFrom)
		// leftAmount = leftAmount.Div(leftAmount, den)
		rightAmount = rightAmount * reserveTo * legFee / (den * 1e6)
		// rightAmount = rightAmount.Mul(rightAmount, reserveTo)
		// rightAmount = rightAmount.Mul(rightAmount, legFee)
		// rightAmount = rightAmount.Div(rightAmount, tenToSix)
		// rightAmount = rightAmount.Div(rightAmount, den)
	}
	amountIn := math.Sqrt(rightAmount*leftAmount*r1/1e6) - leftAmount
	amountIn *= 1e6 / r1
	// amountIn := new(big.Int).Mul(rightAmount, leftAmount)
	// amountIn = amountIn.Mul(amountIn, r1)
	// amountIn = amountIn.Div(amountIn, tenToSix)
	// amountIn = amountIn.Sqrt(amountIn)
	// amountIn = amountIn.Sub(amountIn, leftAmount)
	// amountIn = amountIn.Mul(amountIn, tenToSix)
	// amountIn = amountIn.Div(amountIn, r1)
	return amountIn
}

func (s *LinearStrategy) isHotPath(route []*Leg, now time.Time) bool {
	for _, leg := range route {
		s.mu.RLock()
		poolInfo := s.poolsInfo[leg.PoolAddr]
		s.mu.RUnlock()
		if now.Sub(poolInfo.LastUpdate) < poolCoolOff {
			log.Info("Route contains hot pair, skipping", "addr", leg.PoolAddr, "LastUpdate", poolInfo.LastUpdate, "now", now)
			return true
		}
	}
	return false
}
