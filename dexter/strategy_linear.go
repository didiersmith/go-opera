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

	"github.com/Fantom-foundation/go-opera/contracts/fish3_lite"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

type LinearStrategy struct {
	Name                string
	ID                  int
	RailgunChan         chan *RailgunPacket
	inPossibleTxsChan   chan *PossibleTx
	inPermUpdatesChan   chan []*PairUpdate
	cfg                 LinearStrategyConfig
	pairsInfo           map[common.Address]*PairInfo
	pairsInfoUpdateChan chan *PairsInfoUpdate
	interestedPairs     map[common.Address]struct{}
	routeCache          RouteCache
	subStrategies       []Strategy
	mu                  sync.RWMutex
}

type LinearStrategyConfig struct {
	RoutesFileName          string
	PairToRouteIdxsFileName string
	SelectSecondBest        bool
}

func NewLinearStrategy(name string, id int, railgun chan *RailgunPacket, cfg LinearStrategyConfig) Strategy {
	s := &LinearStrategy{
		Name:                name,
		ID:                  id,
		RailgunChan:         railgun,
		inPossibleTxsChan:   make(chan *PossibleTx, 256),
		inPermUpdatesChan:   make(chan []*PairUpdate, 256),
		cfg:                 cfg,
		pairsInfo:           make(map[common.Address]*PairInfo),
		pairsInfoUpdateChan: make(chan *PairsInfoUpdate),
		interestedPairs:     make(map[common.Address]struct{}),
	}
	s.loadJson()
	return s
}

func (s *LinearStrategy) SetPairsInfo(pairsInfo map[common.Address]*PairInfo) {
	for k, v := range pairsInfo {
		s.pairsInfo[k] = v
	}
}

func (s *LinearStrategy) ProcessPossibleTx(t *PossibleTx) {
	s.inPossibleTxsChan <- t
}

func (s *LinearStrategy) ProcessPermUpdates(us []*PairUpdate) {
	s.inPermUpdatesChan <- us
}

func (s *LinearStrategy) GetInterestedPairs() map[common.Address]struct{} {
	return s.interestedPairs
}

func (s *LinearStrategy) AddSubStrategy(sub Strategy) {
	s.subStrategies = append(s.subStrategies, sub)
}

func (s *LinearStrategy) Start() {
	scores := s.makeScores()
	s.routeCache.PairToRouteIdxs = s.sortPairToRouteIdxMap(scores)
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
	var routeCacheJson RouteCacheJson
	json.Unmarshal(routeCacheRoutesBytes, &(routeCacheJson.Routes))
	log.Info("Loaded routes")

	routeCachePairToRouteIdxsFile, err := os.Open(s.cfg.PairToRouteIdxsFileName)
	if err != nil {
		log.Info("Error opening routeCachePairToRouteIdxs", "routeCachePairToRouteIdxsFileName", s.cfg.PairToRouteIdxsFileName, "err", err)
		return
	}
	defer routeCachePairToRouteIdxsFile.Close()
	routeCachePairToRouteIdxsBytes, _ := ioutil.ReadAll(routeCachePairToRouteIdxsFile)
	json.Unmarshal(routeCachePairToRouteIdxsBytes, &(routeCacheJson.PairToRouteIdxs))
	log.Info("Loaded pairToRouteIdxs")

	routeCache := RouteCache{
		Routes:          make([][]*Leg, len(routeCacheJson.Routes)),
		PairToRouteIdxs: make(map[PairKey][]uint),
	}
	for i, routeJson := range routeCacheJson.Routes {
		route := make([]*Leg, len(routeJson))
		for x, leg := range routeJson {
			pairAddr := common.HexToAddress(leg.PairAddr)
			s.interestedPairs[pairAddr] = struct{}{}
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
	log.Info("Processed route cache", "name", s.Name)
	s.routeCache = routeCache
}

func (s *LinearStrategy) runPermUpdater(scores []uint64) {
	for {
		us := <-s.inPermUpdatesChan
		pairsInfoUpdates := make(map[common.Address]*PairInfo)
		for _, u := range us {
			s.mu.RLock()
			pairInfo, ok := s.pairsInfo[u.Addr]
			s.mu.RUnlock()
			if !ok {
				continue
			}
			pairsInfoUpdates[u.Addr] = &PairInfo{
				Reserves:     u.Reserves,
				Token0:       pairInfo.Token0,
				Token1:       pairInfo.Token1,
				FeeNumerator: pairInfo.FeeNumerator,
			}
			key0 := pairKeyFromAddrs(pairInfo.Token0, pairInfo.Token1, u.Addr)
			key1 := pairKeyFromAddrs(pairInfo.Token1, pairInfo.Token0, u.Addr)
			s.refreshScoresForPair(key0, pairsInfoUpdates, scores) // Updates scores
			s.refreshScoresForPair(key1, pairsInfoUpdates, scores)
		}
		if len(pairsInfoUpdates) > 0 {
			pairToRouteIdxs := s.sortPairToRouteIdxMap(scores)
			update := &PairsInfoUpdate{
				PairsInfoUpdates: pairsInfoUpdates,
				PairToRouteIdxs:  pairToRouteIdxs,
			}
			s.pairsInfoUpdateChan <- update
		}
	}
}

func (s *LinearStrategy) runStrategy() {
	for {
		select {
		case update := <-s.pairsInfoUpdateChan:
			// log.Info("Received update", "pairs", len(update.PairsInfoUpdates), "PairToRouteIdxs", len(update.PairToRouteIdxs))
			s.mu.Lock()
			s.routeCache.PairToRouteIdxs = update.PairToRouteIdxs
			for pairAddr, pairInfo := range update.PairsInfoUpdates {
				s.pairsInfo[pairAddr] = pairInfo
			}
			s.mu.Unlock()
		case p := <-s.inPossibleTxsChan:
			s.processPotentialTx(p)
		}
	}
}

func (s *LinearStrategy) sortPairToRouteIdxMap(scores []uint64) map[PairKey][]uint {
	pairToRouteIdxs := make(map[PairKey][]uint, len(s.routeCache.PairToRouteIdxs))
	s.mu.RLock()
	for pairKey, routeIdxs := range s.routeCache.PairToRouteIdxs {
		newRouteIdxs := make([]uint, len(routeIdxs))
		copy(newRouteIdxs, routeIdxs)
		pairToRouteIdxs[pairKey] = newRouteIdxs
	}
	s.mu.RUnlock()
	for _, routeIdxs := range pairToRouteIdxs {
		sort.Slice(routeIdxs, func(a, b int) bool {
			return scores[routeIdxs[a]] > scores[routeIdxs[b]]
		})
	}
	return pairToRouteIdxs
}

func (s *LinearStrategy) getScore(route []*Leg, pairsInfoOverride map[common.Address]*PairInfo) uint64 {
	amountOut := s.getRouteAmountOut(route, startTokensIn, pairsInfoOverride)
	amountOut.Div(amountOut, big.NewInt(1e12))
	return uint64(amountOut.Int64())
}

func (s *LinearStrategy) makeScores() []uint64 {
	scores := make([]uint64, len(s.routeCache.Routes))
	for i, route := range s.routeCache.Routes {
		scores[i] = s.getScore(route, nil)
	}
	return scores
}

func (s *LinearStrategy) refreshScoresForPair(key PairKey, pairsInfoOverride map[common.Address]*PairInfo, scores []uint64) {
	if routeIdxs, ok := s.routeCache.PairToRouteIdxs[key]; ok {
		for _, routeIdx := range routeIdxs {
			route := s.routeCache.Routes[routeIdx]
			scores[routeIdx] = s.getScore(route, pairsInfoOverride)
		}
	}
}

func (s *LinearStrategy) getRouteAmountOut(route []*Leg, amountIn *big.Int, pairsInfoOverride map[common.Address]*PairInfo) *big.Int {
	var amountOut *big.Int
	for _, leg := range route {
		s.mu.RLock()
		pairInfo := getPairInfo(s.pairsInfo, pairsInfoOverride, leg.PairAddr)
		s.mu.RUnlock()
		var reserveFrom *big.Int
		var reserveTo *big.Int
		if bytes.Compare(pairInfo.Token0.Bytes(), leg.From.Bytes()) == 0 {
			reserveFrom, reserveTo = pairInfo.Reserves[0], pairInfo.Reserves[1]
		} else {
			reserveFrom, reserveTo = pairInfo.Reserves[1], pairInfo.Reserves[0]
		}
		amountOut = getAmountOutUniswap(amountIn, reserveFrom, reserveTo, pairInfo.FeeNumerator)
		amountIn = amountOut
	}
	return amountOut
}

func (s *LinearStrategy) processPotentialTx(ptx *PossibleTx) {
	// start := time.Now()
	pairsInfoOverride := make(map[common.Address]*PairInfo)
	var updatedKeys []PairKey
	for _, u := range ptx.Updates {
		s.mu.RLock()
		if pairInfo, ok := s.pairsInfo[u.Addr]; ok {
			pairsInfoOverride[u.Addr] = &PairInfo{
				Reserves:     u.Reserves,
				Token0:       pairInfo.Token0,
				Token1:       pairInfo.Token1,
				FeeNumerator: pairInfo.FeeNumerator,
			}
			updatedKeys = append(updatedKeys,
				pairKeyFromAddrs(pairInfo.Token0, pairInfo.Token1, u.Addr),
				pairKeyFromAddrs(pairInfo.Token1, pairInfo.Token0, u.Addr))
		}
		s.mu.RUnlock()
	}
	// log.Info("Finished processing tx", "hash", tx.Hash().Hex(), "t", utils.PrettyDuration(time.Now().Sub(start)), "updatedKeys", len(updatedKeys))
	// for pairAddr, pairInfo := range pairsInfoOverride {
	// 	log.Info("dexter.PairInfo override", "pairAddr", pairAddr, "reserve0", pairInfo.reserve0, "reserve1", pairInfo.reserve1, "info", *pairInfo)
	// }
	var allProfitableRoutes []uint
	candidateRoutes := 0
	for _, key := range updatedKeys {
		profitableRoutes := s.getProfitableRoutes(key, pairsInfoOverride)
		allProfitableRoutes = append(allProfitableRoutes, profitableRoutes...)
		candidateRoutes += len(s.routeCache.PairToRouteIdxs[key])
	}
	sort.Slice(allProfitableRoutes, func(a, b int) bool { return allProfitableRoutes[a] < allProfitableRoutes[b] })
	allProfitableRoutes = uniq(allProfitableRoutes)
	if len(ptx.AvoidPairAddrs) > 0 {
		allProfitableRoutes = s.filterRoutesAvoidPairs(allProfitableRoutes, ptx.AvoidPairAddrs)
	}
	if len(allProfitableRoutes) == 0 {
		return
	}
	plan := s.getMostProfitablePath(allProfitableRoutes, pairsInfoOverride, ptx.Tx.GasPrice())
	if plan == nil {
		return
	}
	// log.Info("Computed route", "strategy", s.Name, "profitable", len(allProfitableRoutes), "/", candidateRoutes, "t", utils.PrettyDuration(time.Now().Sub(start)), "hash", ptx.Tx.Hash().Hex(), "gasPrice", ptx.Tx.GasPrice(), "size", ptx.Tx.Size(), "avoiding", len(ptx.AvoidPairAddrs))
	// d.txLagRequestChan <- tx.Hash()
	// log.Info("Found plan", "name", s.Name, "plan", plan)
	s.RailgunChan <- &RailgunPacket{
		Type:         SwapSinglePath,
		StrategyID:   s.ID,
		Target:       ptx.Tx,
		Response:     plan,
		ValidatorIDs: ptx.ValidatorIDs,
	}
	if len(s.subStrategies) > 0 && len(ptx.AvoidPairAddrs) == 0 {
		subPtx := &PossibleTx{
			Tx:      ptx.Tx,
			Updates: ptx.Updates,
		}
		for _, step := range plan.Path {
			if _, ok := pairsInfoOverride[step.Pair]; !ok {
				// Avoid pairs that aren't updated by the target tx.
				subPtx.AvoidPairAddrs = append(subPtx.AvoidPairAddrs, &step.Pair)
			}
		}
		for _, sub := range s.subStrategies {
			sub.ProcessPossibleTx(subPtx)
		}
	}
}

func (s *LinearStrategy) getProfitableRoutes(key PairKey, pairsInfoOverride map[common.Address]*PairInfo) []uint {
	if routeIdxs, ok := s.routeCache.PairToRouteIdxs[key]; ok {
		for i, routeIdx := range routeIdxs {
			route := s.routeCache.Routes[routeIdx]
			amountOut := s.getRouteAmountOut(route, startTokensIn, pairsInfoOverride)
			if amountOut.Cmp(startTokensIn) < 1 {
				return routeIdxs[:i]
			}
		}
	}
	return nil
}

func (s *LinearStrategy) filterRoutesAvoidPairs(routeIdxs []uint, avoidPairs []*common.Address) []uint {
	var filteredIdxs []uint
	for _, routeIdx := range routeIdxs {
		route := s.routeCache.Routes[routeIdx]
		found := false
		for _, leg := range route {
			if found {
				break
			}
			for _, pairAddr := range avoidPairs {
				if bytes.Compare(pairAddr.Bytes(), leg.PairAddr.Bytes()) == 0 {
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
func (s *LinearStrategy) getMostProfitablePath(routeIdxs []uint, pairsInfoOverride map[common.Address]*PairInfo, gasPrice *big.Int) *Plan {
	maxProfit := new(big.Int)
	var bestAmountIn, bestAmountOut, bestGas *big.Int
	var bestRouteIdx uint
	for _, routeIdx := range routeIdxs {
		route := s.routeCache.Routes[routeIdx]
		amountIn := s.getRouteOptimalAmountIn(route, pairsInfoOverride)
		if amountIn.Sign() == -1 {
			log.Info("WARNING: Negative amountIn for route", "routeIdx", routeIdx, "amountIn", amountIn)
			amountOut := s.getRouteAmountOut(route, startTokensIn, pairsInfoOverride)
			log.Info("Amount out for startTokensIn", "routeIdx", routeIdx, "amountOut", amountOut)

			continue
		}
		amountOut := s.getRouteAmountOut(route, amountIn, pairsInfoOverride)
		if amountOut.Cmp(amountIn) == -1 {
			continue
		}
		profit := new(big.Int).Sub(amountOut, amountIn)
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
	return s.makePlan(bestRouteIdx, bestGas, gasPrice, bestAmountIn, maxProfit)
}

func (s *LinearStrategy) makePlan(routeIdx uint, gasCost, gasPrice, amountIn, netProfit *big.Int) *Plan {
	route := s.routeCache.Routes[routeIdx]
	plan := &Plan{
		GasPrice:  gasPrice,
		GasCost:   gasCost,
		NetProfit: netProfit,
		AmountIn:  amountIn,
		Path:      make([]fish3_lite.SwapCommand, len(route)),
	}
	for i, leg := range route {
		s.mu.RLock()
		pairInfo := s.pairsInfo[leg.PairAddr] // No need to use override as we don't look up reserves
		s.mu.RUnlock()
		fromToken0 := bytes.Compare(pairInfo.Token0.Bytes(), leg.From.Bytes()) == 0
		plan.Path[i] = fish3_lite.SwapCommand{
			Pair:         leg.PairAddr,
			Token0:       pairInfo.Token0,
			Token1:       pairInfo.Token1,
			Fraction:     big.NewInt(1),
			FeeNumerator: pairInfo.FeeNumerator,
			FromToken0:   fromToken0,
			SwapTo:       0,
		}
	}
	return plan
}

func (s *LinearStrategy) getRouteOptimalAmountIn(route []*Leg, pairsInfoOverride map[common.Address]*PairInfo) *big.Int {
	s.mu.RLock()
	startPairInfo := getPairInfo(s.pairsInfo, pairsInfoOverride, route[0].PairAddr)
	s.mu.RUnlock()
	leftAmount, rightAmount := new(big.Int), new(big.Int)
	if bytes.Compare(startPairInfo.Token0.Bytes(), route[0].From.Bytes()) == 0 {
		leftAmount.Set(startPairInfo.Reserves[0])
		rightAmount.Set(startPairInfo.Reserves[1])
	} else {
		leftAmount.Set(startPairInfo.Reserves[1])
		rightAmount.Set(startPairInfo.Reserves[0])
	}
	r1 := startPairInfo.FeeNumerator
	tenToSix := big.NewInt(int64(1e6))
	for _, leg := range route[1:] {
		s.mu.RLock()
		pairInfo := getPairInfo(s.pairsInfo, pairsInfoOverride, leg.PairAddr)
		s.mu.RUnlock()
		var reserveFrom, reserveTo *big.Int
		if bytes.Compare(pairInfo.Token0.Bytes(), leg.From.Bytes()) == 0 {
			reserveFrom, reserveTo = pairInfo.Reserves[0], pairInfo.Reserves[1]
		} else {
			reserveTo, reserveFrom = pairInfo.Reserves[0], pairInfo.Reserves[1]
		}
		legFee := pairInfo.FeeNumerator
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
