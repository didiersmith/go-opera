package dexter

import (
	"fmt"
	"testing"

	"github.com/Fantom-foundation/go-opera/logger"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

func TestBalancerLinearStrategy(t *testing.T) {
	fmt.Println("Testing strategy_balancer_linear.go")
	logger.SetTestMode(t)
	logger.SetLevel("debug")

	root := "/home/ubuntu/dexter/carb/"
	s := NewBalancerLinearStrategy(
		"Balancer Linear", 0, nil, BalancerLinearStrategyConfig{
			RoutesFileName:          root + "balancer_cache_routes_len2.json",
			PoolToRouteIdxsFileName: root + "balancer_cache_poolToRouteIdxs_len2.json",
		})
	b := s.(*BalancerLinearStrategy)

	t.Run("Intialize", func(t *testing.T) {
		assert := assert.New(t)
		assert.Equal(b.Name, "Balancer Linear", "Wrong name")
		assert.Equal(b.ID, 0, "Wrong ID")
		assert.Equal(b.cfg.RoutesFileName, root+"balancer_cache_routes_len2.json", "Wrong ID")
		assert.Equal(b.cfg.PoolToRouteIdxsFileName, root+"balancer_cache_poolToRouteIdxs_len2.json", "Wrong ID")
		assert.Equal(b.cfg.SelectSecondBest, false, "Wrong ID")
		assert.Equal(len(b.subStrategies), 0, "Wrong subStrategies")
	})

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

	t.Run("getTokenBalanceGivenInvAndBalances", func(t *testing.T) {
		assert := assert.New(t)
		amp := 3250e18
		balances := map[common.Address]float64{
			common.HexToAddress("0x1"): 14692118844975178000000000000.,
			common.HexToAddress("0x2"): 1682595164225370133548061.,
		}
		var invariant float64 = 3.01403730850137e24
		getTokBalances := getTokenBalanceGivenInvAndBalances(amp, invariant, balances, common.HexToAddress("0x2"))
		var expectedOut float64 = 4879695783742
		// fmt.Printf("tokBalancesOut: %v\n", getTokBalances)
		if assert.InDelta(expectedOut, getTokBalances, 1) {
			fmt.Println("\tgetTokenBalancesOut: \tPASS")
		}
	})

	t.Run("getAmountOutBalancerStable", func(t *testing.T) {
		assert := assert.New(t)
		var amountIn float64 = 14690787402830902000000000000
		amp := 3250e18
		balances := map[common.Address]float64{
			common.HexToAddress("0x1"): 1331442144276000000000000.,
			common.HexToAddress("0x2"): 1682595164225370133548061,
		}
		amountOut := getAmountOutBalancerStable(
			amountIn, 0, amp, balances, common.HexToAddress("0x1"), common.HexToAddress("0x2"))
		var expectedOut float64 = 1682595164220490437764318
		// fmt.Printf("amountOut: %v\n", amountOut)

		if assert.InDelta(expectedOut, amountOut, 1e9) {
			fmt.Println("\tamountOut: \t\tPASS")
		}
	})

	t.Run("loadJson", func(t *testing.T) {
		assert := assert.New(t)
		assert.Greater(len(b.interestedPools), 0, "No intereseted pools")
		assert.Greater(len(b.routeCache.Routes), 0, "No routes in routeCache")
		assert.Greater(len(b.routeCache.PoolToRouteIdxs), 0, "No poolToRouteIdxs in routeCache")
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
