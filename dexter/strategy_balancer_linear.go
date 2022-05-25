package dexter

import (
	"bytes"
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
		for a, r := range v.Reserves {
			reserves[a] = BigIntToFloat(r)
			weights[a] = BigIntToFloat(v.Weights[a])
		}

		poolInfo := &PoolInfoFloat{
			Tokens:         v.Tokens,
			Reserves:       reserves,
			Weights:        weights,
			FeeNumerator:   BigIntToFloat(v.FeeNumerator),
			FeeNumeratorBI: v.FeeNumerator,
			Fee:            BigIntToFloat(v.Fee) / 1e18,
			FeeBI:          v.Fee,
			LastUpdate:     time.Now(),
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
	scores := s.makeScores()
	s.routeCache.PoolToRouteIdxs = s.sortPoolToRouteIdxMap(scores)
	go s.runPermUpdater(scores)
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
			amountOut = getAmountOutBalancer(amountIn, reserveFrom, reserveTo, weightFrom, weightTo, poolInfo.Fee)
			if debug {
				log.Info("Leg: balancerWeighted", "i", i, "reserveFrom", reserveFrom, "reserveTo", reserveTo, "feeNumerator", poolInfo.Fee, "weightFrom", weightFrom, "weightTo", weightTo, "amountIn", amountIn, "amountOut", amountOut)
			}
		} else if leg.Type == BalancerStablePool {
			amountOut = getAmountOutBalancerStable(
				amountIn, poolInfo.Fee, poolInfo.AmplificationParam, poolInfo.Reserves, leg.From, leg.To)
			if debug {
				log.Info("Leg: balancerStable", "i", i, "reserveFrom", reserveFrom, "reserveTo", reserveTo, "feeNumerator", poolInfo.Fee, "amountIn", amountIn, "amountOut", amountOut)
			}
		}
		amountIn = amountOut
	}
	return amountOut
}

func calcStableInvariant(amp float64, balances map[common.Address]float64) float64 {
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
		inv = (numToks*inv*inv + ampTimesTotal*sum*P_D) / (inv*(numToks+1) + (ampTimesTotal-1)*P_D)
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

func getTokenBalanceGivenInvAndBalances(amp, inv float64, balances map[common.Address]float64, tokenOut common.Address) float64 {
	numToks := float64(len(balances))
	ampTimesTotal := amp * numToks
	sum := 0.
	P_D := 1.
	for _, bal := range balances {
		P_D = P_D * bal * numToks / inv
		sum += bal
	}
	sum -= balances[tokenOut]
	P_D *= inv
	c := inv * inv * balances[tokenOut] / (ampTimesTotal * P_D)
	b := sum + (inv / ampTimesTotal * 1e18)
	prevBal := 0.
	currentBal := ((inv*inv+c)-1)/(inv+b) + 1
	for i := 0; i < 255; i++ {
		prevBal = currentBal
		currentBal = (currentBal*currentBal/1e18 + c) * 1e18 / (2*currentBal + b - inv)
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

func getAmountOutBalancerStable(amountIn, fee, amp float64, balances map[common.Address]float64, tokenIn, tokenOut common.Address) float64 {
	amountIn = amountIn * (1 - fee)
	inv := calcStableInvariant(amp, balances)
	balances[tokenIn] += amountIn
	finalOut := getTokenBalanceGivenInvAndBalances(amp, inv, balances, tokenOut)
	balances[tokenIn] -= amountIn
	return balances[tokenOut] - finalOut - 1
}

func (s *BalancerLinearStrategy) sortPoolToRouteIdxMap(scores []float64) map[PoolKey][]uint {
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

func (s *BalancerLinearStrategy) makeScores() []float64 {
	scores := make([]float64, len(s.routeCache.Routes))
	for i, route := range s.routeCache.Routes {
		scores[i] = s.getScore(route, nil)
	}
	return scores
}

func (s *BalancerLinearStrategy) getScore(route []*Leg, poolsInfoOverride map[common.Address]*PoolInfoFloat) float64 {
	unitPrice := s.getRouteAmountOutBalancer(route, unitIn, poolsInfoOverride, false) / unitIn
	// kiloPrice := s.getRouteAmountOutBalancer(route, kiloIn, poolsInfoOverride, false) / kiloIn
	return unitPrice // + kiloPrice
}

func (s *BalancerLinearStrategy) refreshScoresForPool(
	key PoolKey, poolsInfoOverride map[common.Address]*PoolInfoFloat, scores []float64) {
	if routeIdxs, ok := s.routeCache.PoolToRouteIdxs[key]; ok {
		for _, routeIdx := range routeIdxs {
			route := s.routeCache.Routes[routeIdx]
			scores[routeIdx] = s.getScore(route, poolsInfoOverride)
		}
	}
}

func (s *BalancerLinearStrategy) runPermUpdater(scores []float64) {
	for {
		us := <-s.inPermUpdatesChan
		poolsInfoUpdates := make(map[common.Address]*PoolInfoFloat)
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
				Reserves:           reserves,
				Tokens:             poolInfo.Tokens,
				Weights:            poolInfo.Weights,
				AmplificationParam: poolInfo.AmplificationParam,
				Fee:                poolInfo.Fee,
				FeeNumerator:       poolInfo.FeeNumerator,
				FeeBI:              poolInfo.FeeBI,
				FeeNumeratorBI:     poolInfo.FeeNumeratorBI,
				LastUpdate:         time.Now(),
			}
			for _, token0 := range poolInfo.Tokens {
				for _, token1 := range poolInfo.Tokens {
					if token0 == token1 {
						continue
					}
					key := poolKeyFromAddrs(token0, token1, u.Addr)
					s.refreshScoresForPool(key, poolsInfoUpdates, scores)
				}
			}
		}
		if len(poolsInfoUpdates) > 0 {
			poolToRouteIdxs := s.sortPoolToRouteIdxMap(scores)
			update := &PoolsInfoUpdateFloat{
				PoolsInfoUpdates: poolsInfoUpdates,
				PoolToRouteIdxs:  poolToRouteIdxs,
			}
			s.poolsInfoUpdateChan <- update
		}
	}
}

func (s *BalancerLinearStrategy) runStrategy() {
	for {
		select {
		case update := <-s.poolsInfoUpdateChan:
			s.mu.Lock()
			s.routeCache.PoolToRouteIdxs = update.PoolToRouteIdxs
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
			reserves[a] = BigIntToFloat(r)
		}
		poolsInfoOverride[u.Addr] = &PoolInfoFloat{
			Reserves:           reserves,
			Tokens:             poolInfo.Tokens,
			Weights:            poolInfo.Weights,
			AmplificationParam: poolInfo.AmplificationParam,
			Fee:                poolInfo.Fee,
			FeeNumerator:       poolInfo.FeeNumerator,
			FeeBI:              poolInfo.FeeBI,
			FeeNumeratorBI:     poolInfo.FeeNumeratorBI,
			LastUpdate:         time.Now(),
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
	for _, key := range updatedKeys {
		pop = append(pop, s.getProfitableRoutes(key, poolsInfoOverride)...)
		candidateRoutes += len(s.routeCache.PoolToRouteIdxs[key])
	}
	if len(pop) < 2 {
		return
	}
	// log.Info("strategy_balancer_linear full routes", "profitable", len(pop), "/", candidateRoutes, "t", utils.PrettyDuration(time.Now().Sub(ptx.StartTime)), "hash", ptx.Tx.Hash().Hex(), "gasPrice", ptx.Tx.GasPrice())
	plan := s.getMostProfitablePath(pop, poolsInfoOverride, ptx.Tx.GasPrice())
	if plan == nil {
		return
	}
	s.routeCache.LastFiredTime[plan.RouteIdx] = time.Now()
	// log.Info("strategy_balancer_linear final route", "strategy", s.Name, "profitable", len(pop), "/", candidateRoutes, "t", utils.PrettyDuration(time.Now().Sub(ptx.StartTime)), "hash", ptx.Tx.Hash().Hex(), "gasPrice", ptx.Tx.GasPrice())
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
	if routeIdxs, ok := s.routeCache.PoolToRouteIdxs[key]; ok {
		for _, routeIdx := range routeIdxs {
			if now.Sub(s.routeCache.LastFiredTime[routeIdx]) < 4*time.Second {
				continue
			}
			route := s.routeCache.Routes[routeIdx]
			amountOutUnit := s.getRouteAmountOutBalancer(route, unitIn, poolsInfoOverride, false)
			if amountOutUnit < unitIn {
				return pop
			}
			pop = append(pop, Candidate{
				DiscreteGene:   routeIdx,
				ContinuousGene: unitIn,
				Fitness:        amountOutUnit - unitIn,
			})
			for _, in := range inCandidates {
				amountIn := in + rand.NormFloat64()*in/4
				amountOut := s.getRouteAmountOutBalancer(route, amountIn, poolsInfoOverride, false)
				pop = append(pop, Candidate{
					DiscreteGene:   routeIdx,
					ContinuousGene: amountIn,
					Fitness:        amountOut - amountIn,
				})
				if amountOut < amountIn {
					break
				}
			}
		}
	}
	return pop
}

func (s *BalancerLinearStrategy) getMostProfitablePath(pop Population, poolsInfoOverride map[common.Address]*PoolInfoFloat, gasPrice *big.Int) *Plan {
	// start := time.Now()
	// log.Info("Initial population", "len", len(pop))
	// sort.Sort(pop)
	// if len(pop) < 10 {
	// 	pop.Print()
	// } else {
	// 	pop[:10].Print()
	// }
	// log.Info("Performing evolution on population", "size", len(pop))
	for i := 0; i < 3; i++ {
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
	gas := estimateFishGasFloat(5, len(route), gasPrice)
	// log.Info("Best route", "routeIdx", bestRouteIdx, "bestAmountIn", bestAmountIn, "bestAmountOut", bestAmountOut, "bestGas", bestGas)
	netProfit := winner.Fitness - gas
	if netProfit < 0 {
		return nil
	}
	// log.Info("Found best final candidate", "route", winner.DiscreteGene, "amountIn", winner.ContinuousGene/1e18, "profit", winner.Fitness/1e18, "time", utils.PrettyDuration(time.Now().Sub(start)))
	// s.getRouteAmountOutBalancer(route, winner.ContinuousGene, poolsInfoOverride, true)
	return s.makePlan(winner.DiscreteGene, gas, winner.ContinuousGene, netProfit, gasPrice)
}

func (s *BalancerLinearStrategy) makePlan(routeIdx uint, gasCost, amountIn, netProfit float64, gasPrice *big.Int) *Plan {
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
	}
	for i, leg := range route {
		s.mu.RLock()
		poolInfo := s.poolsInfo[leg.PoolAddr] // No need to use override as we don't look up reserves
		s.mu.RUnlock()
		if leg.Type == UniswapV2Pair {
			fromToken0 := bytes.Compare(poolInfo.Tokens[0].Bytes(), leg.From.Bytes()) == 0
			plan.Path[i] = fish5_lite.LinearSwapCommand{
				Token0:       poolInfo.Tokens[0],
				Token1:       poolInfo.Tokens[1],
				FeeNumerator: poolInfo.FeeNumeratorBI,
				FromToken0:   fromToken0,
				PoolType:     uint8(leg.Type),
			}
			copy(plan.Path[i].PoolId[:], leg.PoolAddr.Bytes())
		} else {
			plan.Path[i] = fish5_lite.LinearSwapCommand{
				Token0:     leg.From,
				Token1:     leg.To,
				FromToken0: true,
				PoolType:   uint8(leg.Type),
			}
			plan.Path[i].FeeNumerator = poolInfo.FeeBI
			plan.Path[i].PoolId = leg.PoolId
		}
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
