package fish4_lite

// abigen --pkg fish4 --abi=Fish4Abi.json
import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

const ContractABI = "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"vault\",\"type\":\"address\"},{\"internalType\":\"address[]\",\"name\":\"_members\",\"type\":\"address[]\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"member\",\"type\":\"address\"}],\"name\":\"addMember\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address[]\",\"name\":\"_members\",\"type\":\"address[]\"}],\"name\":\"addMembers\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"}],\"name\":\"dumpTokens\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"maxAmountIn\",\"type\":\"uint256\"},{\"components\":[{\"internalType\":\"address\",\"name\":\"pair\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"token0\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"token1\",\"type\":\"address\"},{\"internalType\":\"uint112\",\"name\":\"feeNumerator\",\"type\":\"uint112\"},{\"internalType\":\"bool\",\"name\":\"fromToken0\",\"type\":\"bool\"}],\"internalType\":\"structFish4.LinearSwapCommand[]\",\"name\":\"path\",\"type\":\"tuple[]\"}],\"name\":\"getAmountsOutLinear\",\"outputs\":[{\"internalType\":\"uint256[]\",\"name\":\"amountsOut\",\"type\":\"uint256[]\"},{\"internalType\":\"uint256\",\"name\":\"optimalAmountIn\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"amountIn\",\"type\":\"uint256\"},{\"components\":[{\"internalType\":\"address\",\"name\":\"pair\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"token0\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"token1\",\"type\":\"address\"},{\"internalType\":\"uint112\",\"name\":\"fraction\",\"type\":\"uint112\"},{\"internalType\":\"uint112\",\"name\":\"feeNumerator\",\"type\":\"uint112\"},{\"internalType\":\"bool\",\"name\":\"fromToken0\",\"type\":\"bool\"},{\"internalType\":\"uint8\",\"name\":\"swapTo\",\"type\":\"uint8\"}],\"internalType\":\"structFish4.SwapCommand[]\",\"name\":\"path\",\"type\":\"tuple[]\"}],\"name\":\"getAmountsOutMulti\",\"outputs\":[{\"internalType\":\"uint256[]\",\"name\":\"amounts\",\"type\":\"uint256[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"contractIERC20[]\",\"name\":\"tokens\",\"type\":\"address[]\"},{\"internalType\":\"uint256[]\",\"name\":\"amounts\",\"type\":\"uint256[]\"},{\"internalType\":\"uint256[]\",\"name\":\"feeAmounts\",\"type\":\"uint256[]\"},{\"internalType\":\"bytes\",\"name\":\"userData\",\"type\":\"bytes\"}],\"name\":\"receiveFlashLoan\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"maxAmountIn\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"minProfit\",\"type\":\"uint256\"},{\"components\":[{\"internalType\":\"address\",\"name\":\"pair\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"token0\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"token1\",\"type\":\"address\"},{\"internalType\":\"uint112\",\"name\":\"feeNumerator\",\"type\":\"uint112\"},{\"internalType\":\"bool\",\"name\":\"fromToken0\",\"type\":\"bool\"}],\"internalType\":\"structFish4.LinearSwapCommand[]\",\"name\":\"path\",\"type\":\"tuple[]\"},{\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"}],\"name\":\"swapLinear\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"amountIn\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"minProfit\",\"type\":\"uint256\"},{\"components\":[{\"internalType\":\"address\",\"name\":\"pair\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"token0\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"token1\",\"type\":\"address\"},{\"internalType\":\"uint112\",\"name\":\"fraction\",\"type\":\"uint112\"},{\"internalType\":\"uint112\",\"name\":\"feeNumerator\",\"type\":\"uint112\"},{\"internalType\":\"bool\",\"name\":\"fromToken0\",\"type\":\"bool\"},{\"internalType\":\"uint8\",\"name\":\"swapTo\",\"type\":\"uint8\"}],\"internalType\":\"structFish4.SwapCommand[]\",\"name\":\"path\",\"type\":\"tuple[]\"},{\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"}],\"name\":\"swapMultiPath\",\"outputs\":[{\"internalType\":\"uint256[]\",\"name\":\"amountsOut\",\"type\":\"uint256[]\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"withdrawTokens\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]"

var (
	sAbi, _ = abi.JSON(strings.NewReader(ContractABI))
)

// Methods

type LinearSwapCommand struct {
	Pair         common.Address
	Token0       common.Address
	Token1       common.Address
	FeeNumerator *big.Int
	FromToken0   bool
}

func SwapLinear(maxAmountIn, minProfit *big.Int, path []LinearSwapCommand, to common.Address) []byte {
	data, err := sAbi.Pack("swapLinear", maxAmountIn, minProfit, path, to)
	if err != nil {
		log.Error("Error packing swapLinear", "err", err)
	}
	return data
}
