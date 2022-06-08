package dexter

import (
	"github.com/ethereum/go-ethereum/common"
)

var (
	specialMethods = []Method{
		Method{10, 242, 16, 161},
		Method{108, 181, 140, 147}, // leave(address vault, uint256 share)
		Method{112, 250, 226, 13},
		Method{116, 58, 203, 128},
		Method{124, 2, 82, 0},
		Method{126, 27, 149, 7},
		Method{127, 243, 106, 181}, // []byte{0x7f, 0xf3, 0x6a, 0xb5},
		Method{136, 3, 219, 238},   // []byte{0x88, 0x03, 0xdb, 0xee},
		Method{139, 219, 57, 19},   // BalancerV2
		Method{14, 92, 1, 30},
		Method{119, 55, 13, 98},   // Tarot deleverage(address underlying, uint256 redeemTokens, uint256 amountAMin, uint256 amountBMin, uint256 deadline, bytes permitData)
		Method{148, 91, 206, 201}, // BalancerV2 swap
		Method{152, 208, 241, 112},
		Method{162, 140, 54, 27},  // beefOut(address beefyVault, uint256 withdrawAmount)
		Method{169, 78, 120, 239}, // AugustusSwapper
		Method{172, 150, 80, 216}, // Statera: multicall
		Method{184, 123, 11, 76},  // Gelato exec
		Method{185, 92, 172, 40},  // BalancerV2
		Method{194, 113, 126, 85}, // PaintSwap pay(bool _payingInBrush, address[] _path)
		Method{191, 179, 92, 38},  // 0x01c892c7b9fb9b1acd3584be12efff91278d5230
		Method{197, 134, 4, 101},  // Swing trader
		Method{24, 203, 175, 229}, // []byte{0x18, 0xcb, 0xaf, 0xe5},
		Method{202, 198, 55, 200},
		Method{207, 16, 129, 219}, // 0x256410f7b8951c879111dba3efa756ad1f2b402e
		Method{209, 208, 255, 73},
		Method{215, 46, 247, 113}, // work(uint256 id, address worker, uint256 principalAmount, uint256 loan, uint256 maxReturn, bytes data) (interest bearing fantom)
		Method{222, 95, 98, 104},
		Method{222, 217, 56, 42},
		Method{226, 187, 177, 88}, // deposit
		Method{225, 22, 210, 2},
		Method{230, 90, 1, 23}, // earn(uint256 _pid)
		Method{232, 227, 55, 0},
		Method{234, 253, 131, 50}, // Swing trader
		Method{236, 250, 49, 29},
		Method{243, 5, 215, 25},
		Method{243, 188, 168, 114}, // Swing trader
		Method{245, 208, 123, 96},  // beefIn(address beefyVault, uint256 tokenAmountOutMin, address tokenIn, uint256 tokenInAmount)
		Method{251, 59, 219, 65},   // []byte{0xfb, 0x3b, 0xdb, 0x41},
		Method{253, 181, 160, 62},  // Reinvest
		Method{253, 181, 254, 252}, // earn(address _bountyHunter)
		Method{255, 194, 88, 15},
		Method{28, 255, 121, 205},
		Method{33, 149, 153, 92},
		Method{46, 211, 145, 196},
		Method{47, 89, 103, 235},
		Method{56, 237, 23, 57},   // []byte{0x38, 0xed, 0x17, 0x39},
		Method{65, 85, 101, 176},  // []byte{0x38, 0xed, 0x17, 0x39},
		Method{70, 65, 37, 125},   // Harvest
		Method{70, 198, 123, 109}, // AugustusSwapper
		Method{74, 37, 217, 74},   // []byte{0x4a, 0x25, 0xd9, 0x4a},
		Method{81, 201, 207, 145}, // beefOutAndSwap(address beefyVault, uint256 withdrawAmount, address desiredToken, uint256 desiredTokenOutMin)
		Method{82, 187, 190, 41},  // BalancerV2
		Method{84, 227, 243, 27},  // AugustusSwapper
		Method{92, 17, 215, 149},  // swapExactTokensForTokensSupportingFeeOnTransferTokens
	}
)

func runEvents(m *Methodist, method Method, counts []int) {
	for t := 0; t < len(counts); t++ {
		for i := 0; i < counts[t]; i++ {
			m.Record(MethodEvent{M: method, Type: MethodEventType(t), TxHash: common.HexToHash("0x000")})
		}
	}
}

// func TestMethodist(t *testing.T) {
// 	m := NewMethodist("/home/ubuntu/dexter/carb/", 1000*time.Millisecond)
// 	m1 := Method{1, 0, 0, 0}
// 	runEvents(m, m1, []int{4, 5, 8, 3, 1})
// 	m2 := Method{2, 0, 0, 0}
// 	runEvents(m, m2, []int{1, 3, 1, 1, 0})
// 	m3 := Method{3, 0, 0, 0}
// 	runEvents(m, m3, []int{1, 1, 5, 5, 5})
// 	// time.Sleep(200 * time.Millisecond)
// }

// func TestLoad(t *testing.T) {
// 	m := NewMethodist("/home/ubuntu/dexter/carb/", 1000*time.Millisecond)
// 	white, black := m.GetLists()
// 	whiteMap := make(map[Method]struct{})
// 	blackMap := make(map[Method]struct{})
// 	for _, m := range white {
// 		whiteMap[m] = struct{}{}
// 	}
// 	for _, m := range black {
// 		blackMap[m] = struct{}{}
// 	}
// 	fmt.Printf("Whitelist: %v\n", white)
// 	fmt.Printf("Blacklist: %v\n", black)
// 	for _, m := range specialMethods {
// 		if _, inWhite := whiteMap[m]; inWhite {
// 			fmt.Printf("Method in whitelist: %v\n", m)
// 		} else if _, inBlack := blackMap[m]; inBlack {
// 			fmt.Printf("Method in blacklist: %v\n", m)
// 		} else {
// 			fmt.Printf("Method in no list: %v\n", m)
// 		}
// 	}
// }
