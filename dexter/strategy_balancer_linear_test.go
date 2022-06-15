package dexter

import (
	"fmt"
	"math"
	"testing"

	"github.com/Fantom-foundation/go-opera/logger"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

var (
	SpookyWftmBtc           = common.HexToAddress("0xfdb9ab8b9513ad9e419cf19530fee49d412c3ee3")
	SpookyWftmUsdc          = common.HexToAddress("0x2b4c76d0dc16be1c31d4c1dc53bf9b45987fc75c")
	ALateQuartet            = common.HexToAddress("0xf3a602d30dcb723a74a0198313a7551feaca7dac")
	OneGodBetweenTwoStables = common.HexToAddress("0x8b858eaf095a7337de6f9bc212993338773ca34e")
)

func TestBalancerLinearStrategy(t *testing.T) {
	fmt.Println("Testing strategy_balancer_linear.go")
	logger.SetTestMode(t)
	logger.SetLevel("debug")

	root := "/home/ubuntu/dexter/carb/"
	s := NewBalancerLinearStrategy(
		"Balancer Linear", 0, nil, BalancerLinearStrategyConfig{
			RoutesFileName:          root + "route_caches/solidly_balancer_routes_len2-3.json",
			PoolToRouteIdxsFileName: root + "route_caches/solidly_balancer_poolToRouteIdxs_len2-3.json",
		})
	// s := NewBalancerLinearStrategy(
	// 	"Balancer Linear", 0, nil, BalancerLinearStrategyConfig{
	// 		RoutesFileName:          root + "route_caches/solidly_balancer_routes_len2-4.json",
	// 		PoolToRouteIdxsFileName: root + "route_caches/solidly_balancer_poolToRouteIdxs_len2-4.json",
	// 	})
	b := s.(*BalancerLinearStrategy)

	t.Run("Intialize", func(t *testing.T) {
		assert := assert.New(t)
		assert.Equal(b.Name, "Balancer Linear", "Wrong name")
		assert.Equal(b.ID, 0, "Wrong ID")
		assert.Equal(b.cfg.RoutesFileName, root+"route_caches/solidly_balancer_routes_len2-3.json", "Wrong ID")
		assert.Equal(b.cfg.PoolToRouteIdxsFileName, root+"route_caches/solidly_balancer_poolToRouteIdxs_len2-3.json", "Wrong ID")
		assert.Equal(b.cfg.SelectSecondBest, false, "Wrong ID")
		assert.Equal(len(b.subStrategies), 0, "Wrong subStrategies")
	})

	t.Run("loadJson", func(t *testing.T) {
		assert := assert.New(t)
		assert.Greater(len(b.interestedPools), 0, "No intereseted pools")
		assert.Greater(len(b.routeCache.Routes), 0, "No routes in routeCache")
		assert.Greater(len(b.routeCache.PoolToRouteIdxs), 0, "No poolToRouteIdxs in routeCache")
	})

	// 	t.Run("SetPoolsInfo", func(t *testing.T) {
	// 		// Fees and tokens for uniswap pools
	// 		poolsInfo := make(map[common.Address]*PoolInfo)
	// 		edgePools := make(map[EdgeKey][]common.Address)
	// 		poolsFileName := root + "pairs.json"
	// 		poolsFile, err := os.Open(poolsFileName)
	// 		assert.Nil(t, err)
	// 		defer poolsFile.Close()
	// 		poolsBytes, _ := ioutil.ReadAll(poolsFile)
	// 		var jsonPools []PoolInfoJson
	// 		json.Unmarshal(poolsBytes, &jsonPools)
	// 		for _, jsonPool := range jsonPools {
	// 			poolAddr := common.HexToAddress(jsonPool.Addr)
	// 			poolInfo := &PoolInfo{
	// 				FeeNumerator: big.NewInt(jsonPool.FeeNumerator),
	// 				Reserves:     make(map[common.Address]*big.Int),
	// 				Tokens: []common.Address{
	// 					common.HexToAddress(jsonPool.Token0),
	// 					common.HexToAddress(jsonPool.Token1),
	// 				},
	// 				Type: UniswapV2Pair,
	// 			}
	// 			poolsInfo[poolAddr] = poolInfo
	// 			edgeKey := MakeEdgeKey(poolInfo.Tokens[0], poolInfo.Tokens[1])
	// 			if pools, ok := edgePools[edgeKey]; ok {
	// 				edgePools[edgeKey] = append(pools, poolAddr)
	// 			} else {
	// 				edgePools[edgeKey] = []common.Address{poolAddr}
	// 			}
	// 		}
	// 		fmt.Println("\tSetPoolsInfo: \t\t Loaded pools")

	// 		// Reserves for uniswap pools
	// 		reservesFileName := root + "data/pair_reserves.json"
	// 		reservesFile, err := os.Open(reservesFileName)
	// 		assert.Nil(t, err)
	// 		defer reservesFile.Close()
	// 		reservesBytes, _ := ioutil.ReadAll(reservesFile)
	// 		jsonReserves := make(map[string]map[string]string)
	// 		err = json.Unmarshal(reservesBytes, &jsonReserves)
	// 		assert.Nil(t, err)
	// 		for pairAddrHex, reserves := range jsonReserves {
	// 			pairAddr := common.HexToAddress(pairAddrHex)
	// 			for tokAddrHex, reservesStr := range reserves {
	// 				tokAddr := common.HexToAddress(tokAddrHex)
	// 				poolsInfo[pairAddr].Reserves[tokAddr] = StringToBigInt(reservesStr)
	// 			}
	// 		}
	// 		fmt.Printf("Spooky wftm btc: %v\n", poolsInfo[SpookyWftmBtc])
	// 		fmt.Printf("Spooky wftm usdc: %v\n", poolsInfo[SpookyWftmUsdc])
	// 		fmt.Println("\tSetPoolsInfo: \t\tPASS")

	// 		// Balancer pools
	// 		balPoolsFileName := root + "data/beets_formatted.json"
	// 		balPoolsFile, err := os.Open(balPoolsFileName)
	// 		assert.Nil(t, err)
	// 		defer balPoolsFile.Close()
	// 		balPoolsBytes, _ := ioutil.ReadAll(balPoolsFile)
	// 		jsonBalPools := make(map[string]map[string][]BalancerPoolJson)
	// 		fmt.Println("Unmarshalling")
	// 		err = json.Unmarshal(balPoolsBytes, &jsonBalPools)
	// 		assert.Nil(t, err)
	// 		data := jsonBalPools["data"]
	// 		pools := data["pools"]
	// 		for _, pool := range pools {
	// 			var pt PoolType
	// 			if pool.PoolType == "Weighted" {
	// 				pt = BalancerWeightedPool
	// 			} else if pool.PoolType == "Stable" {
	// 				pt = BalancerStablePool
	// 			} else {
	// 				continue
	// 			}
	// 			address := common.HexToAddress(pool.Address)
	// 			swapFee, err := strconv.ParseFloat(pool.SwapFee, 10)
	// 			poolInfo := &PoolInfo{
	// 				Fee:          FloatToBigInt(swapFee * 1e18),
	// 				Reserves:     make(map[common.Address]*big.Int),
	// 				Weights:      make(map[common.Address]*big.Int),
	// 				ScaleFactors: make(map[common.Address]*big.Int),
	// 				Tokens:       make([]common.Address, len(pool.Tokens)),
	// 				LastUpdate:   time.Now(),
	// 				Type:         pt,
	// 			}
	// 			amp, err := strconv.ParseFloat(pool.Amp, 10)
	// 			if err == nil {
	// 				poolInfo.AmplificationParam = FloatToBigInt(amp * 1e18)
	// 			}
	// 			for i, tok := range pool.Tokens {
	// 				tokAddr := common.HexToAddress(tok.Address)
	// 				poolInfo.Tokens[i] = tokAddr
	// 				decimalsDiff := int64(18 - tok.Decimals)
	// 				scaleFactor := new(big.Int).Exp(big.NewInt(10), big.NewInt(decimalsDiff), nil)
	// 				poolInfo.ScaleFactors[tokAddr] = scaleFactor
	// 				if tok.Weight != "" {
	// 					weightFloat, err := strconv.ParseFloat(tok.Weight, 10)
	// 					assert.Nil(t, err)
	// 					poolInfo.Weights[tokAddr] = big.NewInt(int64(weightFloat * 1e18))
	// 				}
	// 				balance, _ := strconv.ParseFloat(tok.Balance, 10)
	// 				balance *= math.Pow(10, float64(tok.Decimals))
	// 				poolInfo.Reserves[tokAddr] = FloatToBigInt(balance)

	// 			}
	// 			poolsInfo[address] = poolInfo
	// 		}
	// 		fmt.Printf("ALateQuartet: %v\n", poolsInfo[ALateQuartet])
	// 		fmt.Printf("OneGodBetweenTwoStables: %v\n", poolsInfo[OneGodBetweenTwoStables])
	// 		b.SetPoolsInfo(poolsInfo)
	// 		b.SetEdgePools(edgePools)
	// 		b.aggregatePools = makeAggregatePoolsFloat(b.edgePools, b.poolsInfo, nil)
	// 		for i := 0; i < len(ScoreTiers); i++ {
	// 			b.routeCache.Scores[i] = b.makeScores(ScoreTiers[i])
	// 			for idx, score := range b.routeCache.Scores[i] {
	// 				if math.IsNaN(score) {
	// 					fmt.Printf("NaN score: %d\n", idx)
	// 					b.getRouteAmountOutBalancer(b.routeCache.Routes[idx], 1e18, nil, true)
	// 					break
	// 				}
	// 			}
	// 		}
	// 		// fmt.Printf("Scores: %v", b.routeCache.Scores)
	// 		fmt.Println("\tSetPoolsInfo: \t\tPASS")
	// 	})

	// 	t.Run("bench", func(t *testing.T) {
	// 		assert := assert.New(t)
	// 		for i := 0; i < 1; i++ {
	// 			start := time.Now()
	// 			updates := []PoolUpdate{
	// 				PoolUpdate{
	// 					Addr: SpookyWftmBtc,
	// 					Reserves: map[common.Address]*big.Int{
	// 						common.HexToAddress("0x21be370D5312f44cB42ce377BC9b8a0cEF1A4C83"): StringToBigInt("10772184364004447628497336"),
	// 						common.HexToAddress("0x321162Cd933E2Be498Cd2267a90534A804051b11"): StringToBigInt("11600000000"),
	// 					},
	// 				},
	// 				PoolUpdate{
	// 					Addr: SpookyWftmUsdc,
	// 					Reserves: map[common.Address]*big.Int{
	// 						common.HexToAddress("0x21be370D5312f44cB42ce377BC9b8a0cEF1A4C83"): StringToBigInt("60565680072230427474078345"),
	// 						common.HexToAddress("0x04068da6c83afcfa0e13ba15a6696662335d5b75"): StringToBigInt("19300000000000"),
	// 					},
	// 				},
	// 			}
	// 			poolsInfoOverride, updatedKeys := b.makeUpdates(updates)
	// 			fmt.Printf("Made updates after %s, keys %d\n", utils.PrettyDuration(time.Now().Sub(start)), len(updatedKeys))
	// 			var pop Population
	// 			candidateRoutes := 0
	// 			maxScoreTier := len(ScoreTiers)
	// 			for _, key := range updatedKeys {
	// 				stepStart := time.Now()
	// 				var keyPop Population
	// 				keyPop, maxScoreTier = b.getProfitableRoutes(key, poolsInfoOverride, time.Duration(0), maxScoreTier)
	// 				pop = append(pop, keyPop...)
	// 				fmt.Printf("getProfitableRoutes completed after %s %s, returned %d/%d, maxScoreTier %d\n",
	// 					utils.PrettyDuration(time.Now().Sub(stepStart)),
	// 					utils.PrettyDuration(time.Now().Sub(start)),
	// 					len(keyPop),
	// 					len(b.routeCache.PoolToRouteIdxs[key][0]),
	// 					maxScoreTier,
	// 				)
	// 				// pop[:10].Print()
	// 				candidateRoutes += len(b.routeCache.PoolToRouteIdxs[key][0])
	// 			}
	// 			fmt.Printf("Computed candidate routes after %s, len %d/%d\n", utils.PrettyDuration(time.Now().Sub(start)), len(pop), candidateRoutes)
	// 			plan := b.getMostProfitablePath(pop, poolsInfoOverride, FloatToBigInt(1e10))
	// 			assert.NotNil(t, plan)
	// 			fmt.Printf("Made plan after %s, route %d, profit %f\n\n", utils.PrettyDuration(time.Now().Sub(start)),
	// 				plan.RouteIdx, BigIntToFloat(plan.NetProfit)/1e18)
	// 			assert.Greater(BigIntToFloat(plan.NetProfit)/1e18, 49000.0)
	// 		}
	// 	})

	// 	t.Run("bench 2", func(t *testing.T) {
	// 		for i := 0; i < 1; i++ {
	// 			start := time.Now()
	// 			updates := []PoolUpdate{
	// 				PoolUpdate{
	// 					Addr: SpookyWftmUsdc,
	// 					Reserves: map[common.Address]*big.Int{
	// 						common.HexToAddress("0x21be370D5312f44cB42ce377BC9b8a0cEF1A4C83"): StringToBigInt("60565680072230427474078345"),
	// 						common.HexToAddress("0x04068da6c83afcfa0e13ba15a6696662335d5b75"): StringToBigInt("19300000000000"),
	// 					},
	// 				},
	// 			}
	// 			poolsInfoOverride, updatedKeys := b.makeUpdates(updates)
	// 			var pop Population
	// 			maxScoreTier := len(ScoreTiers)
	// 			for _, key := range updatedKeys {
	// 				// stepStart := time.Now()
	// 				var keyPop Population
	// 				keyPop, maxScoreTier = b.getProfitableRoutes(key, poolsInfoOverride, time.Duration(0), maxScoreTier)
	// 				pop = append(pop, keyPop...)
	// 			}
	// 			for i, c := range pop {
	// 				if c.ContinuousGene < 0 {
	// 					fmt.Printf("Negative continuous gene, %d, %f", i, c.ContinuousGene)
	// 				}
	// 			}
	// 			plan := b.getMostProfitablePath(pop, poolsInfoOverride, FloatToBigInt(1e10))
	// 			fmt.Printf("Made plan after %s, route %d, profit %f\n", utils.PrettyDuration(time.Now().Sub(start)),
	// 				plan.RouteIdx, BigIntToFloat(plan.NetProfit)/1e18)
	// 			assert := assert.New(t)
	// 			assert.Greater(BigIntToFloat(plan.NetProfit)/1e18, 49000.0)
	// 		}
	// 	})

	t.Run("invariant", func(t *testing.T) {
		assert := assert.New(t)
		amp := 3250e18
		balances := map[common.Address]float64{
			common.HexToAddress("0x1"): 1331442144276000000000000.,
			common.HexToAddress("0x2"): 1682595164225370133548061,
		}
		invariant := calcStableInvariant(amp, balances)
		var expectedInvariant float64 = 3014037308501370133548055
		// fmt.Printf("Invariant: %v\n", invariant)
		if assert.InDelta(expectedInvariant, invariant, 1e9) {
			fmt.Println("\tInvariant: \t\tPASS")
		}
	})

	// t.Run("getTokenBalanceGivenInvAndBalances", func(t *testing.T) {
	// 	assert := assert.New(t)
	// 	amp := 3250000.
	// 	balances := map[common.Address]float64{
	// 		common.HexToAddress("0x1"): 14692118844975178000000000000.,
	// 		common.HexToAddress("0x2"): 1682595164225370133548061.,
	// 	}
	// 	invariant := calcStableInvariant(amp, balances)
	// 	getTokBalances := getTokenBalanceGivenInvAndBalances(amp, invariant, balances, balances[common.HexToAddress("0x1")], common.HexToAddress("0x1"), common.HexToAddress("0x2"))
	// 	var expectedOut float64 = 4879695783742
	// 	// fmt.Printf("tokBalancesOut: %v\n", getTokBalances)
	// 	if assert.InDelta(expectedOut, getTokBalances, 1) {
	// 		fmt.Println("\tgetTokenBalancesOut: \tPASS")
	// 	}
	// })

	// 	t.Run("getAmountOutBalancerStable", func(t *testing.T) {
	// 		assert := assert.New(t)
	// 		var amountIn float64 = 14690787402830902000000000000
	// 		amp := 3250e18
	// 		balances := map[common.Address]float64{
	// 			common.HexToAddress("0x1"): 1331442144276000000000000.,
	// 			common.HexToAddress("0x2"): 1682595164225370133548061,
	// 		}
	// 		amountOut := getAmountOutBalancerStable(
	// 			amountIn, 0, amp, balances, common.HexToAddress("0x1"), common.HexToAddress("0x2"), 1, 1)
	// 		var expectedOut float64 = 1682595164220490437764318
	// 		// fmt.Printf("amountOut: %v\n", amountOut)
	// 		if assert.InDelta(expectedOut, amountOut, 1e9) {
	// 			fmt.Println("\tamountOut: \t\tPASS")
	// 		}
	// 	})

	// 	t.Run("getAmountOutBalancerStable real", func(t *testing.T) {
	// 		assert := assert.New(t)
	// 		var amountIn float64 = 7253813376921310855168.000
	// 		amp := 200000.
	// 		balances := map[common.Address]float64{
	// 			common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75"): 679856808712999949828096.000,
	// 			common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E"): 721111464066631640023040.000,
	// 			common.HexToAddress("0xfB98B335551a418cD0737375a2ea0ded62Ea213b"): 1534579666976792763170816.000,
	// 		}
	// 		amountOut := getAmountOutBalancerStable(
	// 			amountIn, 0, amp, balances, common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E"), common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75"), 1, math.Pow(10, 12))
	// 		var expectedOut float64 = -158380826769.649
	// 		// fmt.Printf("amountOut: %v\n", amountOut)
	// 		if assert.InDelta(expectedOut, amountOut, 1) {
	// 			fmt.Println("\tamountOut: \t\tPASS")
	// 		}
	// 	})

	t.Run("getAmountOut 0xc655e79b73a2aeadef432e5c041bd3b5208e046e60e77965904164ac2969194f", func(t *testing.T) {
		assert := assert.New(t)
		var amountIn float64 = 11567409553171964409.000
		// var amountIn float64 = 11566252812216647212.000
		amp := 3250000.
		fee := 0.0001
		balances := map[common.Address]float64{
			common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75"): 1532050663405000000000000.000,
			common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E"): 1773019145975880925445320.000,
		}
		tokenIn := common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E")
		tokenOut := common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75")
		amountOut := getAmountOutBalancerStable(
			amountIn, fee, amp, balances, tokenIn, tokenOut, 1, math.Pow(10, 12))
		var expectedOut float64 = 11565728.459265565
		// fmt.Printf("amountOut: %v\n", amountOut)
		if assert.InDelta(expectedOut, amountOut, 1e-3) {
			fmt.Println("\tamountOut: \t\tPASS")
		}
	})

	// 	t.Run("getAmountOut", func(t *testing.T) {
	// 		assert := assert.New(t)
	// 		oneFtm := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	// 		fromToken := common.HexToAddress("0x100")
	// 		toToken := common.HexToAddress("0x200")
	// 		pool := &PoolInfo{
	// 			Reserves: map[common.Address]*big.Int{fromToken: new(big.Int).Mul(big.NewInt(1000),
	// 				oneFtm), toToken: new(big.Int).Mul(big.NewInt(2000), oneFtm)},
	// 			Tokens: []common.Address{common.HexToAddress("0x100"), common.HexToAddress("0x200")},
	// 			Weights: map[common.Address]*big.Int{fromToken: new(big.Int).Div(oneFtm, big.NewInt(2)),
	// 				toToken: new(big.Int).Div(oneFtm, big.NewInt(2))},
	// 			Fee: new(big.Int).Mul(new(big.Int).Exp(big.NewInt(10), big.NewInt(15), nil), big.NewInt(3)),
	// 		}
	// 		amountOut := b.getAmountOutBalancer(
	// 			new(big.Int).Exp(big.NewInt(10), big.NewInt(10), nil),
	// 			pool.Reserves[fromToken], pool.Reserves[toToken], pool.Weights[fromToken], pool.Weights[toToken],
	// 			pool.Fee)
	// 		// fmt.Printf("AmountOut: %s\n", amountOut)
	// 		assert.Equal(amountOut, big.NewInt(19940049611), "Calculated amountOut incorrectly")
	// 	})

	// 	t.Run("getRouteAmountOut", func(t *testing.T) {
	// 		assert := assert.New(t)
	// 		oneFtm := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	// 		amountIn := new(big.Int).Mul(oneFtm, big.NewInt(1))
	// 		token0 := common.HexToAddress("0x100")
	// 		token1 := common.HexToAddress("0x200")
	// 		poolsInfo := make(map[common.Address]*PoolInfo)
	// 		poolsInfo[common.HexToAddress("0x121")] = &PoolInfo{
	// 			Reserves: map[common.Address]*big.Int{
	// 				token0: StringToBigInt("4192289608298907098823614"),
	// 				token1: StringToBigInt("5105962348245921629330012")},
	// 			Tokens: []common.Address{token0, token1},
	// 			Weights: map[common.Address]*big.Int{token0: FloatToBigInt(5 * math.Pow(10, 17)),
	// 				token1: FloatToBigInt(5 * math.Pow(10, 17))},
	// 			FeeNumerator: StringToBigInt("998000"),
	// 		}
	// 		poolsInfo[common.HexToAddress("0x211")] = &PoolInfo{
	// 			Reserves: map[common.Address]*big.Int{
	// 				token1: StringToBigInt("273463168969934367747605"),
	// 				token0: StringToBigInt("225978221712898512549896")},
	// 			Tokens: []common.Address{token0, token1},
	// 			Weights: map[common.Address]*big.Int{token0: FloatToBigInt(5 * math.Pow(10, 17)),
	// 				token1: FloatToBigInt(5 * math.Pow(10, 17))},
	// 			Fee: StringToBigInt("1500000000000000"), // 0.15%
	// 		}
	// 		var poolId0 BalPoolId
	// 		copy(poolId0[:], common.FromHex("0x121"))
	// 		var poolId1 BalPoolId
	// 		copy(poolId1[:], common.FromHex("0x211"))
	// 		route := []*Leg{
	// 			&Leg{
	// 				From:     common.HexToAddress("0x100"),
	// 				To:       common.HexToAddress("0x200"),
	// 				PoolAddr: common.HexToAddress("0x121"),
	// 				PoolId:   poolId0,
	// 				Type:     UniswapV2Pair,
	// 			},
	// 			&Leg{
	// 				From:     common.HexToAddress("0x200"),
	// 				To:       common.HexToAddress("0x100"),
	// 				PoolAddr: common.HexToAddress("0x211"),
	// 				PoolId:   poolId1,
	// 				Type:     BalancerWeightedPool,
	// 			},
	// 		}
	// 		amountOut := b.getRouteAmountOutBalancer(route, amountIn, poolsInfo)
	// 		expectedAmountOut, _ := new(big.Int).SetString("1002930076753535435", 10)
	// 		assert.Greater(BigIntToFloat(expectedAmountOut), BigIntToFloat(amountOut)*0.999, "Calculated amountOut incorrectly")
	// 		assert.Less(BigIntToFloat(expectedAmountOut), BigIntToFloat(amountOut)*1.001, "Calculated amountOut incorrectly")
	// 	})

	// 		t.Run("getRouteAmountOut", func(t *testing.T) {
	// 			assert := assert.New(t)
	// 			oneFtm := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	// 			amountIn := new(big.Int).Mul(oneFtm, big.NewInt(1))
	// 			token0 := common.HexToAddress("0x100")
	// 			token1 := common.HexToAddress("0x200")
	// 			token2 := common.HexToAddress("0x300")
	// 			poolsInfo := make(map[common.Address]*PoolInfo)
	// 			poolsInfo[common.HexToAddress("0x121")] = &PoolInfo{
	// 				Reserves: map[common.Address]*big.Int{token0: FloatToBigInt(10 * math.Pow(10, 18)),
	// 					token1: FloatToBigInt(22 * math.Pow(10, 18))},
	// 				Tokens: []common.Address{token0, token1},
	// 				Weights: map[common.Address]*big.Int{token0: FloatToBigInt(5 * math.Pow(10, 17)),
	// 					token1: FloatToBigInt(5 * math.Pow(10, 17))},
	// 				FeeNumerator: StringToBigInt("997000"),
	// 			}
	// 			poolsInfo[common.HexToAddress("0x231")] = &PoolInfo{
	// 				Reserves: map[common.Address]*big.Int{token1: FloatToBigInt(10 * math.Pow(10, 18)),
	// 					token2: FloatToBigInt(22 * math.Pow(10, 12))},
	// 				Tokens: []common.Address{token0, token1},
	// 				Weights: map[common.Address]*big.Int{token1: FloatToBigInt(5 * math.Pow(10, 17)),
	// 					token2: FloatToBigInt(5 * math.Pow(10, 17))},
	// 				Fee: new(big.Int).Mul(new(big.Int).Exp(big.NewInt(10), big.NewInt(15), nil), big.NewInt(997)),
	// 			}
	// 			poolsInfo[common.HexToAddress("0x311")] = &PoolInfo{
	// 				Reserves: map[common.Address]*big.Int{token2: FloatToBigInt(300 * math.Pow(10, 12)),
	// 					token0: FloatToBigInt(100 * math.Pow(10, 18))},
	// 				Tokens: []common.Address{token0, token1},
	// 				Weights: map[common.Address]*big.Int{token2: FloatToBigInt(5 * math.Pow(10, 17)),
	// 					token0: FloatToBigInt(5 * math.Pow(10, 17))},
	// 				Fee: new(big.Int).Mul(new(big.Int).Exp(big.NewInt(10), big.NewInt(15), nil), big.NewInt(997)),
	// 			}
	// 			var poolId0 BalPoolId
	// 			copy(poolId0[:], common.FromHex("0x121"))
	// 			var poolId1 BalPoolId
	// 			copy(poolId1[:], common.FromHex("0x231"))
	// 			var poolId2 BalPoolId
	// 			copy(poolId2[:], common.FromHex("0x311"))
	// 			route := []*Leg{
	// 				&Leg{
	// 					From:     common.HexToAddress("0x100"),
	// 					To:       common.HexToAddress("0x200"),
	// 					PoolAddr: common.HexToAddress("0x121"),
	// 					PoolId:   poolId0,
	// 					Type:     UniswapV2Pair,
	// 				},
	// 				&Leg{
	// 					From:     common.HexToAddress("0x200"),
	// 					To:       common.HexToAddress("0x300"),
	// 					PoolAddr: common.HexToAddress("0x231"),
	// 					PoolId:   poolId1,
	// 					Type:     BalancerWeightedPool,
	// 				},
	// 				&Leg{
	// 					From:     common.HexToAddress("0x300"),
	// 					To:       common.HexToAddress("0x100"),
	// 					PoolAddr: common.HexToAddress("0x311"),
	// 					PoolId:   poolId2,
	// 					Type:     BalancerWeightedPool,
	// 				},
	// 			}
	// 			amountOut := b.getRouteAmountOutBalancer(route, amountIn, poolsInfo)
	// 			expectedAmountOut, _ := new(big.Int).SetString("1198210535726419815", 10)
	// 			assert.Equal(amountOut, expectedAmountOut, "Calculated amountOut incorrectly")
	// 		})

	// 	t.Run("spotPriceAfterSwap", func(t *testing.T) {
	// 		assert := assert.New(t)
	// 		oneFtm := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	// 		amountIn := new(big.Int).Div(oneFtm, big.NewInt(10))
	// 		token0 := common.HexToAddress("0x100")
	// 		token1 := common.HexToAddress("0x200")
	// 		poolsInfo := &PoolInfo{
	// 			Reserves: map[common.Address]*big.Int{token0: FloatToBigInt(10 * math.Pow(10, 18)),
	// 				token1: FloatToBigInt(22 * math.Pow(10, 18))},
	// 			Tokens: []common.Address{token0, token1},
	// 			Weights: map[common.Address]*big.Int{token0: FloatToBigInt(5 * math.Pow(10, 17)),
	// 				token1: FloatToBigInt(5 * math.Pow(10, 17))},
	// 			Fee: FloatToBigInt(0.003 * math.Pow(10, 18)),
	// 		}
	// 		spotPriceAfterSwap := b.spotPriceAfterSwapExactTokenInForTokenOut(amountIn, poolsInfo, token0, token1)
	// 		expectedAmountOut, _ := new(big.Int).SetString("465049421400565312", 10)
	// 		assert.Equal(spotPriceAfterSwap, expectedAmountOut, "Calculated amountOut incorrectly")
	// 	})

	// 	t.Run("derivativeSpotPriceAfterSwap", func(t *testing.T) {
	// 		assert := assert.New(t)
	// 		oneFtm := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	// 		amountIn := new(big.Int).Div(oneFtm, big.NewInt(1))
	// 		token0 := common.HexToAddress("0x100")
	// 		token1 := common.HexToAddress("0x200")
	// 		poolsInfo := &PoolInfo{
	// 			Reserves: map[common.Address]*big.Int{token0: FloatToBigInt(10 * math.Pow(10, 18)),
	// 				token1: FloatToBigInt(20 * math.Pow(10, 18))},
	// 			Tokens: []common.Address{token0, token1},
	// 			Weights: map[common.Address]*big.Int{token0: FloatToBigInt(5 * math.Pow(10, 17)),
	// 				token1: FloatToBigInt(5 * math.Pow(10, 17))},
	// 			Fee: FloatToBigInt(0.003 * math.Pow(10, 18)),
	// 		}
	// 		derivative := b.derivativeSpotPriceAfterSwapExactTokenInForTokenOut(amountIn, poolsInfo, token0, token1)
	// 		expectedDerivative, _ := new(big.Int).SetString("9970000000000000", 10)
	// 		assert.Equal(derivative, expectedDerivative, "Calculated derivative incorrectly")
	// 	})

}
