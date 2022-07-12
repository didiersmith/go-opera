package dexter

import (
	"fmt"
	"math"
	"testing"

	"github.com/Fantom-foundation/go-opera/logger"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

func TestCurveAmountOut(t *testing.T) {
	fmt.Println("Testing getAmountOut curve pools")
	logger.SetTestMode(t)
	logger.SetLevel("debug")

	root := "/home/ubuntu/dexter/carb/"
	s := NewBalancerLinearStrategy(
		"Balancer Linear", 0, nil, BalancerLinearStrategyConfig{
			RoutesFileName:          root + "route_caches/balancer_routes_len2.json",
			PoolToRouteIdxsFileName: root + "route_caches/balancer_poolToRouteIdxs_len2.json",
		})
	b := s.(*BalancerLinearStrategy)

	t.Run("Intialize", func(t *testing.T) {
		assert := assert.New(t)
		assert.Equal(b.Name, "Balancer Linear", "Wrong name")
		assert.Equal(b.ID, 0, "Wrong ID")
		assert.Equal(b.cfg.RoutesFileName, root+"route_caches/balancer_routes_len2.json", "Wrong ID")
		assert.Equal(b.cfg.PoolToRouteIdxsFileName, root+"route_caches/balancer_poolToRouteIdxs_len2.json", "Wrong ID")
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
			fmt.Println("\tInvariant: \t\t\t\tPASS")
		}
	})

	t.Run("getAmountOut 2pool", func(t *testing.T) {
		assert := assert.New(t)
		var amountIn float64 = 1e5
		amp := 450000.
		fee := 0.0004
		balances := map[common.Address]float64{
			common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75"): 1.9925923435242e25,
			common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E"): 21281812932357733710418873.000,
		}
		tokenIn := common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75")
		tokenOut := common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E")
		amountOut := getAmountOutBalancerStable(
			amountIn, fee, amp, balances, tokenIn, tokenOut, math.Pow(10, 12), 1)
		var expectedOut float64 = 99974618210288880.
		// fmt.Printf("amountOut: %v\n", amountOut)
		if assert.InDelta(expectedOut, amountOut, 1e10) {
			fmt.Println("\tamountOut - 2pool: \t\t\tPASS")
		}
	})

	t.Run("getAmountOut 2pool", func(t *testing.T) {
		assert := assert.New(t)
		var amountIn float64 = 1e5
		amp := 450000.
		fee := 0.0004
		balances := map[common.Address]float64{
			common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75"): 18310134618553000000000000.0,
			common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E"): 23246307080429008537424351.0,
		}
		tokenIn := common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75")
		tokenOut := common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E")
		// amountOut := getAmountOutBalancerStable(amountIn, fee, amp, balances, tokenIn, tokenOut, 1e12, 1)
		amountOut := getAmountOutCurve(amountIn, fee, amp, balances, tokenIn, tokenOut, 1e12, 1)
		var expectedOut float64 = 100014180546366086.
		// fmt.Printf("amountOut: %v\n", amountOut)
		if assert.InDelta(expectedOut, amountOut, 1e10) {
			fmt.Println("\tamountOut - 2pool: \t\t\tPASS")
		}
	})

	t.Run("getAmountOut WBTC basepool", func(t *testing.T) {
		assert := assert.New(t)
		var amountIn float64 = 1e8
		amp := 2e5
		fee := 0.0004
		balances := map[common.Address]float64{
			common.HexToAddress("0x321162Cd933E2Be498Cd2267a90534A804051b11"): 190453830470000000000.0,
			common.HexToAddress("0xDBf31dF14B66535aF65AaC99C32e9eA844e14501"): 95109302880000000000.0,
		}
		tokenIn := common.HexToAddress("0xDBf31dF14B66535aF65AaC99C32e9eA844e14501")
		tokenOut := common.HexToAddress("0x321162Cd933E2Be498Cd2267a90534A804051b11")
		// amountOut := getAmountOutBalancerStable(amountIn, fee, amp, balances, tokenIn, tokenOut, 1e10, 1e10)
		amountOut := getAmountOutCurve(amountIn, fee, amp, balances, tokenIn, tokenOut, 1e10, 1e10)
		var expectedOut float64 = 100374013.
		// fmt.Printf("amountOut: %v\n", amountOut)
		if assert.InDelta(expectedOut, amountOut, 1) {
			fmt.Println("\tamountOut - WBTC pool: \t\t\tPASS")
		}
	})

	t.Run("getAmountOut 2pool plain pool", func(t *testing.T) {
		assert := assert.New(t)
		var amountIn float64 = 1e6
		amp := 2e5
		fee := 0.0004
		balances := map[common.Address]float64{
			common.HexToAddress("0x1B6382DBDEa11d97f24495C9A90b7c88469134a4"): 1053491923029000000000000.0,
			common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75"): 948974993193000000000000.0,
		}
		tokenIn := common.HexToAddress("0x1B6382DBDEa11d97f24495C9A90b7c88469134a4")
		tokenOut := common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75")
		// amountOut := getAmountOutBalancerStable(amountIn, fee, amp, balances, tokenIn, tokenOut, 1e12, 1e12)
		amountOut := getAmountOutCurve(amountIn, fee, amp, balances, tokenIn, tokenOut, 1e12, 1e12)
		var expectedOut float64 = 999078.
		// fmt.Printf("amountOut: %v\n", amountOut)
		if assert.InDelta(expectedOut, amountOut, 1) {
			fmt.Println("\tamountOut - 2PoolV1 factory: \t\tPASS")
		}
	})

	t.Run("getAmountOut 2pool plain pool", func(t *testing.T) {
		assert := assert.New(t)
		var amountIn float64 = 1e18
		amp := 2e5
		fee := 0.0004
		balances := map[common.Address]float64{
			common.HexToAddress("0x21be370D5312f44cB42ce377BC9b8a0cEF1A4C83"): 10437120724510645756741.0,
			common.HexToAddress("0xB42bF10ab9Df82f9a47B86dd76EEE4bA848d0Fa2"): 9099449642774092018305.0,
		}
		tokenIn := common.HexToAddress("0x21be370D5312f44cB42ce377BC9b8a0cEF1A4C83")
		tokenOut := common.HexToAddress("0xB42bF10ab9Df82f9a47B86dd76EEE4bA848d0Fa2")
		// amountOut := getAmountOutBalancerStable(amountIn, fee, amp, balances, tokenIn, tokenOut, 1, 1)
		amountOut := getAmountOutCurve(amountIn, fee, amp, balances, tokenIn, tokenOut, 1, 1)
		var expectedOut float64 = 998912316920136470.
		// fmt.Printf("amountOut: %v\n", amountOut)
		if assert.InDelta(expectedOut, amountOut, 1e7) { // Possibly a 1 d.p too slack
			fmt.Println("\tamountOut - 2PoolV2 factory: \t\tPASS")
		}
	})

	t.Run("getAmountOut 3poolV1 plain pool", func(t *testing.T) {
		assert := assert.New(t)
		var amountIn float64 = 1e18
		amp := 2e5
		fee := 0.0004
		balances := map[common.Address]float64{
			common.HexToAddress("0x82f0B8B456c1A451378467398982d4834b6829c1"): 353772044521512743163.0,
			common.HexToAddress("0xdc301622e621166BD8E82f2cA0A26c13Ad0BE355"): 480167879404872448572.0,
			common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E"): 288661772164546200165.0,
		}
		tokenIn := common.HexToAddress("0xdc301622e621166BD8E82f2cA0A26c13Ad0BE355")
		tokenOut := common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E")
		// amountOut := getAmountOutBalancerStable(amountIn, fee, amp, balances, tokenIn, tokenOut, 1, 1)
		amountOut := getAmountOutCurve(amountIn, fee, amp, balances, tokenIn, tokenOut, 1, 1)
		var expectedOut float64 = 996840911882084409.
		// fmt.Printf("amountOut: %v\n", amountOut)
		if assert.InDelta(expectedOut, amountOut, 1e6) { // Possibly 1 d.p too slack
			fmt.Println("\tamountOut - 3PoolV1 factory: \t\tPASS")
		}
	})

	t.Run("getAmountOut 3poolV2 plain pool", func(t *testing.T) {
		assert := assert.New(t)
		var amountIn float64 = 1e6
		amp := 2e6
		fee := 0.0004
		balances := map[common.Address]float64{
			common.HexToAddress("0x82f0B8B456c1A451378467398982d4834b6829c1"): 7463449269837196544932328.0,
			common.HexToAddress("0x049d68029688eAbF473097a2fC38ef61633A3C7A"): 3499631928779000000000000.0,
			common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75"): 1414378062849000000000000.0,
		}
		tokenIn := common.HexToAddress("0x049d68029688eAbF473097a2fC38ef61633A3C7A")
		tokenOut := common.HexToAddress("0x82f0B8B456c1A451378467398982d4834b6829c1")
		// amountOut := getAmountOutBalancerStable(amountIn, fee, amp, balances, tokenIn, tokenOut, 1e12, 1)
		amountOut := getAmountOutCurve(amountIn, fee, amp, balances, tokenIn, tokenOut, 1e12, 1)
		var expectedOut float64 = 1000194254740624210.
		// fmt.Printf("amountOut: %v\n", amountOut)
		if assert.InDelta(expectedOut, amountOut, 1e10) { // Possibly 1 d.p too slack
			fmt.Println("\tamountOut - 3PoolV2 factory: \t\tPASS")
		}
	})

	t.Run("getAmountOut 4pool plain pool", func(t *testing.T) {
		assert := assert.New(t)
		var amountIn float64 = 1e18
		amp := 4e5
		fee := 0.0008
		balances := map[common.Address]float64{
			common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75"): 52837175273000000000000.0,
			common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E"): 58981114835162611459577.0,
			common.HexToAddress("0x9879aBDea01a879644185341F7aF7d8343556B7a"): 114786791779623332767319.0,
			common.HexToAddress("0xdc301622e621166BD8E82f2cA0A26c13Ad0BE355"): 113553386233087730210628.0,
		}
		tokenIn := common.HexToAddress("0xdc301622e621166BD8E82f2cA0A26c13Ad0BE355")
		tokenOut := common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E")
		// amountOut := getAmountOutBalancerStable(amountIn, fee, amp, balances, tokenIn, tokenOut, 1, 1)
		amountOut := getAmountOutCurve(amountIn, fee, amp, balances, tokenIn, tokenOut, 1, 1)
		var expectedOut float64 = 996983708531568571.
		// fmt.Printf("amountOut: %v\n", amountOut)
		if assert.InDelta(expectedOut, amountOut, 1e8) { // Possibly 1 d.p too slack
			fmt.Println("\tamountOut - 4PoolV1 factory: \t\tPASS")
		}
	})

	t.Run("getAmountOut FRAX2pool", func(t *testing.T) {
		assert := assert.New(t)
		metaAddr := common.HexToAddress("0x7a656b342e14f745e2b164890e88017e27ae7320")
		baseAddr := common.HexToAddress("0x27e611fd27b276acbd5ffd632e5eaebec9761e40")
		poolsInfoOverride := make(map[common.Address]*PoolInfoFloat)
		poolsInfoOverride[metaAddr] = &PoolInfoFloat{
			Tokens: []common.Address{
				common.HexToAddress("0xdc301622e621166bd8e82f2ca0a26c13ad0be355"),
				common.HexToAddress("0x27e611fd27b276acbd5ffd632e5eaebec9761e40"),
			},
			Fee:                4e-4,
			AmplificationParam: 2e5,
			UnderlyingReserves: map[common.Address]float64{
				common.HexToAddress("0xdc301622e621166bd8e82f2ca0a26c13ad0be355"): 41391354083316194165402004.0,
				common.HexToAddress("0x27e611fd27b276acbd5ffd632e5eaebec9761e40"): 26533451786919290911841471.0,
			},
			Reserves: map[common.Address]float64{
				common.HexToAddress("0xdc301622e621166bd8e82f2ca0a26c13ad0be355"): 41391354083316194165402004.0,
				common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75"): 11844303362082000000000000.0,
				common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E"): 15037372408487387024159281.0,
			},
		}
		poolsInfoOverride[baseAddr] = &PoolInfoFloat{
			Fee:                4e-4,
			AmplificationParam: 45e4,
			Tokens: []common.Address{
				common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E"),
				common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75"),
			},
			Reserves: map[common.Address]float64{
				common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75"): 18310134618553000000000000.0,
				common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E"): 23246307080429008537424351.0,
			},
			MetaTokenSupply: 41018100462201771561650475.,
		}
		scaleIn := 1e12
		scaleOut := 1.
		var amountIn float64 = 1e5
		tokenIn := common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75")
		tokenOut := common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E")
		amountOut := b.getAmountOutCurveMeta(amountIn, scaleIn, scaleOut, tokenIn, tokenOut, metaAddr, baseAddr, poolsInfoOverride)
		var expectedOut float64 = 100014180546366086.
		// fmt.Printf("amountOut: %v\n", amountOut)
		if assert.InDelta(expectedOut, amountOut, 1e10) {
			fmt.Println("\tamountOut - FRAX2pool (USDC->DAI): \tPASS")
		}
	})

	t.Run("getAmountOut FRAX2pool - 2", func(t *testing.T) {
		assert := assert.New(t)
		metaAddr := common.HexToAddress("0x7a656b342e14f745e2b164890e88017e27ae7320")
		baseAddr := common.HexToAddress("0x27e611fd27b276acbd5ffd632e5eaebec9761e40")
		poolsInfoOverride := make(map[common.Address]*PoolInfoFloat)
		poolsInfoOverride[metaAddr] = &PoolInfoFloat{
			Tokens: []common.Address{
				common.HexToAddress("0xdc301622e621166bd8e82f2ca0a26c13ad0be355"),
				common.HexToAddress("0x27e611fd27b276acbd5ffd632e5eaebec9761e40"),
			},
			Fee:                4e-4,
			AmplificationParam: 2e5,
			Reserves: map[common.Address]float64{
				common.HexToAddress("0xdc301622e621166bd8e82f2ca0a26c13ad0be355"): 41396779837353507386939604.0,
				common.HexToAddress("0x27e611fd27b276acbd5ffd632e5eaebec9761e40"): 26528109706264052682161839.0,
			},
			UnderlyingReserves: map[common.Address]float64{
				common.HexToAddress("0xdc301622e621166bd8e82f2ca0a26c13ad0be355"): 41396779837353507386939604.0,
				common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75"): 11651222754819000000000000.0,
				common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E"): 14877328406060889758137148.0,
			},
		}
		poolsInfoOverride[baseAddr] = &PoolInfoFloat{
			Fee:                4e-4,
			AmplificationParam: 45e4,
			Tokens: []common.Address{
				common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E"),
				common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75"),
			},
			Reserves: map[common.Address]float64{
				common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75"): 18205381619809000000000000.0,
				common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E"): 23246267521881335438806618.0,
			},
			MetaTokenSupply: 41450959355740333479476479.,
		}
		scaleIn := 1.
		scaleOut := 1.
		var amountIn float64 = 1e18
		tokenIn := common.HexToAddress("0xdc301622e621166bd8e82f2ca0a26c13ad0be355")
		tokenOut := common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E")
		amountOut := b.getAmountOutCurveMeta(amountIn, scaleIn, scaleOut, tokenIn, tokenOut, metaAddr, baseAddr, poolsInfoOverride)
		var expectedOut float64 = 997287797628906173.
		// fmt.Printf("amountOut: %v\n", amountOut)
		if assert.InDelta(expectedOut, amountOut, 1e10) {
			fmt.Println("\tamountOut - FRAX2pool (FRAX->DAI): \tPASS")
		}
	})

	t.Run("getAmountOut FRAX2pool - 3", func(t *testing.T) {
		assert := assert.New(t)
		metaAddr := common.HexToAddress("0x7a656b342e14f745e2b164890e88017e27ae7320")
		baseAddr := common.HexToAddress("0x27e611fd27b276acbd5ffd632e5eaebec9761e40")
		poolsInfoOverride := make(map[common.Address]*PoolInfoFloat)
		poolsInfoOverride[metaAddr] = &PoolInfoFloat{
			Tokens: []common.Address{
				common.HexToAddress("0xdc301622e621166bd8e82f2ca0a26c13ad0be355"),
				common.HexToAddress("0x27e611fd27b276acbd5ffd632e5eaebec9761e40"),
			},
			Fee:                4e-4,
			AmplificationParam: 2e5,
			Reserves: map[common.Address]float64{
				common.HexToAddress("0xdc301622e621166bd8e82f2ca0a26c13ad0be355"): 41396779837353507386939604.0,
				common.HexToAddress("0x27e611fd27b276acbd5ffd632e5eaebec9761e40"): 26528109706264052682161839.0,
			},
			UnderlyingReserves: map[common.Address]float64{
				common.HexToAddress("0xdc301622e621166bd8e82f2ca0a26c13ad0be355"): 41396779837353507386939604.0,
				common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75"): 11651222754819000000000000.0,
				common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E"): 14877328406060889758137148.0,
			},
		}
		poolsInfoOverride[baseAddr] = &PoolInfoFloat{
			Fee:                4e-4,
			AmplificationParam: 45e4,
			Tokens: []common.Address{
				common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E"),
				common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75"),
			},
			Reserves: map[common.Address]float64{
				common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75"): 18205381619809000000000000.0,
				common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E"): 23246267521881335438806618.0,
			},
			MetaTokenSupply: 41450959355740333479476479.,
		}
		scaleIn := 1e12
		scaleOut := 1.
		var amountIn float64 = 1e6
		tokenIn := common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75")
		tokenOut := common.HexToAddress("0xdc301622e621166bd8e82f2ca0a26c13ad0be355")
		amountOut := b.getAmountOutCurveMeta(amountIn, scaleIn, scaleOut, tokenIn, tokenOut, metaAddr, baseAddr, poolsInfoOverride)
		var expectedOut float64 = 1002097687116900269.
		// fmt.Printf("amountOut: %v\n", amountOut)
		if assert.InDelta(expectedOut, amountOut, 1e10) {
			fmt.Println("\tamountOut - FRAX2pool (USDC->FRAX): \tPASS")
		}
	})

	t.Run("getAmountOut FRAX2pool - 4", func(t *testing.T) {
		assert := assert.New(t)
		metaAddr := common.HexToAddress("0x7a656b342e14f745e2b164890e88017e27ae7320")
		baseAddr := common.HexToAddress("0x27e611fd27b276acbd5ffd632e5eaebec9761e40")
		poolsInfoOverride := make(map[common.Address]*PoolInfoFloat)
		poolsInfoOverride[metaAddr] = &PoolInfoFloat{
			Fee:                4e-4,
			AmplificationParam: 2e5,
			Tokens: []common.Address{
				common.HexToAddress("0xdc301622e621166bd8e82f2ca0a26c13ad0be355"),
				common.HexToAddress("0x27e611fd27b276acbd5ffd632e5eaebec9761e40"),
			},
			Reserves: map[common.Address]float64{
				common.HexToAddress("0xdc301622e621166bd8e82f2ca0a26c13ad0be355"): 41396779837353507386939604.0,
				common.HexToAddress("0x27e611fd27b276acbd5ffd632e5eaebec9761e40"): 26528109706264052682161839.0,
			},
			UnderlyingReserves: map[common.Address]float64{
				common.HexToAddress("0xdc301622e621166bd8e82f2ca0a26c13ad0be355"): 41396779837353507386939604.0,
				common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75"): 11651222754819000000000000.0,
				common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E"): 14877328406060889758137148.0,
			},
		}
		poolsInfoOverride[baseAddr] = &PoolInfoFloat{
			Fee:                4e-4,
			AmplificationParam: 45e4,
			Tokens: []common.Address{
				common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E"),
				common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75"),
			},
			Reserves: map[common.Address]float64{
				common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75"): 18205381619809000000000000.0,
				common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E"): 23246267521881335438806618.0,
			},
			MetaTokenSupply: 41450959355740333479476479.,
		}
		scaleIn := 1.
		scaleOut := 1.
		var amountIn float64 = 1e18
		tokenIn := common.HexToAddress("0xdc301622e621166bd8e82f2ca0a26c13ad0be355")
		tokenOut := common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75")
		amountOut := b.getAmountOutCurveMeta(amountIn, scaleIn, scaleOut, tokenIn, tokenOut, metaAddr, baseAddr, poolsInfoOverride)
		var expectedOut float64 = 996685000000000000.
		// fmt.Printf("amountOut: %v\n", amountOut)
		if assert.InDelta(expectedOut, amountOut, 1e12) {
			fmt.Println("\tamountOut - FRAX2pool (FRAX->USDC): \tPASS")
		}
	})

	t.Run("getAmountOut Dola2pool - real", func(t *testing.T) {
		assert := assert.New(t)
		metaAddr := common.HexToAddress("0x7a656b342e14f745e2b164890e88017e27ae7320")
		baseAddr := common.HexToAddress("0x27e611fd27b276acbd5ffd632e5eaebec9761e40")
		poolsInfoOverride := make(map[common.Address]*PoolInfoFloat)
		poolsInfoOverride[metaAddr] = &PoolInfoFloat{
			Fee:                4e-4,
			AmplificationParam: 1e5,
			Tokens: []common.Address{
				common.HexToAddress("0x3129662808bEC728a27Ab6a6b9AFd3cBacA8A43c"),
				common.HexToAddress("0x27e611fd27b276acbd5ffd632e5eaebec9761e40"),
			},
			Reserves: map[common.Address]float64{
				common.HexToAddress("0x3129662808bEC728a27Ab6a6b9AFd3cBacA8A43c"): 437887754357325037952575.0,
				common.HexToAddress("0x27e611fd27b276acbd5ffd632e5eaebec9761e40"): 360566983460605909192520.0,
			},
			UnderlyingReserves: map[common.Address]float64{
				common.HexToAddress("0x3129662808bEC728a27Ab6a6b9AFd3cBacA8A43c"): 437887754357325037952575.0,
				common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75"): 161397058812000000000000.0,
				common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E"): 199174361159550727368681.0,
			},
		}
		poolsInfoOverride[baseAddr] = &PoolInfoFloat{
			Fee:                4e-4,
			AmplificationParam: 45e4,
			Tokens: []common.Address{
				common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E"),
				common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75"),
			},
			Reserves: map[common.Address]float64{
				common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75"): 14321362274457000000000000.0,
				common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E"): 17673483042002634356152881.0,
			},
			MetaTokenSupply: 31994451648283811191765856.,
		}
		scaleIn := 1e12
		scaleOut := 1.
		var amountIn float64 = 2564441787.
		tokenIn := common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75")
		tokenOut := common.HexToAddress("0x3129662808bEC728a27Ab6a6b9AFd3cBacA8A43c")
		amountOut := b.getAmountOutCurveMeta(amountIn, scaleIn, scaleOut, tokenIn, tokenOut, metaAddr, baseAddr, poolsInfoOverride)
		var expectedOut float64 = 2568384124963231214266.
		// fmt.Printf("amountOut: %v\n", amountOut)
		if assert.InDelta(expectedOut, amountOut, 1e10) {
			fmt.Println("\tamountOut - Dola2pool (USDC->Dola): \tPASS")
		}
	})

	t.Run("getAmountOut FRAX2pool - real", func(t *testing.T) {
		assert := assert.New(t)
		metaAddr := common.HexToAddress("0x7a656b342e14f745e2b164890e88017e27ae7320")
		baseAddr := common.HexToAddress("0x27e611fd27b276acbd5ffd632e5eaebec9761e40")
		poolsInfoOverride := make(map[common.Address]*PoolInfoFloat)
		poolsInfoOverride[metaAddr] = &PoolInfoFloat{
			Fee:                4e-4,
			AmplificationParam: 2e5,
			Tokens: []common.Address{
				common.HexToAddress("0xdc301622e621166BD8E82f2cA0A26c13Ad0BE355"),
				common.HexToAddress("0x27e611fd27b276acbd5ffd632e5eaebec9761e40"),
			},
			Reserves: map[common.Address]float64{
				common.HexToAddress("0xdc301622e621166BD8E82f2cA0A26c13Ad0BE355"): 1.569079573847554e25,
				common.HexToAddress("0x27e611fd27b276acbd5ffd632e5eaebec9761e40"): 1.5100598984894187e25,
			},
			UnderlyingReserves: map[common.Address]float64{
				common.HexToAddress("0xdc301622e621166BD8E82f2cA0A26c13Ad0BE355"): 1.569079573847554e25,
				common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75"): 6803881052428000000000000,
				common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E"): 8296883192305614810216765,
			},
		}
		poolsInfoOverride[baseAddr] = &PoolInfoFloat{
			Fee:                4e-4,
			AmplificationParam: 45e4,
			Tokens: []common.Address{
				common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E"),
				common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75"),
			},
			Reserves: map[common.Address]float64{
				common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75"): 1.4382158959524e25,
				common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E"): 1.7538092159584697e25,
			},
			MetaTokenSupply: 31919901790061285563770648.,
		}
		scaleIn := 1e12
		scaleOut := 1.
		var amountIn float64 = 2537966262.
		tokenIn := common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75")
		tokenOut := common.HexToAddress("0xdc301622e621166BD8E82f2cA0A26c13Ad0BE355")
		amountOut := b.getAmountOutCurveMeta(amountIn, scaleIn, scaleOut, tokenIn, tokenOut, metaAddr, baseAddr, poolsInfoOverride)
		var expectedOut float64 = 2537520926880207836577.
		// fmt.Printf("amountOut: %v\n", amountOut)
		if assert.InDelta(expectedOut, amountOut, 1e11) {
			fmt.Println("\tamountOut - FRAX2pool (USDC->FRAX): \tPASS")
		}
	})

	t.Run("getAmountOut FRAX2pool - real 2", func(t *testing.T) {
		assert := assert.New(t)
		metaAddr := common.HexToAddress("0x7a656b342e14f745e2b164890e88017e27ae7320")
		baseAddr := common.HexToAddress("0x27e611fd27b276acbd5ffd632e5eaebec9761e40")
		poolsInfoOverride := make(map[common.Address]*PoolInfoFloat)
		poolsInfoOverride[metaAddr] = &PoolInfoFloat{
			Fee:                4e-4,
			AmplificationParam: 2e5,
			Tokens: []common.Address{
				common.HexToAddress("0xdc301622e621166BD8E82f2cA0A26c13Ad0BE355"),
				common.HexToAddress("0x27e611fd27b276acbd5ffd632e5eaebec9761e40"),
			},
			Reserves: map[common.Address]float64{
				common.HexToAddress("0xdc301622e621166BD8E82f2cA0A26c13Ad0BE355"): 15682959320533548536954880.,
				common.HexToAddress("0x27e611fd27b276acbd5ffd632e5eaebec9761e40"): 15108333484194851410935808,
			},
			UnderlyingReserves: map[common.Address]float64{
				common.HexToAddress("0xdc301622e621166BD8E82f2cA0A26c13Ad0BE355"): 15682959320533548536954880.,
				common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E"): 8294023591558666479985323.,
				common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75"): 6814472074925.,
			},
		}
		poolsInfoOverride[baseAddr] = &PoolInfoFloat{
			Fee:                4e-4,
			AmplificationParam: 45e4,
			Tokens: []common.Address{
				common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E"),
				common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75"),
			},
			Reserves: map[common.Address]float64{
				common.HexToAddress("0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E"): 17519210297027796999864320.,
				common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75"): 14393999248488000000000000.,
			},
			MetaTokenSupply: 31912866972867481066065137.,
		}
		scaleIn := 1.
		scaleOut := 1e12
		var amountIn float64 = 2603671150225096441856.
		tokenIn := common.HexToAddress("0xdc301622e621166BD8E82f2cA0A26c13Ad0BE355")
		tokenOut := common.HexToAddress("0x04068DA6C83AFCFA0e13ba15A6696662335D5B75")
		amountOut := b.getAmountOutCurveMeta(amountIn, scaleIn, scaleOut, tokenIn, tokenOut, metaAddr, baseAddr, poolsInfoOverride)
		var expectedOut float64 = 2600968158.
		// fmt.Printf("amountOut: %v\n", amountOut)
		if assert.InDelta(expectedOut, amountOut, 1) {
			fmt.Println("\tamountOut - FRAX2pool (FRAX->USDC): \tPASS")
		}
	})

}
