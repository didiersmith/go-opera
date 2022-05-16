package dexter

import (
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

type Dexter struct {
	poolsInfo map[common.Address]*PoolInfo
}

type MultiLeg struct {
	From      common.Address
	To        common.Address
	PoolAddrs []common.Address
	Root      *big.Int
}

func findRoot(poolInfo1, poolInfo2 *PoolInfo, fromToken common.Address) *big.Int {
	return nil
	// var r_a_i, r_a_o, r_b_i, r_b_o *big.Int
	// if bytes.Compare(poolInfo1.Tokens[0].Bytes(), fromToken.Bytes()) == 0 {
	// 	r_a_i, r_a_o = poolInfo1.Reserves[0], poolInfo1.Reserves[1]
	// } else {
	// 	r_a_o, r_a_i = poolInfo1.Reserves[0], poolInfo1.Reserves[1]
	// }
	// if bytes.Compare(poolInfo2.Tokens[0].Bytes(), fromToken.Bytes()) == 0 {
	// 	r_b_i, r_b_o = poolInfo2.Reserves[0], poolInfo2.Reserves[1]
	// } else {
	// 	r_b_o, r_b_i = poolInfo2.Reserves[0], poolInfo2.Reserves[1]
	// }
	// f_d := new(big.Int).Exp(big.NewInt(10), big.NewInt(6), nil)
	// f_a := poolInfo1.FeeNumerator
	// a := new(big.Int).Div(new(big.Int).Mul(r_b_o, f_a), f_d)
	// b := new(big.Int).Mul(r_b_o, new(big.Int).Add(new(big.Int).Div(new(big.Int).Mul(r_a_i, f_a), f_d), r_a_i))
	// c := new(big.Int).Sub(new(big.Int).Mul(r_b_o, new(big.Int).Exp(r_a_i, big.NewInt(2), nil)),
	// 	new(big.Int).Mul(r_b_i, new(big.Int).Mul(r_a_o, r_a_i)))
	// sqrt_b_squared_four_a_c := new(big.Int).Sqrt(new(big.Int).Sub(new(big.Int).Exp(b, big.NewInt(2), nil),
	// 	new(big.Int).Mul(big.NewInt(4), new(big.Int).Mul(a, c))))
	// root := new(big.Int).Div(new(big.Int).Add(new(big.Int).Neg(b), sqrt_b_squared_four_a_c),
	// 	new(big.Int).Mul(a, big.NewInt(2)))
	// return root
	// root = (-b+sqrt(b**2-4*a*c))/(2*a)
}

func (d *Dexter) getPoolInfo(poolsInfoOverride map[common.Address]*PoolInfo, poolAddr common.Address) *PoolInfo {
	return nil
	// if poolsInfoOverride != nil {
	// 	if poolInfo, ok := poolsInfoOverride[poolAddr]; ok {
	// 		if poolInfo.Reserves[0].BitLen() == 0 || poolInfo.Reserves[1].BitLen() == 0 {
	// 			// log.Info("WARNING: getPoolInfo found override with 0 Reserves[0] or Reserves[1]", "Addr", poolAddr, "len", len(poolsInfoOverride))
	// 		}
	// 		return poolInfo
	// 	}
	// }
	// poolInfo := d.poolsInfo[poolAddr]
	// // fmt.Printf("R0: %s, R1: %s\n", poolInfo.Reserves[0], poolInfo.Reserves[1])
	// if poolInfo.Reserves[0].BitLen() == 0 || poolInfo.Reserves[1].BitLen() == 0 {
	// 	// log.Info("WARNING: getPoolInfo found baseline with 0 Reserves[0] or Reserves[1]", "Addr", poolAddr, "Reserves[0]", poolInfo.Reserves[0], "Reserves[1]", poolInfo.Reserves[1])
	// }
	// return poolInfo
}

// fracInFirst = (root + (amountIn-root)(totalReserves1)/totalReserves)/amountIn
func calcInFractions(
	amountIn, root, pool1ReserveIn, pool1ReserveOut, pool2ReserveIn, pool2ReserveOut *big.Int) *big.Int {
	totalReserves1 := new(big.Int).Add(pool1ReserveIn, pool1ReserveOut)
	totalReserves := new(big.Int).Add(totalReserves1, pool2ReserveIn)
	totalReserves = totalReserves.Add(totalReserves, pool2ReserveOut)
	fracInFirst := new(big.Int).Sub(amountIn, root)
	fracInFirst = fracInFirst.Mul(fracInFirst, totalReserves1)
	fracInFirst = fracInFirst.Div(fracInFirst, totalReserves)
	fracInFirst = fracInFirst.Add(fracInFirst, root)
	fracInFirst = fracInFirst.Mul(fracInFirst, big.NewInt(1000000))
	fracInFirst = fracInFirst.Div(fracInFirst, amountIn)
	return fracInFirst
}

func getAmountOut(amountIn, reserveIn, reserveOut, feeNumerator *big.Int) *big.Int {
	amountInWithFee := new(big.Int).Mul(amountIn, feeNumerator)
	numerator := new(big.Int).Mul(amountInWithFee, reserveOut)
	denominator := new(big.Int).Exp(big.NewInt(10), big.NewInt(6), nil)
	denominator = denominator.Mul(denominator, reserveIn)
	denominator = denominator.Add(denominator, amountInWithFee)
	return new(big.Int).Div(numerator, denominator)
}

func getAmountOutMulti(
	amountIn, root, fee1, fee2 *big.Int, reserves [4]*big.Int) *big.Int {
	pool1ReserveIn, pool1ReserveOut := reserves[0], reserves[1]
	pool2ReserveIn, pool2ReserveOut := reserves[2], reserves[3]
	fracInFirst := calcInFractions(
		amountIn, root, pool1ReserveIn, pool1ReserveOut, pool2ReserveIn, pool2ReserveOut)
	amountInFirst := new(big.Int).Mul(amountIn, fracInFirst)
	amountInFirst = amountInFirst.Div(amountInFirst, big.NewInt(1000000))
	amountInSecond := new(big.Int).Sub(amountIn, amountInFirst)
	// fmt.Printf("Root: %s\n", root)
	// fmt.Printf("FracinFirst: %s, amountIntFirst: %s, amountInSecond: %s\n",
	// 	fracInFirst, amountInFirst, amountInSecond)
	amountOutFirst := getAmountOut(amountInFirst, pool1ReserveIn, pool1ReserveOut, fee1)
	amountOutSecond := getAmountOut(amountInSecond, pool2ReserveIn, pool2ReserveOut, fee2)
	return new(big.Int).Add(amountOutFirst, amountOutSecond)
}

func getAmountOutPrime(amountInBI, reserveInBI, reserveOutBI, feeNumerator *big.Int) float64 {
	amountIn := BigIntToFloat(amountInBI)
	reserveIn := BigIntToFloat(reserveInBI)
	reserveOut := BigIntToFloat(reserveOutBI)
	fee := BigIntToFloat(feeNumerator) / math.Pow(10, 6)
	numerator := fee * reserveIn * reserveOut
	denominator := math.Pow(reserveIn+fee*amountIn, 2)
	if denominator == 0 {
		return 0.
	}
	return numerator / denominator
}

func getAmountOutMultiPrime(
	amountInBI, rootBI, fee1, fee2 *big.Int, reserves [4]*big.Int) float64 {
	pool1ReserveIn, pool1ReserveOut := BigIntToFloat(reserves[0]), BigIntToFloat(reserves[1])
	pool2ReserveIn, pool2ReserveOut := BigIntToFloat(reserves[2]), BigIntToFloat(reserves[3])
	feeDenominator := 1000000.
	amountIn := BigIntToFloat(amountInBI)
	root := BigIntToFloat(rootBI)
	pool1Fee := BigIntToFloat(fee1) / feeDenominator
	pool2Fee := BigIntToFloat(fee2) / feeDenominator
	totalReserves1 := pool1ReserveIn + pool1ReserveOut
	totalReserves2 := pool2ReserveIn + pool2ReserveOut
	totalReserves := totalReserves1 + totalReserves2
	pool1LeftNum := pool1Fee * pool1ReserveOut * totalReserves1
	pool1LeftDen := pool1Fee*totalReserves1*(amountIn-root) + (pool1Fee*root+pool1ReserveIn)*totalReserves
	pool1RightNum := -pool1LeftNum * pool1Fee * (totalReserves1*(amountIn-root) + root*totalReserves)
	pool1RightDen := math.Pow(pool1LeftDen, 2)
	pool2LeftNum := pool2Fee * pool2ReserveOut * totalReserves2
	pool2LeftDen := pool2Fee*totalReserves2*(amountIn-root) + pool2ReserveIn*totalReserves
	pool2RightNum := -pool2LeftNum * pool2Fee * totalReserves2 * (amountIn - root)
	pool2RightDen := math.Pow(pool2LeftDen, 2)
	pool1Prime := pool1LeftNum/pool1LeftDen + pool1RightNum/pool1RightDen
	pool2Prime := pool2LeftNum/pool2LeftDen + pool2RightNum/pool2RightDen
	amountOutPrime := pool1Prime + pool2Prime
	// fmt.Printf("P1LN: %g, P1LD: %g, P1RN: %g, P1RD: %g\n",
	// 	pool1LeftNum, pool1LeftDen, pool1RightNum, pool1RightDen)
	// fmt.Printf("P2LN: %g, P2LD: %g, P2RN: %g, P2RD: %g\n",
	// 	pool2LeftNum, pool2LeftDen, pool2RightNum, pool2RightDen)
	return amountOutPrime
}

func getAmountOutSecondPrime(amountInBI, reserveInBI, reserveOutBI, feeNumerator *big.Int) float64 {
	amountIn := BigIntToFloat(amountInBI)
	reserveIn := BigIntToFloat(reserveInBI)
	reserveOut := BigIntToFloat(reserveOutBI)
	fee := BigIntToFloat(feeNumerator) / math.Pow(10, 6)
	numerator := -2. * math.Pow(fee, 2) * reserveIn * reserveOut
	denominator := math.Pow(reserveIn+fee*amountIn, 3)
	if denominator == 0 {
		return 0.
	}
	return numerator / denominator
}

func getAmountOutMultiSecondPrime(
	amountInBI, rootBI, fee1, fee2 *big.Int, reserves [4]*big.Int) float64 {
	pool1ReserveIn, pool1ReserveOut := BigIntToFloat(reserves[0]), BigIntToFloat(reserves[1])
	pool2ReserveIn, pool2ReserveOut := BigIntToFloat(reserves[2]), BigIntToFloat(reserves[3])
	feeDenominator := 1000000.
	amountIn := BigIntToFloat(amountInBI)
	root := BigIntToFloat(rootBI)
	pool1Fee := BigIntToFloat(fee1) / feeDenominator
	pool2Fee := BigIntToFloat(fee2) / feeDenominator
	totalReserves1 := pool1ReserveIn + pool1ReserveOut
	totalReserves2 := pool2ReserveIn + pool2ReserveOut
	totalReserves := totalReserves1 + totalReserves2
	// Pool 1
	pool1LeftNum := -(2 * math.Pow(pool1Fee, 2) * pool1ReserveOut * math.Pow(totalReserves1, 2))
	pool1RightNum := pool1LeftNum * pool1Fee * (totalReserves1*(amountIn-root) + root*totalReserves)
	pool1DenExp := pool1Fee*totalReserves1*(amountIn-root) + (pool1Fee*root+pool1ReserveIn)*totalReserves
	pool1LeftDen := math.Pow(pool1DenExp, 2)
	pool1RightDen := math.Pow(pool1DenExp, 3)
	// Pool 2
	pool2LeftNum := -(2 * math.Pow(pool2Fee, 2) * pool2ReserveOut * math.Pow(totalReserves2, 2))
	pool2RightNum := pool2LeftNum * pool2Fee * totalReserves2 * (amountIn - root)
	pool2DenExp := pool2Fee*totalReserves2*(amountIn-root) + pool2ReserveIn*totalReserves
	pool2LeftDen := math.Pow(pool2DenExp, 2)
	pool2RightDen := math.Pow(pool2DenExp, 3)
	// Merger
	pool1Prime := pool1LeftNum/pool1LeftDen + pool1RightNum/pool1RightDen
	pool2Prime := pool2LeftNum/pool2LeftDen + pool2RightNum/pool2RightDen
	amountOutSecondPrime := pool1Prime + pool2Prime
	// fmt.Printf("amountIn: %g\n", amountIn)
	// fmt.Printf("P1LN: %g, P1LD: %g, P1RN: %g, P1RD: %g\n",
	// 	pool1LeftNum, pool1LeftDen, pool1RightNum, pool1RightDen)
	// fmt.Printf("P2LN: %g, P2LD: %g, P2RN: %g, P2RD: %g\n",
	// 	pool2LeftNum, pool2LeftDen, pool2RightNum, pool2RightDen)
	// fmt.Printf("2ndPrime: %g\n", amountOutSecondPrime)
	return amountOutSecondPrime
}

func (d *Dexter) getRouteAmountOut(
	route []*MultiLeg, amountIn *big.Int, poolsInfoOverride map[common.Address]*PoolInfo) *big.Int {
	return nil
	// var amountOut *big.Int
	// var reserves [4]*big.Int
	// for _, leg := range route {
	// 	poolInfo1 := d.getPoolInfo(poolsInfoOverride, leg.PoolAddrs[0])
	// 	if bytes.Compare(poolInfo1.Tokens[0].Bytes(), leg.From.Bytes()) == 0 {
	// 		reserves[0], reserves[1] = poolInfo1.Reserves[0], poolInfo1.Reserves[1]
	// 	} else {
	// 		reserves[0], reserves[1] = poolInfo1.Reserves[1], poolInfo1.Reserves[0]
	// 	}
	// 	if len(leg.PoolAddrs) > 1 {
	// 		poolInfo2 := d.getPoolInfo(poolsInfoOverride, leg.PoolAddrs[1])
	// 		if bytes.Compare(poolInfo2.Tokens[0].Bytes(), leg.From.Bytes()) == 0 {
	// 			reserves[2], reserves[3] = poolInfo2.Reserves[0], poolInfo2.Reserves[1]
	// 		} else {
	// 			reserves[2], reserves[3] = poolInfo2.Reserves[1], poolInfo2.Reserves[0]
	// 		}
	// 		amountOut = getAmountOutMulti(
	// 			amountIn, leg.Root, poolInfo1.FeeNumerator, poolInfo2.FeeNumerator, reserves)
	// 		// fmt.Printf("Leg: %d, amountOut(Multi): %s\n", i, amountOut)
	// 	} else {
	// 		amountOut = getAmountOut(amountIn, reserves[0], reserves[1], poolInfo1.FeeNumerator)
	// 		// fmt.Printf("Leg: %d, amountOut(Single): %s\n", i, amountOut)
	// 	}
	// 	amountIn = amountOut
	// 	// fmt.Printf("R1In: %s, R1Out: %s, R2InL: %s, R2Out: %s\n",
	// 	// 	reserves[0], reserves[1], reserves[2], reserves[3])
	// }
	// return amountOut
}

func (d *Dexter) getRouteAmountOutPrime(
	route []*MultiLeg, amountIn1, amountIn2 *big.Int, poolsInfoOverride map[common.Address]*PoolInfo) (float64, float64) {
	return 0., 0.
	// prime1 := 1.
	// prime2 := 1.
	// var reserves [4]*big.Int
	// for _, leg := range route {
	// 	poolInfo1 := d.getPoolInfo(poolsInfoOverride, leg.PoolAddrs[0])
	// 	if bytes.Compare(poolInfo1.Tokens[0].Bytes(), leg.From.Bytes()) == 0 {
	// 		reserves[0], reserves[1] = poolInfo1.Reserves[0], poolInfo1.Reserves[1]
	// 	} else {
	// 		reserves[0], reserves[1] = poolInfo1.Reserves[1], poolInfo1.Reserves[0]
	// 	}
	// 	if len(leg.PoolAddrs) > 1 {
	// 		poolInfo2 := d.getPoolInfo(poolsInfoOverride, leg.PoolAddrs[1])
	// 		if bytes.Compare(poolInfo2.Tokens[0].Bytes(), leg.From.Bytes()) == 0 {
	// 			reserves[2], reserves[3] = poolInfo2.Reserves[0], poolInfo2.Reserves[1]
	// 		} else {
	// 			reserves[2], reserves[3] = poolInfo2.Reserves[1], poolInfo2.Reserves[0]
	// 		}
	// 		prime1 *= getAmountOutMultiPrime(
	// 			amountIn1, leg.Root, poolInfo1.FeeNumerator, poolInfo2.FeeNumerator, reserves)
	// 		amountIn1 = getAmountOutMulti(
	// 			amountIn1, leg.Root, poolInfo1.FeeNumerator, poolInfo2.FeeNumerator, reserves)
	// 		prime2 *= getAmountOutMultiPrime(
	// 			amountIn2, leg.Root, poolInfo1.FeeNumerator, poolInfo2.FeeNumerator, reserves)
	// 		amountIn2 = getAmountOutMulti(
	// 			amountIn2, leg.Root, poolInfo1.FeeNumerator, poolInfo2.FeeNumerator, reserves)
	// 	} else {
	// 		prime1 *= getAmountOutPrime(amountIn1, reserves[0], reserves[1], poolInfo1.FeeNumerator)
	// 		amountIn1 = getAmountOut(amountIn1, reserves[0], reserves[1], poolInfo1.FeeNumerator)
	// 		prime2 *= getAmountOutPrime(amountIn2, reserves[0], reserves[1], poolInfo1.FeeNumerator)
	// 		amountIn2 = getAmountOut(amountIn2, reserves[0], reserves[1], poolInfo1.FeeNumerator)
	// 	}
	// }
	// return prime1, prime2
}

func (d *Dexter) getRouteAmountOutSecondPrime(
	route []*MultiLeg, amountIn *big.Int, poolsInfoOverride map[common.Address]*PoolInfo) float64 {
	return 0.
	//// Get reserves and fees for each pool of each leg of route, store as 2-D arrays
	//reserves := make([][4]*big.Int, len(route))
	//fees := make([][2]*big.Int, len(route))
	//for i, leg := range route {
	//	poolInfo1 := d.getPoolInfo(poolsInfoOverride, leg.PoolAddrs[0])
	//	var reserveFrom *big.Int
	//	var reserveTo *big.Int
	//	if bytes.Compare(poolInfo1.Tokens[0].Bytes(), leg.From.Bytes()) == 0 {
	//		reserveFrom, reserveTo = poolInfo1.Reserves[0], poolInfo1.Reserves[1]
	//	} else {
	//		reserveFrom, reserveTo = poolInfo1.Reserves[1], poolInfo1.Reserves[0]
	//	}
	//	reserves[i][0] = reserveFrom
	//	reserves[i][1] = reserveTo
	//	fees[i][0] = poolInfo1.FeeNumerator
	//	if len(leg.PoolAddrs) > 1 {
	//		poolInfo2 := d.getPoolInfo(poolsInfoOverride, leg.PoolAddrs[0])
	//		var reserveFrom *big.Int
	//		var reserveTo *big.Int
	//		if bytes.Compare(poolInfo2.Tokens[0].Bytes(), leg.From.Bytes()) == 0 {
	//			reserveFrom, reserveTo = poolInfo2.Reserves[0], poolInfo2.Reserves[1]
	//		} else {
	//			reserveFrom, reserveTo = poolInfo2.Reserves[1], poolInfo2.Reserves[0]
	//		}
	//		reserves[i][2] = reserveFrom
	//		reserves[i][3] = reserveTo
	//		fees[i][1] = poolInfo2.FeeNumerator
	//	}
	//}
	//// for _, i := range reserves {
	//// 	fmt.Printf("RESERVES - In1: %v, Out1: %v, In2: %v, Out2: %v\n", i[0], i[1], i[2], i[3])
	//// }
	//// for _, i := range fees {
	//// 	fmt.Printf("Fee1: %v, Fee2: %v\n", i[0], i[1])
	//// }
	//// Get amountIn for each leg of route, store as array
	//amountsIn := make([]*big.Int, len(route))
	//for i, leg := range reserves {
	//	if i == 0 {
	//		amountsIn[i] = amountIn
	//		continue
	//	}
	//	if leg[2] != nil {
	//		amountsIn[i] = getAmountOutMulti(
	//			amountsIn[i-1], route[i].Root, fees[i][0], fees[i][1], leg)
	//	} else {
	//		amountsIn[i] = getAmountOut(amountsIn[i-1], leg[0], leg[1], fees[i][0])
	//	}
	//}
	//// for f, amountIn := range amountsIn {
	//// 	fmt.Printf("amountIn%d: %s\n", f, amountIn)
	//// }
	//// Second derivative of getRouteAmountOut
	//// let f' denote the derivate of f
	//// gAOi(j) = getAmountOut for leg i with j amountIn
	//// AIi = amountIn at leg i
	//// Examples:
	//// Length 1 cycle
	//// gRAO'' = gAO0(AI0)
	//// Length 2 cycle
	//// gRAO'' = gAO1''(AI1) * 2*gAO0'(AI0)
	////		  + gAO1'(AI1)  * gAO0''(AI0)
	//// Length 3 cycle
	//// gRAO'' = gAO2''(AI2) * 2*gAO1'(AI1) * 2*gAO0'(AI0)
	////		  + gAO2'(AI2)  * gAO1''(AI1)  * 2*gAO0'(AI0)
	////		  + gAO2'(AI2)  * gAO1'(AI1)   * gAO0''(AI0)
	//// The patterns that emerge for cycle of length n:
	//// 1. The derivative is a sum of n terms of n products
	//// 2. Each term is multiplied by 2^i, i is the index of the leg
	//// 3. For the i-th term, the i-th product is a second derivative
	//// 4. The amount in for product i is getRouteAmountOut for subroute upto not including i
	//secondPrime := 0.
	//for i, _ := range route {
	//	term := math.Pow(2., float64(i))
	//	for k, leg := range route {
	//		if k == i {
	//			if reserves[k][2] == nil {
	//				term *= getAmountOutSecondPrime(amountIn, reserves[k][0], reserves[k][1], fees[k][0])
	//			} else {
	//				term *= getAmountOutMultiSecondPrime(
	//					amountsIn[k], leg.Root, fees[k][0], fees[k][1], reserves[k])
	//			}
	//		}
	//		if reserves[k][2] == nil {
	//			term *= getAmountOutPrime(amountIn, reserves[k][0], reserves[k][1], fees[k][0])
	//		} else {
	//			term *= getAmountOutMultiPrime(amountsIn[k], leg.Root, fees[k][0], fees[k][1], reserves[k])
	//		}
	//	}
	//	secondPrime += term
	//}
	//return secondPrime
}

// Secant method to calculate optimal amountIn
func (d *Dexter) calcOptimalAmountInSecant(
	amountIn *big.Int, route []*MultiLeg, poolsInfoOverride map[common.Address]*PoolInfo) (bool, *big.Int) {
	tenToSix := big.NewInt(1000000)
	ten := big.NewInt(10)
	two := big.NewInt(2)
	ninenine := big.NewInt(99)
	hundred := big.NewInt(100)
	amountIn1 := new(big.Int).Sub(amountIn, tenToSix)
	amountIn2 := new(big.Int)
	var step, prime, prime1 float64
	tolerance := math.Pow(10., 12) // 6 (18-12) token decimals of accuracy
	for i := 0; i <= 40; i++ {
		// fmt.Printf("\n")
		prime, prime1 = d.getRouteAmountOutPrime(route, amountIn, amountIn1, map[common.Address]*PoolInfo{})
		prime = prime - 1
		prime1 = prime1 - 1
		step = prime1 * (BigIntToFloat(amountIn1) - BigIntToFloat(amountIn)) / (prime1 - prime)
		// fmt.Printf("amountIn: %s, amountIn1: %s\n", amountIn, amountIn1)
		// fmt.Printf("prime: %g, prime1: %g\n", prime, prime1)
		// fmt.Printf("step: %g\n", step)
		if step == math.Inf(-1) {
			amountIn = amountIn.Mul(amountIn1, ten)
			amountIn1 = amountIn1.Div(amountIn2.Mul(amountIn, ninenine), hundred)
		} else if step == math.Inf(1) {
			amountIn = amountIn.Div(amountIn1, two)
			amountIn1 = amountIn1.Div(amountIn2.Mul(amountIn, ninenine), hundred)
		} else {
			amountIn2 = amountIn2.Sub(amountIn1, FloatToBigInt(step))
			amountIn, amountIn1 = amountIn.Set(amountIn1), amountIn1.Set(amountIn2)
		}
		if math.Abs(step) <= tolerance {
			// fmt.Printf("Converged in %d iterations\n", i)
			return true, amountIn1
		}
	}
	return false, amountIn1
}

// // Newton's method to calculate optimal amountIn
// func (d *Dexter) calcOptimalAmountInNewton(
// 	amountIn *big.Int, route []*MultiLeg, poolsInfoOverride map[common.Address]*PoolInfo) (bool, *big.Int) {
// 	step := 0.
// 	tolerance := math.Pow(10., 12) // 6 (18-12) token decimals of accuracy
// 	for i := 0; i <= 100; i++ {
// 		// fmt.Printf("\n")
// 		prime := d.getRouteAmountOutPrime(route, amountIn, poolsInfoOverride) - 1
// 		secondPrime := d.getRouteAmountOutSecondPrime(route, amountIn, poolsInfoOverride)
// 		step = (prime) / secondPrime
// 		fmt.Printf("amountIn: %s, prime: %g, 2ndPrime: %g, step: %g\n", amountIn, prime, secondPrime, step)
// 		profit := BigIntToFloat(new(big.Int).Sub(
// 			d.getRouteAmountOut(route, amountIn, poolsInfoOverride), amountIn)) / math.Pow(10, 18)
// 		fmt.Printf("profit: %g\n", profit)
// 		amountIn = amountIn.Sub(amountIn, FloatToBigInt(step))
// 		if math.Abs(step) <= tolerance {
// 			// fmt.Printf("Converged in %d iterations\n", i)
// 			return true, amountIn
// 			break
// 		}
// 	}
// 	return false, amountIn
// }

func (d *Dexter) getRouteOptimalAmountIn(route []*MultiLeg, poolsInfoOverride map[common.Address]*PoolInfo) *big.Int {
	return nil
	// startPoolInfo := d.getPoolInfo(poolsInfoOverride, route[0].PoolAddrs[0])
	// leftAmount, rightAmount := new(big.Int), new(big.Int)
	// if bytes.Compare(startPoolInfo.Tokens[0].Bytes(), route[0].From.Bytes()) == 0 {
	// 	leftAmount.Set(startPoolInfo.Reserves[0])
	// 	rightAmount.Set(startPoolInfo.Reserves[1])
	// } else {
	// 	leftAmount.Set(startPoolInfo.Reserves[1])
	// 	rightAmount.Set(startPoolInfo.Reserves[0])
	// }
	// // log.Info("Starting getRouteOptimalAmountIn", "leftAmount", leftAmount, "rightAmount", rightAmount, "reserves[0]", startPoolInfo.reserves[0], "reserves[1]", startPoolInfo.reserves[1], "startPoolInfo", *startPoolInfo)
	// r1 := startPoolInfo.FeeNumerator
	// tenToSix := big.NewInt(int64(1e6))
	// for _, leg := range route[1:] {
	// 	poolInfo := d.getPoolInfo(poolsInfoOverride, leg.PoolAddrs[0])
	// 	var reserveFrom, reserveTo *big.Int
	// 	if bytes.Compare(poolInfo.Tokens[0].Bytes(), leg.From.Bytes()) == 0 {
	// 		reserveFrom, reserveTo = poolInfo.Reserves[0], poolInfo.Reserves[1]
	// 	} else {
	// 		reserveTo, reserveFrom = poolInfo.Reserves[0], poolInfo.Reserves[1]
	// 	}
	// 	legFee := poolInfo.FeeNumerator
	// 	den := new(big.Int).Mul(rightAmount, legFee)
	// 	den = den.Div(den, tenToSix)
	// 	den = den.Add(den, reserveFrom)
	// 	// log.Info("getRouteOptimalAmountIn step", "leftAmount", leftAmount, "rightAmount", rightAmount, "reserveFrom", reserveFrom, "reserveTo", reserveTo, "reserves[0]", poolInfo.Reserves[0], "reserves[1]", poolInfo.Reserves[1], "den", den, "from", leg.From.Bytes()[:2], "Tokens[0]", poolInfo.Tokens[0].Bytes()[:2])
	// 	leftAmount = leftAmount.Mul(leftAmount, reserveFrom)
	// 	leftAmount = leftAmount.Div(leftAmount, den)
	// 	rightAmount = rightAmount.Mul(rightAmount, reserveTo)
	// 	rightAmount = rightAmount.Mul(rightAmount, legFee)
	// 	rightAmount = rightAmount.Div(rightAmount, tenToSix)
	// 	rightAmount = rightAmount.Div(rightAmount, den)
	// }
	// // log.Info("Computed left and right", "leftAmount", leftAmount, "rightAmount", rightAmount, "lbits", leftAmount.BitLen(), "rbits", rightAmount.BitLen())
	// amountIn := new(big.Int).Mul(rightAmount, leftAmount)
	// amountIn = amountIn.Mul(amountIn, r1)
	// amountIn = amountIn.Div(amountIn, tenToSix)
	// amountIn = amountIn.Sqrt(amountIn)
	// amountIn = amountIn.Sub(amountIn, leftAmount)
	// amountIn = amountIn.Mul(amountIn, tenToSix)
	// amountIn = amountIn.Div(amountIn, r1)
	// return amountIn
}
