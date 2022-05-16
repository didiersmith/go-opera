package dexter

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"math/big"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Fantom-foundation/go-opera/contracts/fish4_lite"
	"github.com/Fantom-foundation/go-opera/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

var (
	wftm = common.HexToAddress("0x21be370D5312f44cB42ce377BC9b8a0cEF1A4C83")
)

type LinearStrategy struct {
	Name                string
	ID                  int
	RailgunChan         chan *RailgunPacket
	inPossibleTxsChan   chan *PossibleTx
	inPermUpdatesChan   chan []*PoolUpdate
	cfg                 LinearStrategyConfig
	poolsInfo           map[common.Address]*PoolInfo
	poolsInfoUpdateChan chan *PoolsInfoUpdate
	interestedPools     map[common.Address]struct{}
	edgePools           map[EdgeKey][]common.Address
	aggregatePools      map[EdgeKey]*PoolInfo
	routeCache          RouteCache
	subStrategies       []Strategy
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
		poolsInfo:           make(map[common.Address]*PoolInfo),
		poolsInfoUpdateChan: make(chan *PoolsInfoUpdate),
		interestedPools:     make(map[common.Address]struct{}),
		aggregatePools:      make(map[EdgeKey]*PoolInfo),
	}
	s.loadJson()
	return s
}

func (s *LinearStrategy) SetPoolsInfo(poolsInfo map[common.Address]*PoolInfo) {
	for k, v := range poolsInfo {
		s.poolsInfo[k] = v
	}
}

func (s *LinearStrategy) SetEdgePools(edgePools map[EdgeKey][]common.Address) {
	s.edgePools = edgePools
}

func (s *LinearStrategy) ProcessPossibleTx(t *PossibleTx) {
	s.inPossibleTxsChan <- t
}

func (s *LinearStrategy) ProcessPermUpdates(us []*PoolUpdate) {
	s.inPermUpdatesChan <- us
}

func (s *LinearStrategy) GetInterestedPools() map[common.Address]struct{} {
	return s.interestedPools
}

func (s *LinearStrategy) AddSubStrategy(sub Strategy) {
	s.subStrategies = append(s.subStrategies, sub)
}

func (s *LinearStrategy) Start() {
	s.aggregatePools = makeAggregatePools(s.edgePools, s.poolsInfo, nil)
	scores := s.makeScores()
	s.routeCache.PoolToRouteIdxs = s.sortPoolToRouteIdxMap(scores)
	fromToken := common.HexToAddress("0x04068da6c83afcfa0e13ba15a6696662335d5b75")
	log.Info("1 usdc -> wftm", "amount", convert(fromToken, wftm, big.NewInt(1000000), s.aggregatePools))
	go s.runPermUpdater(scores)
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
	}
	for i, routeJson := range routeCacheJson.Routes {
		route := make([]*Leg, len(routeJson))
		// log.Info("Route", "routeJson", routeJson)
		for x, leg := range routeJson {
			poolAddr := common.HexToAddress(leg.PairAddr)
			s.interestedPools[poolAddr] = struct{}{}
			route[x] = &Leg{
				From:         common.HexToAddress(leg.From),
				To:           common.HexToAddress(leg.To),
				PoolAddr:     poolAddr,
				ExchangeType: leg.ExchangeType,
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

func (s *LinearStrategy) runPermUpdater(scores []uint64) {
	for {
		us := <-s.inPermUpdatesChan
		poolsInfoUpdates := make(map[common.Address]*PoolInfo)
		aggregatePoolUpdates := make(map[EdgeKey]*PoolInfo)
		for _, u := range us {
			s.mu.RLock()
			poolInfo, ok := s.poolsInfo[u.Addr]
			s.mu.RUnlock()
			if !ok {
				continue
			}
			tokens := []common.Address{poolInfo.Tokens[0], poolInfo.Tokens[1]}
			poolsInfoUpdates[u.Addr] = &PoolInfo{
				Reserves:     u.Reserves,
				Tokens:       tokens,
				FeeNumerator: poolInfo.FeeNumerator,
			}
			key0 := poolKeyFromAddrs(poolInfo.Tokens[0], poolInfo.Tokens[1], u.Addr)
			key1 := poolKeyFromAddrs(poolInfo.Tokens[1], poolInfo.Tokens[0], u.Addr)
			s.refreshScoresForPool(key0, poolsInfoUpdates, scores) // Updates scores
			s.refreshScoresForPool(key1, poolsInfoUpdates, scores)
			aggKey := MakeEdgeKey(poolInfo.Tokens[0], poolInfo.Tokens[1])
			s.mu.RLock()
			aggregatePoolUpdates[aggKey] = refreshAggregatePool(aggKey, s.edgePools[aggKey], s.poolsInfo, poolsInfoUpdates)
			s.mu.RUnlock()

		}
		if len(poolsInfoUpdates) > 0 {
			poolToRouteIdxs := s.sortPoolToRouteIdxMap(scores)
			update := &PoolsInfoUpdate{
				PoolsInfoUpdates: poolsInfoUpdates,
				PoolToRouteIdxs:  poolToRouteIdxs,
			}
			s.poolsInfoUpdateChan <- update
		}
	}
}

func (s *LinearStrategy) runStrategy() {
	for {
		select {
		case update := <-s.poolsInfoUpdateChan:
			// log.Info("Received update", "pools", len(update.PoolsInfoUpdates), "PoolToRouteIdxs", len(update.PoolToRouteIdxs))
			s.mu.Lock()
			s.routeCache.PoolToRouteIdxs = update.PoolToRouteIdxs
			for poolAddr, poolInfo := range update.PoolsInfoUpdates {
				s.poolsInfo[poolAddr] = poolInfo
			}
			for edgeKey, poolInfo := range update.AggregatePools {
				s.aggregatePools[edgeKey] = poolInfo
			}
			s.mu.Unlock()
		case p := <-s.inPossibleTxsChan:
			s.processPotentialTx(p)
		}
	}
}

func (s *LinearStrategy) sortPoolToRouteIdxMap(scores []uint64) map[PoolKey][]uint {
	poolToRouteIdxs := make(map[PoolKey][]uint, len(s.routeCache.PoolToRouteIdxs))
	s.mu.RLock()
	for poolKey, routeIdxs := range s.routeCache.PoolToRouteIdxs {
		newRouteIdxs := make([]uint, len(routeIdxs))
		copy(newRouteIdxs, routeIdxs)
		poolToRouteIdxs[poolKey] = newRouteIdxs
	}
	s.mu.RUnlock()
	for _, routeIdxs := range poolToRouteIdxs {
		sort.Slice(routeIdxs, func(a, b int) bool {
			return scores[routeIdxs[a]] > scores[routeIdxs[b]]
		})
	}
	return poolToRouteIdxs
}

func (s *LinearStrategy) getScore(route []*Leg, poolsInfoOverride map[common.Address]*PoolInfo) uint64 {
	amountIn := convert(wftm, route[0].From, startTokensIn, s.aggregatePools)
	amountOut := s.getRouteAmountOut(route, amountIn, poolsInfoOverride)
	amountOut = convert(route[0].From, wftm, amountOut, s.aggregatePools)
	amountOut.Div(amountOut, big.NewInt(1e6))
	return uint64(amountOut.Int64())
}

func (s *LinearStrategy) makeScores() []uint64 {
	scores := make([]uint64, len(s.routeCache.Routes))
	for i, route := range s.routeCache.Routes {
		scores[i] = s.getScore(route, nil)
	}
	return scores
}

func (s *LinearStrategy) refreshScoresForPool(key PoolKey, poolsInfoOverride map[common.Address]*PoolInfo, scores []uint64) {
	if routeIdxs, ok := s.routeCache.PoolToRouteIdxs[key]; ok {
		for _, routeIdx := range routeIdxs {
			route := s.routeCache.Routes[routeIdx]
			scores[routeIdx] = s.getScore(route, poolsInfoOverride)
		}
	}
}

func (s *LinearStrategy) getRouteAmountOut(route []*Leg, amountIn *big.Int, poolsInfoOverride map[common.Address]*PoolInfo) *big.Int {
	var amountOut *big.Int
	for _, leg := range route {
		s.mu.RLock()
		poolInfo := getPoolInfo(s.poolsInfo, poolsInfoOverride, leg.PoolAddr)
		s.mu.RUnlock()
		var reserveFrom *big.Int
		var reserveTo *big.Int
		if bytes.Compare(poolInfo.Tokens[0].Bytes(), leg.From.Bytes()) == 0 {
			reserveFrom, reserveTo = poolInfo.Reserves[0], poolInfo.Reserves[1]
		} else {
			reserveFrom, reserveTo = poolInfo.Reserves[1], poolInfo.Reserves[0]
		}
		amountOut = getAmountOutUniswap(amountIn, reserveFrom, reserveTo, poolInfo.FeeNumerator)
		amountIn = amountOut
	}
	return amountOut
}

func (s *LinearStrategy) processPotentialTx(ptx *PossibleTx) {
	start := time.Now()
	poolsInfoOverride := make(map[common.Address]*PoolInfo)
	var updatedKeys []PoolKey
	for _, u := range ptx.Updates {
		s.mu.RLock()
		if poolInfo, ok := s.poolsInfo[u.Addr]; ok {
			tokens := []common.Address{poolInfo.Tokens[0], poolInfo.Tokens[1]}
			poolsInfoOverride[u.Addr] = &PoolInfo{
				Reserves:     u.Reserves,
				Tokens:       tokens,
				FeeNumerator: poolInfo.FeeNumerator,
			}
			updatedKeys = append(updatedKeys,
				poolKeyFromAddrs(poolInfo.Tokens[0], poolInfo.Tokens[1], u.Addr),
				poolKeyFromAddrs(poolInfo.Tokens[1], poolInfo.Tokens[0], u.Addr))
		}
		s.mu.RUnlock()
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
	if len(allProfitableRoutes) == 0 {
		return
	}
	plan := s.getMostProfitablePath(allProfitableRoutes, poolsInfoOverride, ptx.Tx.GasPrice())
	if s.ID > 0 {
		log.Info("Computed route", "strategy", s.Name, "profitable", len(allProfitableRoutes), "/", candidateRoutes, "t", utils.PrettyDuration(time.Now().Sub(start)), "hash", ptx.Tx.Hash().Hex(), "gasPrice", ptx.Tx.GasPrice(), "size", ptx.Tx.Size(), "avoiding", len(ptx.AvoidPoolAddrs))
	}
	if plan == nil {
		return
	}
	// d.txLagRequestChan <- tx.Hash()
	// log.Info("Found plan", "name", s.Name, "plan", plan)
	s.RailgunChan <- &RailgunPacket{
		Type:         SwapSinglePath,
		StrategyID:   s.ID,
		Target:       ptx.Tx,
		Response:     plan,
		ValidatorIDs: ptx.ValidatorIDs,
	}
	if len(s.subStrategies) > 0 && len(ptx.AvoidPoolAddrs) == 0 {
		subPtx := &PossibleTx{
			Tx:      ptx.Tx,
			Updates: ptx.Updates,
		}
		for _, step := range plan.Path {
			if _, ok := poolsInfoOverride[step.Pair]; !ok {
				// Avoid pools that aren't updated by the target tx.
				subPtx.AvoidPoolAddrs = append(subPtx.AvoidPoolAddrs, &step.Pair)
			}
		}
		for _, sub := range s.subStrategies {
			sub.ProcessPossibleTx(subPtx)
		}
	}
}

func (s *LinearStrategy) getProfitableRoutes(key PoolKey, poolsInfoOverride map[common.Address]*PoolInfo) []uint {
	if routeIdxs, ok := s.routeCache.PoolToRouteIdxs[key]; ok {
		for i, routeIdx := range routeIdxs {
			route := s.routeCache.Routes[routeIdx]
			amountIn := convert(wftm, route[0].From, startTokensIn, s.aggregatePools)
			amountOut := s.getRouteAmountOut(route, amountIn, poolsInfoOverride)
			if amountOut.Cmp(amountIn) < 1 {
				return routeIdxs[:i]
			}
		}
	}
	return nil
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
func (s *LinearStrategy) getMostProfitablePath(routeIdxs []uint, poolsInfoOverride map[common.Address]*PoolInfo, gasPrice *big.Int) *Plan {
	maxProfit := new(big.Int)
	var bestAmountIn, bestAmountOut, bestGas *big.Int
	var bestRouteIdx uint
	for _, routeIdx := range routeIdxs {
		route := s.routeCache.Routes[routeIdx]
		amountIn := s.getRouteOptimalAmountIn(route, poolsInfoOverride)
		if amountIn.Sign() == -1 {
			log.Info("WARNING: Negative amountIn for route", "routeIdx", routeIdx, "amountIn", amountIn)
			continue
		}
		amountOut := s.getRouteAmountOut(route, amountIn, poolsInfoOverride)
		if amountOut.Cmp(amountIn) == -1 {
			continue
		}
		profit := new(big.Int).Sub(amountOut, amountIn)
		if bytes.Compare(route[0].From.Bytes(), wftm.Bytes()) != 0 {
			profit = convert(route[0].From, wftm, profit, s.aggregatePools) // -> wftm
		}
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
	log.Info("Best route", "strategy", s.Name, "routeIdx", bestRouteIdx, "bestAmountIn", bestAmountIn, "bestAmountOut", bestAmountOut, "bestGas", bestGas, "maxProfit", maxProfit)
	return s.makePlan(bestRouteIdx, bestGas, gasPrice, bestAmountIn, maxProfit)
}

func (s *LinearStrategy) makePlan(routeIdx uint, gasCost, gasPrice, amountIn, netProfit *big.Int) *Plan {
	route := s.routeCache.Routes[routeIdx]
	minProfit := gasCost
	if bytes.Compare(route[0].From.Bytes(), wftm.Bytes()) != 0 {
		minProfit = convert(wftm, route[0].From, gasCost, s.aggregatePools)
	}
	plan := &Plan{
		GasPrice:  gasPrice,
		GasCost:   gasCost,
		NetProfit: netProfit,
		MinProfit: minProfit,
		AmountIn:  amountIn,
		Path:      make([]fish4_lite.LinearSwapCommand, len(route)),
	}
	for i, leg := range route {
		s.mu.RLock()
		poolInfo := s.poolsInfo[leg.PoolAddr] // No need to use override as we don't look up reserves
		s.mu.RUnlock()
		fromToken0 := bytes.Compare(poolInfo.Tokens[0].Bytes(), leg.From.Bytes()) == 0
		plan.Path[i] = fish4_lite.LinearSwapCommand{
			Pair:         leg.PoolAddr,
			Token0:       poolInfo.Tokens[0],
			Token1:       poolInfo.Tokens[1],
			FeeNumerator: poolInfo.FeeNumerator,
			FromToken0:   fromToken0,
		}
	}
	return plan
}

func (s *LinearStrategy) getRouteOptimalAmountIn(route []*Leg, poolsInfoOverride map[common.Address]*PoolInfo) *big.Int {
	s.mu.RLock()
	startPoolInfo := getPoolInfo(s.poolsInfo, poolsInfoOverride, route[0].PoolAddr)
	s.mu.RUnlock()
	leftAmount, rightAmount := new(big.Int), new(big.Int)
	if bytes.Compare(startPoolInfo.Tokens[0].Bytes(), route[0].From.Bytes()) == 0 {
		leftAmount.Set(startPoolInfo.Reserves[0])
		rightAmount.Set(startPoolInfo.Reserves[1])
	} else {
		leftAmount.Set(startPoolInfo.Reserves[1])
		rightAmount.Set(startPoolInfo.Reserves[0])
	}
	r1 := startPoolInfo.FeeNumerator
	tenToSix := big.NewInt(int64(1e6))
	for _, leg := range route[1:] {
		s.mu.RLock()
		poolInfo := getPoolInfo(s.poolsInfo, poolsInfoOverride, leg.PoolAddr)
		s.mu.RUnlock()
		var reserveFrom, reserveTo *big.Int
		if bytes.Compare(poolInfo.Tokens[0].Bytes(), leg.From.Bytes()) == 0 {
			reserveFrom, reserveTo = poolInfo.Reserves[0], poolInfo.Reserves[1]
		} else {
			reserveTo, reserveFrom = poolInfo.Reserves[0], poolInfo.Reserves[1]
		}
		legFee := poolInfo.FeeNumerator
		den := new(big.Int).Mul(rightAmount, legFee)
		den = den.Div(den, tenToSix)
		den = den.Add(den, reserveFrom)
		leftAmount = leftAmount.Mul(leftAmount, reserveFrom)
		leftAmount = leftAmount.Div(leftAmount, den)
		rightAmount = rightAmount.Mul(rightAmount, reserveTo)
		rightAmount = rightAmount.Mul(rightAmount, legFee)
		rightAmount = rightAmount.Div(rightAmount, tenToSix)
		rightAmount = rightAmount.Div(rightAmount, den)
	}
	amountIn := new(big.Int).Mul(rightAmount, leftAmount)
	amountIn = amountIn.Mul(amountIn, r1)
	amountIn = amountIn.Div(amountIn, tenToSix)
	amountIn = amountIn.Sqrt(amountIn)
	amountIn = amountIn.Sub(amountIn, leftAmount)
	amountIn = amountIn.Mul(amountIn, tenToSix)
	amountIn = amountIn.Div(amountIn, r1)
	if amountIn.Cmp(MaxAmountIn) == 1 { // amountIn > MaxAmountIn
		return MaxAmountIn
	}
	return amountIn
}
