package dexter

import (
	"fmt"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/Fantom-foundation/go-opera/logger"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

func TestOptimize(t *testing.T) {
	fmt.Println("Testing optimize.go")
	logger.SetTestMode(t)
	logger.SetLevel("debug")

	t.Run("root", func(t *testing.T) {
		assert := assert.New(t)
		pairInfo1 := &PairInfo{
			reserve0:     big.NewInt(200),
			reserve1:     big.NewInt(500),
			token0:       common.HexToAddress("0x100"),
			token1:       common.HexToAddress("0x200"),
			feeNumerator: big.NewInt(1000000),
		}
		pairInfo2 := &PairInfo{
			reserve0:     big.NewInt(100),
			reserve1:     big.NewInt(100),
			token0:       common.HexToAddress("0x100"),
			token1:       common.HexToAddress("0x200"),
			feeNumerator: big.NewInt(1000000),
		}
		root := findRoot(pairInfo1, pairInfo2, common.HexToAddress("0x100"))
		if assert.Equal(root, big.NewInt(116), "Calculated root incorrectly") {
			fmt.Println("\tfindRoot: \t\t\t\tPASS")
		}
	})

	t.Run("getAmountOut", func(t *testing.T) {
		assert := assert.New(t)
		amountIn := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
		reserveIn, _ := new(big.Int).SetString("241533338345271804344072", 10)
		reserveOut, _ := new(big.Int).SetString("950633023450013363421", 10)
		feeNumerator := big.NewInt(998000)
		amountOut := getAmountOut(amountIn, reserveIn, reserveOut, feeNumerator)
		expectedAmountOut, _ := new(big.Int).SetString("3927937417754580", 10)
		if assert.Equal(amountOut, expectedAmountOut, "Calculated amountOut incorrectly") {
			fmt.Println("\tgetAmountOut: \t\t\t\tPASS")
		}
	})

	t.Run("getAmountOutMulti", func(t *testing.T) {
		assert := assert.New(t)
		oneFtm := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
		amountIn := oneFtm
		pairInfo1 := &PairInfo{
			reserve0:     new(big.Int).Mul(big.NewInt(1000000), oneFtm),
			reserve1:     new(big.Int).Mul(big.NewInt(1000000), oneFtm),
			token0:       common.HexToAddress("0x100"),
			token1:       common.HexToAddress("0x200"),
			feeNumerator: big.NewInt(997000),
		}
		pairInfo2 := &PairInfo{
			reserve0:     new(big.Int).Mul(big.NewInt(1000000), oneFtm),
			reserve1:     new(big.Int).Mul(big.NewInt(1000000), oneFtm),
			token0:       common.HexToAddress("0x100"),
			token1:       common.HexToAddress("0x200"),
			feeNumerator: big.NewInt(997000),
		}
		var reserves [4]*big.Int
		reserves[0] = new(big.Int).Mul(big.NewInt(1000000), oneFtm)
		reserves[1] = new(big.Int).Mul(big.NewInt(1000000), oneFtm)
		reserves[2] = new(big.Int).Mul(big.NewInt(1000000), oneFtm)
		reserves[3] = new(big.Int).Mul(big.NewInt(1000000), oneFtm)
		root := findRoot(pairInfo1, pairInfo2, common.HexToAddress("0x100"))
		amountOut := getAmountOutMulti(amountIn, root, pairInfo1.feeNumerator, pairInfo2.feeNumerator, reserves)
		expectedAmountOut, _ := new(big.Int).SetString("996999502995747756", 10)
		if assert.Equal(amountOut, expectedAmountOut, "Calculated amountOut incorrectly") &&
			assert.Equal(root, big.NewInt(0), "Calculated root incorrectly") {
			fmt.Println("\tgetAmountOutMulti: \t\t\tPASS")
		}
	})

	t.Run("getAmountOutMulti - 2", func(t *testing.T) {
		assert := assert.New(t)
		pairInfo1 := &PairInfo{
			reserve0:     StringToBigInt("69117772438428324048527"),
			reserve1:     StringToBigInt("135934794043183906520226"),
			token0:       common.HexToAddress("0x100"),
			token1:       common.HexToAddress("0x200"),
			feeNumerator: big.NewInt(998500),
		}
		pairInfo2 := &PairInfo{
			reserve0:     StringToBigInt("489237421951321733789"),
			reserve1:     StringToBigInt("974789239638859556173"),
			token0:       common.HexToAddress("0x100"),
			token1:       common.HexToAddress("0x200"),
			feeNumerator: big.NewInt(998000),
		}
		var reserves [4]*big.Int
		reserves[0] = StringToBigInt("69117772438428324048527")
		reserves[1] = StringToBigInt("135934794043183906520226")
		reserves[2] = StringToBigInt("489237421951321733789")
		reserves[3] = StringToBigInt("974789239638859556173")
		amountIn, _ := new(big.Int).SetString("907320000000000000000", 10)
		root := findRoot(pairInfo1, pairInfo2, common.HexToAddress("0x100"))
		amountOut := getAmountOutMulti(amountIn, root, pairInfo1.feeNumerator, pairInfo2.feeNumerator, reserves)
		// fmt.Printf("amountOut: %s\n", amountOut)
		expectedAmountOut, _ := new(big.Int).SetString("1759061584975854799378", 10) // 1759061585646113259520
		expectedRoot, _ := new(big.Int).SetString("-448487159758946629794", 10)
		if assert.Equal(amountOut, expectedAmountOut, "Calculated amountOut incorrectly") &&
			assert.Equal(root, expectedRoot, "Calculated root incorrectly") {
			fmt.Println("\tgetAmountOutMulti - 2: \t\t\tPASS")
		}
	})

	t.Run("getAmountOutMultiSecondPrimes", func(t *testing.T) {
		assert := assert.New(t)
		amountIn := big.NewInt(1000)
		pairInfo1 := &PairInfo{
			reserve0:     big.NewInt(1000000),
			reserve1:     big.NewInt(1000000),
			token0:       common.HexToAddress("0x100"),
			token1:       common.HexToAddress("0x200"),
			feeNumerator: big.NewInt(1000000),
		}
		pairInfo2 := &PairInfo{
			reserve0:     big.NewInt(1000000),
			reserve1:     big.NewInt(1000000),
			token0:       common.HexToAddress("0x100"),
			token1:       common.HexToAddress("0x200"),
			feeNumerator: big.NewInt(1000000),
		}
		var reserves [4]*big.Int
		reserves[0] = big.NewInt(1000000)
		reserves[1] = big.NewInt(1000000)
		reserves[2] = big.NewInt(1000000)
		reserves[3] = big.NewInt(1000000)
		root := findRoot(pairInfo1, pairInfo2, common.HexToAddress("0x100"))
		amountOutMultiPrime := getAmountOutMultiPrime(
			amountIn, root, pairInfo1.feeNumerator, pairInfo2.feeNumerator, reserves)
		amountOutMultiSecondPrime := getAmountOutMultiSecondPrime(
			amountIn, root, pairInfo1.feeNumerator, pairInfo2.feeNumerator, reserves)
		// fmt.Printf("root: %s, Prime: %g, 2-Prime: %g\n",
		// 	root, amountOutMultiPrime, amountOutMultiSecondPrime)
		if assert.Equal(amountOutMultiSecondPrime, float64(-9.995000002496877e-07),
			"Calculated amountOutSecondPrime incorrectly") &&
			assert.Equal(amountOutMultiPrime, float64(0.9990007495003124),
				"Calculated amountOutPrime incorrectly") &&
			assert.Equal(root, big.NewInt(0), "Calculated root incorrectly") {
			fmt.Println("\tDerivatives: \t\t\t\tPASS")
		}
	})

	t.Run("getRouteAmountOut", func(t *testing.T) {
		assert := assert.New(t)
		oneFtm := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
		amountIn := new(big.Int).Mul(oneFtm, big.NewInt(1))
		d := &Dexter{
			pairsInfo: make(map[common.Address]*PairInfo),
		}
		d.pairsInfo[common.HexToAddress("0x121")] = &PairInfo{
			reserve0:     FloatToBigInt(10 * math.Pow(10, 18)),
			reserve1:     FloatToBigInt(22 * math.Pow(10, 18)),
			token0:       common.HexToAddress("0x100"),
			token1:       common.HexToAddress("0x200"),
			feeNumerator: StringToBigInt("997000"),
		}
		d.pairsInfo[common.HexToAddress("0x122")] = &PairInfo{
			reserve0:     FloatToBigInt(10 * math.Pow(10, 18)),
			reserve1:     FloatToBigInt(21 * math.Pow(10, 18)),
			token0:       common.HexToAddress("0x100"),
			token1:       common.HexToAddress("0x200"),
			feeNumerator: StringToBigInt("997000"),
		}
		d.pairsInfo[common.HexToAddress("0x231")] = &PairInfo{
			reserve0:     FloatToBigInt(10 * math.Pow(10, 18)),
			reserve1:     FloatToBigInt(22 * math.Pow(10, 12)),
			token0:       common.HexToAddress("0x200"),
			token1:       common.HexToAddress("0x300"),
			feeNumerator: StringToBigInt("997000"),
		}
		d.pairsInfo[common.HexToAddress("0x232")] = &PairInfo{
			reserve0:     FloatToBigInt(100 * math.Pow(10, 18)),
			reserve1:     FloatToBigInt(200 * math.Pow(10, 12)),
			token0:       common.HexToAddress("0x200"),
			token1:       common.HexToAddress("0x300"),
			feeNumerator: StringToBigInt("997000"),
		}
		d.pairsInfo[common.HexToAddress("0x311")] = &PairInfo{
			reserve0:     FloatToBigInt(300 * math.Pow(10, 12)),
			reserve1:     FloatToBigInt(100 * math.Pow(10, 18)),
			token0:       common.HexToAddress("0x300"),
			token1:       common.HexToAddress("0x100"),
			feeNumerator: StringToBigInt("997000"),
		}
		// d.pairsInfo[common.HexToAddress("0x312")] = &PairInfo{
		// 	reserve0:     FloatToBigInt(400 * math.Pow(10, 12)),
		// 	reserve1:     FloatToBigInt(100 * math.Pow(10, 18)),
		// 	token0:       common.HexToAddress("0x300"),
		// 	token1:       common.HexToAddress("0x100"),
		// 	feeNumerator: StringToBigInt("997000"),
		// }
		root1 := findRoot(d.pairsInfo[common.HexToAddress("0x121")],
			d.pairsInfo[common.HexToAddress("0x122")], common.HexToAddress("0x100"))
		root2 := findRoot(d.pairsInfo[common.HexToAddress("0x231")],
			d.pairsInfo[common.HexToAddress("0x232")], common.HexToAddress("0x200"))
		route := []*Leg{
			&Leg{
				From: common.HexToAddress("0x100"),
				To:   common.HexToAddress("0x200"),
				// PairAddrs: []common.Address{common.HexToAddress("0x121")},
				PairAddrs: []common.Address{common.HexToAddress("0x121"), common.HexToAddress("0x122")},
				Root:      root1,
			},
			&Leg{
				From:      common.HexToAddress("0x200"),
				To:        common.HexToAddress("0x300"),
				PairAddrs: []common.Address{common.HexToAddress("0x231"), common.HexToAddress("0x232")},
				Root:      root2,
			},
			&Leg{
				From:      common.HexToAddress("0x300"),
				To:        common.HexToAddress("0x100"),
				PairAddrs: []common.Address{common.HexToAddress("0x311")},
				// Root:      root3,
				Root: big.NewInt(0),
			},
		}
		amountOut := d.getRouteAmountOut(route, amountIn, map[common.Address]*PairInfo{})
		expectedAmountOut, _ := new(big.Int).SetString("1340015361762819683", 10)
		if assert.Equal(amountOut, expectedAmountOut, "Calculated amountOut incorrectly") {
			fmt.Println("\tgetRouteAmountOut: \t\t\tPASS")
		}
	})

	// 	t.Run("getRouteamountOuttestcases", func(t *testing.T) {
	// 		// assert := assert.New(t)
	// 		// oneFtm := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	// 		// amountIn := new(big.Int).Div(oneFtm, big.NewInt(1))
	// 		d := &Dexter{
	// 			pairsInfo: make(map[common.Address]*PairInfo),
	// 		}
	// 		d.pairsInfo[common.HexToAddress("0x121")] = &PairInfo{
	// 			reserve0:     FloatToBigInt(10 * math.Pow(10, 18)),
	// 			reserve1:     FloatToBigInt(22 * math.Pow(10, 18)),
	// 			token0:       common.HexToAddress("0x100"),
	// 			token1:       common.HexToAddress("0x200"),
	// 			feeNumerator: StringToBigInt("997000"),
	// 		}
	// 		// d.pairsInfo[common.HexToAddress("0x122")] = &PairInfo{
	// 		// 	reserve0:     FloatToBigInt(10 * math.Pow(10, 18)),
	// 		// 	reserve1:     FloatToBigInt(21 * math.Pow(10, 18)),
	// 		// 	token0:       common.HexToAddress("0x100"),
	// 		// 	token1:       common.HexToAddress("0x200"),
	// 		// 	feeNumerator: StringToBigInt("997000"),
	// 		// }
	// 		d.pairsInfo[common.HexToAddress("0x231")] = &PairInfo{
	// 			reserve0:     FloatToBigInt(10 * math.Pow(10, 18)),
	// 			reserve1:     FloatToBigInt(22 * math.Pow(10, 18)),
	// 			token0:       common.HexToAddress("0x200"),
	// 			token1:       common.HexToAddress("0x300"),
	// 			feeNumerator: StringToBigInt("997000"),
	// 		}
	// 		// d.pairsInfo[common.HexToAddress("0x232")] = &PairInfo{
	// 		// 	reserve0:     FloatToBigInt(100 * math.Pow(10, 18)),
	// 		// 	reserve1:     FloatToBigInt(200 * math.Pow(10, 12)),
	// 		// 	token0:       common.HexToAddress("0x200"),
	// 		// 	token1:       common.HexToAddress("0x300"),
	// 		// 	feeNumerator: StringToBigInt("997000"),
	// 		// }
	// 		d.pairsInfo[common.HexToAddress("0x311")] = &PairInfo{
	// 			reserve0:     FloatToBigInt(300 * math.Pow(10, 18)), //300 * 10**12
	// 			reserve1:     FloatToBigInt(100 * math.Pow(10, 18)),
	// 			token0:       common.HexToAddress("0x300"),
	// 			token1:       common.HexToAddress("0x100"),
	// 			feeNumerator: StringToBigInt("997000"),
	// 		}
	// 		// d.pairsInfo[common.HexToAddress("0x312")] = &PairInfo{
	// 		// 	reserve0:     FloatToBigInt(400 * math.Pow(10, 12)),
	// 		// 	reserve1:     FloatToBigInt(100 * math.Pow(10, 18)),
	// 		// 	token0:       common.HexToAddress("0x300"),
	// 		// 	token1:       common.HexToAddress("0x100"),
	// 		// 	feeNumerator: StringToBigInt("997000"),
	// 		// }
	// 		// root1 := findRoot(d.pairsInfo[common.HexToAddress("0x121")],
	// 		// 	d.pairsInfo[common.HexToAddress("0x122")], common.HexToAddress("0x100"))
	// 		// root2 := findRoot(d.pairsInfo[common.HexToAddress("0x231")],
	// 		// 	d.pairsInfo[common.HexToAddress("0x232")], common.HexToAddress("0x200"))
	// 		route := []*Leg{
	// 			&Leg{
	// 				From:      common.HexToAddress("0x100"),
	// 				To:        common.HexToAddress("0x200"),
	// 				PairAddrs: []common.Address{common.HexToAddress("0x121")},
	// 				// PairAddrs: []common.Address{common.HexToAddress("0x121"), common.HexToAddress("0x122")},
	// 				// Root:      root1,
	// 				Root: big.NewInt(0),
	// 			},
	// 			&Leg{
	// 				From: common.HexToAddress("0x200"),
	// 				To:   common.HexToAddress("0x300"),
	// 				// PairAddrs: []common.Address{common.HexToAddress("0x231"), common.HexToAddress("0x232")},
	// 				PairAddrs: []common.Address{common.HexToAddress("0x231")},
	// 				// Root:      root2,
	// 				Root: big.NewInt(0),
	// 			},
	// 			&Leg{
	// 				From:      common.HexToAddress("0x300"),
	// 				To:        common.HexToAddress("0x100"),
	// 				PairAddrs: []common.Address{common.HexToAddress("0x311")},
	// 				// Root:      root3,
	// 				Root: big.NewInt(0),
	// 			},
	// 		}
	// 		// converged, amountIn := d.calcOptimalAmountIn(amountIn, route, map[common.Address]*PairInfo{})
	// 		// fmt.Printf("converged: %t, amountIn: %s\n", converged, amountIn)
	// 		fmt.Println("Go")
	// 		amountInGo := StringToBigInt("790917180610885560")
	// 		amountOutGo := d.getRouteAmountOut(route, amountInGo, map[common.Address]*PairInfo{})
	// 		fmt.Println("JS")
	// 		amountInJS := StringToBigInt("3248573625119226087")
	// 		amountOutJS := d.getRouteAmountOut(route, amountInJS, map[common.Address]*PairInfo{})
	// 		fmt.Printf("Go: %s\n", amountOutGo)
	// 		fmt.Printf("JS: %s\n", amountOutJS)
	// 		// expectedAmountOut, _ := new(big.Int).SetString("1340015361762819683", 10)
	// 		// if assert.Equal(amountOut, expectedAmountOut, "Calculated amountOut incorrectly") {
	// 		// 	fmt.Println("\tgetAmountOutMulti: \t\t\tPASS")
	// 		// }
	// 	})

	t.Run("calcOptimalInSecant", func(t *testing.T) {
		assert := assert.New(t)
		oneFtm := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
		amountIn := new(big.Int).Mul(oneFtm, big.NewInt(1))
		d := &Dexter{
			pairsInfo: make(map[common.Address]*PairInfo),
		}
		d.pairsInfo[common.HexToAddress("0x121")] = &PairInfo{
			reserve0:     FloatToBigInt(10 * math.Pow(10, 18)),
			reserve1:     FloatToBigInt(22 * math.Pow(10, 18)),
			token0:       common.HexToAddress("0x100"),
			token1:       common.HexToAddress("0x200"),
			feeNumerator: StringToBigInt("997000"),
		}
		d.pairsInfo[common.HexToAddress("0x122")] = &PairInfo{
			reserve0:     FloatToBigInt(10 * math.Pow(10, 18)),
			reserve1:     FloatToBigInt(21 * math.Pow(10, 18)),
			token0:       common.HexToAddress("0x100"),
			token1:       common.HexToAddress("0x200"),
			feeNumerator: StringToBigInt("997000"),
		}
		d.pairsInfo[common.HexToAddress("0x231")] = &PairInfo{
			reserve0:     FloatToBigInt(10 * math.Pow(10, 18)),
			reserve1:     FloatToBigInt(22 * math.Pow(10, 12)),
			token0:       common.HexToAddress("0x200"),
			token1:       common.HexToAddress("0x300"),
			feeNumerator: StringToBigInt("997000"),
		}
		d.pairsInfo[common.HexToAddress("0x232")] = &PairInfo{
			reserve0:     FloatToBigInt(100 * math.Pow(10, 18)),
			reserve1:     FloatToBigInt(200 * math.Pow(10, 12)),
			token0:       common.HexToAddress("0x200"),
			token1:       common.HexToAddress("0x300"),
			feeNumerator: StringToBigInt("997000"),
		}
		d.pairsInfo[common.HexToAddress("0x311")] = &PairInfo{
			reserve0:     FloatToBigInt(300 * math.Pow(10, 12)), //300 * 10**12
			reserve1:     FloatToBigInt(100 * math.Pow(10, 18)),
			token0:       common.HexToAddress("0x300"),
			token1:       common.HexToAddress("0x100"),
			feeNumerator: StringToBigInt("997000"),
		}
		root1 := findRoot(d.pairsInfo[common.HexToAddress("0x121")],
			d.pairsInfo[common.HexToAddress("0x122")], common.HexToAddress("0x100"))
		root2 := findRoot(d.pairsInfo[common.HexToAddress("0x231")],
			d.pairsInfo[common.HexToAddress("0x232")], common.HexToAddress("0x200"))
		route := []*Leg{
			&Leg{
				From: common.HexToAddress("0x100"),
				To:   common.HexToAddress("0x200"),
				// PairAddrs: []common.Address{common.HexToAddress("0x121")},
				PairAddrs: []common.Address{common.HexToAddress("0x121"), common.HexToAddress("0x122")},
				Root:      root1,
				// Root: big.NewInt(0),
			},
			&Leg{
				From:      common.HexToAddress("0x200"),
				To:        common.HexToAddress("0x300"),
				PairAddrs: []common.Address{common.HexToAddress("0x231"), common.HexToAddress("0x232")},
				// PairAddrs: []common.Address{common.HexToAddress("0x231")},
				Root: root2,
				// Root: big.NewInt(0),
			},
			&Leg{
				From:      common.HexToAddress("0x300"),
				To:        common.HexToAddress("0x100"),
				PairAddrs: []common.Address{common.HexToAddress("0x311")},
				// Root:      root3,
				Root: big.NewInt(0),
			},
		}
		converged, amountIn := d.calcOptimalAmountInSecant(
			amountIn, route, map[common.Address]*PairInfo{})
		profit := new(big.Int).Sub(
			d.getRouteAmountOut(route, amountIn, map[common.Address]*PairInfo{}), amountIn)
		expectedAmountIn := StringToBigInt("2353500752276077987")
		expectedProfit := StringToBigInt("481357736212512950")
		// fmt.Printf("Converged: %t, amountIn: %s, profit: %s\n", converged, amountIn, profit)
		if assert.Equal(amountIn, expectedAmountIn, "Calculated amountIn incorrectly") &&
			assert.Equal(converged, true, "Didn't converge in optimisation") &&
			assert.Equal(profit, expectedProfit, "Calculated profit incorrectly") {
			fmt.Println("\tcalcOptimalAmountInSecant: \t\tPASS")
		}
	})

	t.Run("calcOptimalInSecant - Real", func(t *testing.T) {
		// assert := assert.New(t)
		oneFtm := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
		d := &Dexter{
			pairsInfo: make(map[common.Address]*PairInfo),
		}
		d.pairsInfo[common.HexToAddress("0xd061c6586670792331E14a80f3b3Bb267189C681")] = &PairInfo{
			reserve0:     StringToBigInt("629163351310563109478052"),
			reserve1:     StringToBigInt("42524027912599396341141"),
			token0:       common.HexToAddress("0x21be370D5312f44cB42ce377BC9b8a0cEF1A4C83"),
			token1:       common.HexToAddress("0xb3654dc3D10Ea7645f8319668E8F54d2574FBdC8"),
			feeNumerator: StringToBigInt("997000"),
		}
		d.pairsInfo[common.HexToAddress("0x89d9bC2F2d091CfBFc31e333D6Dc555dDBc2fd29")] = &PairInfo{
			reserve0:     StringToBigInt("3633894376721094672605439"),
			reserve1:     StringToBigInt("241359587850261671320718"),
			token0:       common.HexToAddress("0x21be370D5312f44cB42ce377BC9b8a0cEF1A4C83"),
			token1:       common.HexToAddress("0xb3654dc3D10Ea7645f8319668E8F54d2574FBdC8"),
			feeNumerator: StringToBigInt("998000"),
		}
		route := []*Leg{
			&Leg{
				From:      common.HexToAddress("0x21be370D5312f44cB42ce377BC9b8a0cEF1A4C83"),
				To:        common.HexToAddress("0xb3654dc3D10Ea7645f8319668E8F54d2574FBdC8"),
				PairAddrs: []common.Address{common.HexToAddress("0xd061c6586670792331E14a80f3b3Bb267189C681")},
				Root:      big.NewInt(0),
			},
			&Leg{
				From:      common.HexToAddress("0xb3654dc3D10Ea7645f8319668E8F54d2574FBdC8"),
				To:        common.HexToAddress("0x21be370D5312f44cB42ce377BC9b8a0cEF1A4C83"),
				PairAddrs: []common.Address{common.HexToAddress("0x89d9bC2F2d091CfBFc31e333D6Dc555dDBc2fd29")},
				Root:      big.NewInt(0),
			},
		}
		start := time.Now()
		for i := 0; i < 10000; i++ {
			amountIn := new(big.Int).Mul(oneFtm, big.NewInt(10))
			d.calcOptimalAmountInSecant(
				amountIn, route, map[common.Address]*PairInfo{})
			// converged, optimalIn, profit := d.calcOptimalAmountInSecant(
			// 	amountIn, route, map[common.Address]*PairInfo{})
		}
		end := time.Now()
		fmt.Printf("Secant Method\n")
		fmt.Printf("Total: %s, perCycle: %s\n", end.Sub(start), (end.Sub(start))/10000)
		start = time.Now()
		for i := 0; i <= 10000; i++ {
			d.getRouteOptimalAmountIn(
				route, map[common.Address]*PairInfo{})
		}
		end = time.Now()
		fmt.Printf("Conventional Method\n")
		fmt.Printf("Total: %s, perCycle: %s\n", end.Sub(start), (end.Sub(start))/10000)
		// amountIn1 := d.getRouteOptimalAmountIn(
		// 	route, map[common.Address]*PairInfo{})
		// amountOut1 := d.getRouteAmountOut(
		// 	route, amountIn1, map[common.Address]*PairInfo{})
		// fmt.Printf("Converged: %t, amountIn: %s, profit: %s\n", converged, amountIn, profit)
		// expectedAmountIn := StringToBigInt("3349726263711452143630")
		// expectedProfit := StringToBigInt("20907207122951340032")
		// // fmt.Printf("amountIn: %s, amountOut1: %s\n", amountIn1, amountOut1)
		// if assert.Equal(optimalIn, expectedAmountIn, "Calculated amountIn incorrectly") &&
		// 	assert.Equal(converged, true, "Didn't converge in optimisation") &&
		// 	assert.Equal(profit, expectedProfit, "Calculated profit incorrectly") {
		// 	fmt.Println("\tcalcOptimalAmountInSecant - Real: \tPASS")
		// }
	})

	// 	t.Run("calcOptimalInSecant - Real", func(t *testing.T) {
	// 		// assert := assert.New(t)
	// 		oneFtm := new(big.Int).Exp(big.NewInt(10), big.NewInt(16), nil)
	// 		d := &Dexter{
	// 			pairsInfo: make(map[common.Address]*PairInfo),
	// 		}
	// 		d.pairsInfo[common.HexToAddress("0x001")] = &PairInfo{
	// 			reserve0:     StringToBigInt("2105796329616091887546"),
	// 			reserve1:     StringToBigInt("4268086"),
	// 			token0:       common.HexToAddress("0x100"),
	// 			token1:       common.HexToAddress("0x200"),
	// 			feeNumerator: StringToBigInt("997000"),
	// 		}
	// 		d.pairsInfo[common.HexToAddress("0x002")] = &PairInfo{
	// 			reserve0:     StringToBigInt("4528227"),
	// 			reserve1:     StringToBigInt("611854350563310736"),
	// 			token0:       common.HexToAddress("0x200"),
	// 			token1:       common.HexToAddress("0x300"),
	// 			feeNumerator: StringToBigInt("998000"),
	// 		}
	// 		d.pairsInfo[common.HexToAddress("0x003")] = &PairInfo{
	// 			reserve0:     StringToBigInt("3348798373959605435525"),
	// 			reserve1:     StringToBigInt("12345074667576780714704155"),
	// 			token0:       common.HexToAddress("0x300"),
	// 			token1:       common.HexToAddress("0x100"),
	// 			feeNumerator: StringToBigInt("998000"),
	// 		}
	// 		route := []*Leg{
	// 			&Leg{
	// 				From:      common.HexToAddress("0x100"),
	// 				To:        common.HexToAddress("0x200"),
	// 				PairAddrs: []common.Address{common.HexToAddress("0x001")},
	// 				Root:      big.NewInt(0),
	// 			},
	// 			&Leg{
	// 				From:      common.HexToAddress("0x200"),
	// 				To:        common.HexToAddress("0x300"),
	// 				PairAddrs: []common.Address{common.HexToAddress("0x002")},
	// 				Root:      big.NewInt(0),
	// 			},
	// 			&Leg{
	// 				From:      common.HexToAddress("0x300"),
	// 				To:        common.HexToAddress("0x100"),
	// 				PairAddrs: []common.Address{common.HexToAddress("0x003")},
	// 				Root:      big.NewInt(0),
	// 			},
	// 		}
	// 		for i := 0; i <= 60; i++ {
	// 			amountIn := new(big.Int).Mul(oneFtm, big.NewInt(int64(i*5)))
	// 			profit := new(big.Int).Sub(d.getRouteAmountOut(route, amountIn, map[common.Address]*PairInfo{}),
	// 				amountIn)
	// 			fmt.Printf("%s, %s\n", amountIn, profit)
	// 			// fmt.Printf("In: %s, Profit: %s\n", amountIn, profit)
	// 		}
	// 		// converged, optimalIn, profit := d.calcOptimalAmountInSecant(
	// 		// 	amountIn, route, map[common.Address]*PairInfo{})
	// 		// amountIn1 := d.getRouteOptimalAmountIn(
	// 		// 	route, map[common.Address]*PairInfo{})
	// 		// amountOut1 := d.getRouteAmountOut(
	// 		// 	route, amountIn1, map[common.Address]*PairInfo{})
	// 		// fmt.Printf("Converged: %t, amountIn: %s, profit: %s\n", converged, optimalIn, profit)
	// 		// fmt.Printf("amountIn1: %s, amountOut1: %s, profit: %s\n",
	// 		// 	amountIn1, amountOut1, new(big.Int).Sub(amountOut1, amountIn1))
	// 		// expectedAmountIn := StringToBigInt("3349726263711442938574")
	// 		// expectedProfit := StringToBigInt("20907207122950815744")
	// 		// if assert.Equal(optimalIn, expectedAmountIn, "Calculated amountIn incorrectly") &&
	// 		// 	assert.Equal(converged, true, "Didn't converge in optimisation") &&
	// 		// 	assert.Equal(profit, expectedProfit, "Calculated profit incorrectly") {
	// 		// 	fmt.Println("\tcalcOptimalAmountInSecant - Real: \tPASS")
	// 		// }
	// 	})

	// t.Run("calcOptimalInSecant - Real", func(t *testing.T) {
	// 	// assert := assert.New(t)
	// 	oneFtm := new(big.Int).Exp(big.NewInt(10), big.NewInt(16), nil)
	// 	d := &Dexter{
	// 		pairsInfo: make(map[common.Address]*PairInfo),
	// 	}
	// 	d.pairsInfo[common.HexToAddress("0x001")] = &PairInfo{
	// 		reserve0:     StringToBigInt("432335087760602235148"),
	// 		reserve1:     StringToBigInt("877854"),
	// 		token0:       common.HexToAddress("0x100"),
	// 		token1:       common.HexToAddress("0x200"),
	// 		feeNumerator: StringToBigInt("998000"),
	// 	}
	// 	d.pairsInfo[common.HexToAddress("0x002")] = &PairInfo{
	// 		reserve0:     StringToBigInt("6115595"),
	// 		reserve1:     StringToBigInt("2233505106"),
	// 		token0:       common.HexToAddress("0x200"),
	// 		token1:       common.HexToAddress("0x300"),
	// 		feeNumerator: StringToBigInt("998000"),
	// 	}
	// 	d.pairsInfo[common.HexToAddress("0x003")] = &PairInfo{
	// 		reserve0:     StringToBigInt("19928766698"),
	// 		reserve1:     StringToBigInt("19925714307"),
	// 		token0:       common.HexToAddress("0x300"),
	// 		token1:       common.HexToAddress("0x400"),
	// 		feeNumerator: StringToBigInt("998000"),
	// 	}
	// 	d.pairsInfo[common.HexToAddress("0x004")] = &PairInfo{
	// 		reserve0:     StringToBigInt("45404655799019"),
	// 		reserve1:     StringToBigInt("61936815903307971232184766"),
	// 		token0:       common.HexToAddress("0x400"),
	// 		token1:       common.HexToAddress("0x100"),
	// 		feeNumerator: StringToBigInt("998000"),
	// 	}
	// 	route := []*Leg{
	// 		&Leg{
	// 			From:      common.HexToAddress("0x100"),
	// 			To:        common.HexToAddress("0x200"),
	// 			PairAddrs: []common.Address{common.HexToAddress("0x001")},
	// 			Root:      big.NewInt(0),
	// 		},
	// 		&Leg{
	// 			From:      common.HexToAddress("0x200"),
	// 			To:        common.HexToAddress("0x300"),
	// 			PairAddrs: []common.Address{common.HexToAddress("0x002")},
	// 			Root:      big.NewInt(0),
	// 		},
	// 		&Leg{
	// 			From:      common.HexToAddress("0x300"),
	// 			To:        common.HexToAddress("0x400"),
	// 			PairAddrs: []common.Address{common.HexToAddress("0x003")},
	// 			Root:      big.NewInt(0),
	// 		},
	// 		&Leg{
	// 			From:      common.HexToAddress("0x400"),
	// 			To:        common.HexToAddress("0x100"),
	// 			PairAddrs: []common.Address{common.HexToAddress("0x004")},
	// 			Root:      big.NewInt(0),
	// 		},
	// 	}
	// 	for i := 0; i <= 120; i++ {
	// 		amountIn := new(big.Int).Mul(oneFtm, big.NewInt(int64(i)))
	// 		profit := new(big.Int).Sub(d.getRouteAmountOut(route, amountIn, map[common.Address]*PairInfo{}),
	// 			amountIn)
	// 		fmt.Printf("%s, %s\n", amountIn, profit)
	// 		// fmt.Printf("In: %s, Profit: %s\n", amountIn, profit)
	// 	}
	// 	// amountIn := new(big.Int).Mul(oneFtm, big.NewInt(1))
	// 	// converged, optimalIn, profit := d.calcOptimalAmountInSecant(
	// 	// 	amountIn, route, map[common.Address]*PairInfo{})
	// 	// amountIn1 := d.getRouteOptimalAmountIn(
	// 	// 	route, map[common.Address]*PairInfo{})
	// 	// amountOut1 := d.getRouteAmountOut(
	// 	// 	route, amountIn1, map[common.Address]*PairInfo{})
	// 	// fmt.Printf("Converged: %t, amountIn: %s, profit: %s\n", converged, optimalIn, profit)
	// 	// fmt.Printf("amountIn1: %s, amountOut1: %s, profit: %s\n",
	// 	// 	amountIn1, amountOut1, new(big.Int).Sub(amountOut1, amountIn1))
	// 	// expectedAmountIn := StringToBigInt("3349726263711442938574")
	// 	// expectedProfit := StringToBigInt("20907207122950815744")
	// 	// if assert.Equal(optimalIn, expectedAmountIn, "Calculated amountIn incorrectly") &&
	// 	// 	assert.Equal(converged, true, "Didn't converge in optimisation") &&
	// 	// 	assert.Equal(profit, expectedProfit, "Calculated profit incorrectly") {
	// 	// 	fmt.Println("\tcalcOptimalAmountInSecant - Real: \tPASS")
	// 	// }
	// })

	// t.Run("calcOptimalInSecant - Real", func(t *testing.T) {
	// 	// assert := assert.New(t)
	// 	oneFtm := new(big.Int).Exp(big.NewInt(10), big.NewInt(16), nil)
	// 	d := &Dexter{
	// 		pairsInfo: make(map[common.Address]*PairInfo),
	// 	}
	// 	d.pairsInfo[common.HexToAddress("0x001")] = &PairInfo{
	// 		reserve0:     StringToBigInt("1911226087107811784634"),
	// 		reserve1:     StringToBigInt("514882644141220677"),
	// 		token0:       common.HexToAddress("0x100"),
	// 		token1:       common.HexToAddress("0x200"),
	// 		feeNumerator: StringToBigInt("997000"),
	// 	}
	// 	d.pairsInfo[common.HexToAddress("0x002")] = &PairInfo{
	// 		reserve0:     StringToBigInt("6946347987803888190"),
	// 		reserve1:     StringToBigInt("52285903"),
	// 		token0:       common.HexToAddress("0x200"),
	// 		token1:       common.HexToAddress("0x300"),
	// 		feeNumerator: StringToBigInt("998000"),
	// 	}
	// 	d.pairsInfo[common.HexToAddress("0x003")] = &PairInfo{
	// 		reserve0:     StringToBigInt("2243798"),
	// 		reserve1:     StringToBigInt("300756331455611576"),
	// 		token0:       common.HexToAddress("0x300"),
	// 		token1:       common.HexToAddress("0x400"),
	// 		feeNumerator: StringToBigInt("998000"),
	// 	}
	// 	d.pairsInfo[common.HexToAddress("0x004")] = &PairInfo{
	// 		reserve0:     StringToBigInt("3329561676181668243665"),
	// 		reserve1:     StringToBigInt("12388356730639641897997243"),
	// 		token0:       common.HexToAddress("0x400"),
	// 		token1:       common.HexToAddress("0x100"),
	// 		feeNumerator: StringToBigInt("998000"),
	// 	}
	// 	route := []*Leg{
	// 		&Leg{
	// 			From:      common.HexToAddress("0x100"),
	// 			To:        common.HexToAddress("0x200"),
	// 			PairAddrs: []common.Address{common.HexToAddress("0x001")},
	// 			Root:      big.NewInt(0),
	// 		},
	// 		&Leg{
	// 			From:      common.HexToAddress("0x200"),
	// 			To:        common.HexToAddress("0x300"),
	// 			PairAddrs: []common.Address{common.HexToAddress("0x002")},
	// 			Root:      big.NewInt(0),
	// 		},
	// 		&Leg{
	// 			From:      common.HexToAddress("0x300"),
	// 			To:        common.HexToAddress("0x400"),
	// 			PairAddrs: []common.Address{common.HexToAddress("0x003")},
	// 			Root:      big.NewInt(0),
	// 		},
	// 		&Leg{
	// 			From:      common.HexToAddress("0x400"),
	// 			To:        common.HexToAddress("0x100"),
	// 			PairAddrs: []common.Address{common.HexToAddress("0x004")},
	// 			Root:      big.NewInt(0),
	// 		},
	// 	}
	// 	for i := 0; i <= 60; i++ {
	// 		amountIn := new(big.Int).Mul(oneFtm, big.NewInt(int64(i*2)))
	// 		profit := new(big.Int).Sub(d.getRouteAmountOut(route, amountIn, map[common.Address]*PairInfo{}),
	// 			amountIn)
	// 		fmt.Printf("%s, %s\n", amountIn, profit)
	// 		// fmt.Printf("In: %s, Profit: %s\n", amountIn, profit)
	// 	}
	// 	// amountIn := new(big.Int).Mul(oneFtm, big.NewInt(1))
	// 	// converged, optimalIn, profit := d.calcOptimalAmountInSecant(
	// 	// 	amountIn, route, map[common.Address]*PairInfo{})
	// 	// amountIn1 := d.getRouteOptimalAmountIn(
	// 	// 	route, map[common.Address]*PairInfo{})
	// 	// amountOut1 := d.getRouteAmountOut(
	// 	// 	route, amountIn1, map[common.Address]*PairInfo{})
	// 	// fmt.Printf("Converged: %t, amountIn: %s, profit: %s\n", converged, optimalIn, profit)
	// 	// fmt.Printf("amountIn1: %s, amountOut1: %s, profit: %s\n",
	// 	// 	amountIn1, amountOut1, new(big.Int).Sub(amountOut1, amountIn1))
	// 	// expectedAmountIn := StringToBigInt("3349726263711442938574")
	// 	// expectedProfit := StringToBigInt("20907207122950815744")
	// 	// if assert.Equal(optimalIn, expectedAmountIn, "Calculated amountIn incorrectly") &&
	// 	// 	assert.Equal(converged, true, "Didn't converge in optimisation") &&
	// 	// 	assert.Equal(profit, expectedProfit, "Calculated profit incorrectly") {
	// 	// 	fmt.Println("\tcalcOptimalAmountInSecant - Real: \tPASS")
	// 	// }
	// })

	// 	t.Run("calcOptimalInSecant - Real", func(t *testing.T) {
	// 		// assert := assert.New(t)
	// 		oneFtm := new(big.Int).Exp(big.NewInt(10), big.NewInt(16), nil)
	// 		d := &Dexter{
	// 			pairsInfo: make(map[common.Address]*PairInfo),
	// 		}
	// 		d.pairsInfo[common.HexToAddress("0x001")] = &PairInfo{
	// 			reserve0:     StringToBigInt("1836172990064876167673927"),
	// 			reserve1:     StringToBigInt("549626279432840853385330"),
	// 			token0:       common.HexToAddress("0x100"),
	// 			token1:       common.HexToAddress("0x200"),
	// 			feeNumerator: StringToBigInt("998000"),
	// 		}
	// 		d.pairsInfo[common.HexToAddress("0x002")] = &PairInfo{
	// 			reserve0:     StringToBigInt("11608924620595691979112"),
	// 			reserve1:     StringToBigInt("10459534954520096583"),
	// 			token0:       common.HexToAddress("0x200"),
	// 			token1:       common.HexToAddress("0x300"),
	// 			feeNumerator: StringToBigInt("997000"),
	// 		}
	// 		d.pairsInfo[common.HexToAddress("0x003")] = &PairInfo{
	// 			reserve0:     StringToBigInt("6946347987803888190"),
	// 			reserve1:     StringToBigInt("52285903"),
	// 			token0:       common.HexToAddress("0x300"),
	// 			token1:       common.HexToAddress("0x400"),
	// 			feeNumerator: StringToBigInt("998000"),
	// 		}
	// 		d.pairsInfo[common.HexToAddress("0x004")] = &PairInfo{
	// 			reserve0:     StringToBigInt("4701437"),
	// 			reserve1:     StringToBigInt("2341533977515579320639"),
	// 			token0:       common.HexToAddress("0x400"),
	// 			token1:       common.HexToAddress("0x100"),
	// 			feeNumerator: StringToBigInt("997000"),
	// 		}
	// 		route := []*Leg{
	// 			&Leg{
	// 				From:      common.HexToAddress("0x100"),
	// 				To:        common.HexToAddress("0x200"),
	// 				PairAddrs: []common.Address{common.HexToAddress("0x001")},
	// 				Root:      big.NewInt(0),
	// 			},
	// 			&Leg{
	// 				From:      common.HexToAddress("0x200"),
	// 				To:        common.HexToAddress("0x300"),
	// 				PairAddrs: []common.Address{common.HexToAddress("0x002")},
	// 				Root:      big.NewInt(0),
	// 			},
	// 			&Leg{
	// 				From:      common.HexToAddress("0x300"),
	// 				To:        common.HexToAddress("0x400"),
	// 				PairAddrs: []common.Address{common.HexToAddress("0x003")},
	// 				Root:      big.NewInt(0),
	// 			},
	// 			&Leg{
	// 				From:      common.HexToAddress("0x400"),
	// 				To:        common.HexToAddress("0x100"),
	// 				PairAddrs: []common.Address{common.HexToAddress("0x004")},
	// 				Root:      big.NewInt(0),
	// 			},
	// 		}
	// 		for i := 0; i <= 100; i++ {
	// 			amountIn := new(big.Int).Mul(oneFtm, big.NewInt(int64(i*2)))
	// 			profit := new(big.Int).Sub(d.getRouteAmountOut(route, amountIn, map[common.Address]*PairInfo{}),
	// 				amountIn)
	// 			fmt.Printf("%s, %s\n", amountIn, profit)
	// 			// fmt.Printf("In: %s, Profit: %s\n", amountIn, profit)
	// 		}
	// 		// amountIn := new(big.Int).Mul(oneFtm, big.NewInt(1))
	// 		// converged, optimalIn, profit := d.calcOptimalAmountInSecant(
	// 		// 	amountIn, route, map[common.Address]*PairInfo{})
	// 		// amountIn1 := d.getRouteOptimalAmountIn(
	// 		// 	route, map[common.Address]*PairInfo{})
	// 		// amountOut1 := d.getRouteAmountOut(
	// 		// 	route, amountIn1, map[common.Address]*PairInfo{})
	// 		// fmt.Printf("Converged: %t, amountIn: %s, profit: %s\n", converged, optimalIn, profit)
	// 		// fmt.Printf("amountIn1: %s, amountOut1: %s, profit: %s\n",
	// 		// 	amountIn1, amountOut1, new(big.Int).Sub(amountOut1, amountIn1))
	// 		// expectedAmountIn := StringToBigInt("3349726263711442938574")
	// 		// expectedProfit := StringToBigInt("20907207122950815744")
	// 		// if assert.Equal(optimalIn, expectedAmountIn, "Calculated amountIn incorrectly") &&
	// 		// 	assert.Equal(converged, true, "Didn't converge in optimisation") &&
	// 		// 	assert.Equal(profit, expectedProfit, "Calculated profit incorrectly") {
	// 		// 	fmt.Println("\tcalcOptimalAmountInSecant - Real: \tPASS")
	// 		// }
	// 	})

}
