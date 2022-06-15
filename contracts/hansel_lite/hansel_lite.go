package hansel_lite

// abigen --pkg fish4 --abi=Fish4Abi.json
import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

const ContractABI = "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"vault\",\"type\":\"address\"},{\"internalType\":\"contractIPoolAddressesProvider\",\"name\":\"provider\",\"type\":\"address\"},{\"internalType\":\"address[]\",\"name\":\"_members\",\"type\":\"address[]\"},{\"components\":[{\"internalType\":\"address\",\"name\":\"addr\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"feeNumerator\",\"type\":\"uint256\"},{\"internalType\":\"enumHansel.FactoryType\",\"name\":\"factoryType\",\"type\":\"uint8\"}],\"internalType\":\"structHansel.uniswapFactoryDetails[]\",\"name\":\"_uniswapFactories\",\"type\":\"tuple[]\"},{\"internalType\":\"address[]\",\"name\":\"_midpoints\",\"type\":\"address[]\"},{\"internalType\":\"address\",\"name\":\"_baseToken\",\"type\":\"address\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"inputs\":[],\"name\":\"ADDRESSES_PROVIDER\",\"outputs\":[{\"internalType\":\"contractIPoolAddressesProvider\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"POOL\",\"outputs\":[{\"internalType\":\"contractIPool\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"balanceIn\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"weightIn\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"balanceOut\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"weightOut\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"amountIn\",\"type\":\"uint256\"}],\"name\":\"_calcOutGivenIn\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"}],\"internalType\":\"structHansel.FlashLoanSource[]\",\"name\":\"_sources\",\"type\":\"tuple[]\"}],\"name\":\"addFlashLoanSources\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"member\",\"type\":\"address\"}],\"name\":\"addMember\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address[]\",\"name\":\"_members\",\"type\":\"address[]\"}],\"name\":\"addMembers\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address[]\",\"name\":\"_midpoints\",\"type\":\"address[]\"}],\"name\":\"addMidpoints\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"address\",\"name\":\"addr\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"feeNumerator\",\"type\":\"uint256\"},{\"internalType\":\"enumHansel.FactoryType\",\"name\":\"factoryType\",\"type\":\"uint8\"}],\"internalType\":\"structHansel.uniswapFactoryDetails[]\",\"name\":\"_uniswapFactories\",\"type\":\"tuple[]\"}],\"name\":\"addUniswapFactories\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"}],\"name\":\"dumpTokens\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"asset\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"premium\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"initiator\",\"type\":\"address\"},{\"internalType\":\"bytes\",\"name\":\"params\",\"type\":\"bytes\"}],\"name\":\"executeOperation\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"poolId\",\"type\":\"bytes32\"},{\"internalType\":\"address\",\"name\":\"tokenFrom\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"tokenTo\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"feeNumerator\",\"type\":\"uint256\"},{\"internalType\":\"enumHansel.PoolType\",\"name\":\"poolType\",\"type\":\"uint8\"}],\"internalType\":\"structHansel.Breadcrumb[]\",\"name\":\"crumbs\",\"type\":\"tuple[]\"}],\"name\":\"findRoute\",\"outputs\":[{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"poolId\",\"type\":\"bytes32\"},{\"internalType\":\"address\",\"name\":\"tokenFrom\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"tokenTo\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"feeNumerator\",\"type\":\"uint256\"},{\"internalType\":\"enumHansel.PoolType\",\"name\":\"poolType\",\"type\":\"uint8\"}],\"internalType\":\"structHansel.Breadcrumb[]\",\"name\":\"bestRoute\",\"type\":\"tuple[]\"},{\"internalType\":\"int256\",\"name\":\"profit\",\"type\":\"int256\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"initiator\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"fee\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"onFlashLoan\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"contractIERC20[]\",\"name\":\"tokens\",\"type\":\"address[]\"},{\"internalType\":\"uint256[]\",\"name\":\"amounts\",\"type\":\"uint256[]\"},{\"internalType\":\"uint256[]\",\"name\":\"feeAmounts\",\"type\":\"uint256[]\"},{\"internalType\":\"bytes\",\"name\":\"userData\",\"type\":\"bytes\"}],\"name\":\"receiveFlashLoan\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"startAmountIn\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"minProfit\",\"type\":\"uint256\"},{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"poolId\",\"type\":\"bytes32\"},{\"internalType\":\"address\",\"name\":\"tokenFrom\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"tokenTo\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"feeNumerator\",\"type\":\"uint256\"},{\"internalType\":\"enumHansel.PoolType\",\"name\":\"poolType\",\"type\":\"uint8\"}],\"internalType\":\"structHansel.Breadcrumb[]\",\"name\":\"path\",\"type\":\"tuple[]\"},{\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"}],\"name\":\"swapLinear\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"withdrawTokens\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]"

var (
	sAbi, _ = abi.JSON(strings.NewReader(ContractABI))
)

// Methods

type Breadcrumb struct {
	PoolId       [32]byte       `json:poolId`
	TokenFrom    common.Address `json:tokenFrom`
	TokenTo      common.Address `json:tokenTo`
	FeeNumerator *big.Int       `json:feeNumerator`
	PoolType     uint8          `json:poolType`
}

func SwapLinear(startAmountIn, minProfit *big.Int, path []Breadcrumb, to common.Address) []byte {
	data, err := sAbi.Pack("swapLinear", startAmountIn, minProfit, path, to)
	if err != nil {
		log.Error("Error packing swapLinear", "err", err)
	}
	return data
}

func FindRoute(crumbs []Breadcrumb) []byte {
	data, err := sAbi.Pack("findRoute", crumbs)
	if err != nil {
		log.Error("Error packing findRoute", "err", err)
	}
	return data
}

func UnpackFindRoute(data []byte) ([]Breadcrumb, *big.Int) {
	out, err := sAbi.Unpack("findRoute", data)
	if err != nil {
		return nil, nil
	}
	genericPath := out[0].([]struct {
		PoolId       [32]uint8      "json:\"poolId\""
		TokenFrom    common.Address "json:\"tokenFrom\""
		TokenTo      common.Address "json:\"tokenTo\""
		FeeNumerator *big.Int       "json:\"feeNumerator\""
		PoolType     uint8          "json:\"poolType\""
	})
	path := make([]Breadcrumb, len(genericPath))
	for i, c := range genericPath {
		path[i] = Breadcrumb(c)
	}
	return path, out[1].(*big.Int)
}
