package curve_factory

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

// abigen --abi ~/dexter/contract/contracts/external/curve/curve-factory/build/contracts/FactorySidechainsABI.json --pkg curve_factory --out curve_factory/curve_factory.go

const ContractABI = "[{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"base_pool\",\"type\":\"address\"}],\"name\":\"BasePoolAdded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"coins\",\"type\":\"address[4]\"},{\"indexed\":false,\"name\":\"A\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"fee\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"deployer\",\"type\":\"address\"}],\"name\":\"PlainPoolDeployed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"coin\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"base_pool\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"A\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"fee\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"deployer\",\"type\":\"address\"}],\"name\":\"MetaPoolDeployed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"pool\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"gauge\",\"type\":\"address\"}],\"name\":\"LiquidityGaugeDeployed\",\"type\":\"event\"},{\"inputs\":[{\"name\":\"_fee_receiver\",\"type\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"gas\":8716,\"inputs\":[{\"name\":\"_base_pool\",\"type\":\"address\"}],\"name\":\"metapool_implementations\",\"outputs\":[{\"name\":\"\",\"type\":\"address[10]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"name\":\"_from\",\"type\":\"address\"},{\"name\":\"_to\",\"type\":\"address\"}],\"name\":\"find_pool_for_coins\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"name\":\"_from\",\"type\":\"address\"},{\"name\":\"_to\",\"type\":\"address\"},{\"name\":\"i\",\"type\":\"uint256\"}],\"name\":\"find_pool_for_coins\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":1363,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_base_pool\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":1399,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_n_coins\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":2601,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_meta_n_coins\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"},{\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":3964,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_coins\",\"outputs\":[{\"name\":\"\",\"type\":\"address[4]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":10945,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_underlying_coins\",\"outputs\":[{\"name\":\"\",\"type\":\"address[8]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":8485,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_decimals\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256[4]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":11930,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_underlying_decimals\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256[8]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":2581,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_metapool_rates\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256[2]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":9435,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_balances\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256[4]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":20433,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_underlying_balances\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256[8]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":1735,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_A\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":3021,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_fees\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"},{\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":6635,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_admin_balances\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256[4]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":18943,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"},{\"name\":\"_from\",\"type\":\"address\"},{\"name\":\"_to\",\"type\":\"address\"}],\"name\":\"get_coin_indices\",\"outputs\":[{\"name\":\"\",\"type\":\"int128\"},{\"name\":\"\",\"type\":\"int128\"},{\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":1789,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_gauge\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":1819,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_implementation_address\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":1852,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"is_meta\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":2850,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_pool_asset_type\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":2880,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_fee_receiver\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"name\":\"_name\",\"type\":\"string\"},{\"name\":\"_symbol\",\"type\":\"string\"},{\"name\":\"_coins\",\"type\":\"address[4]\"},{\"name\":\"_A\",\"type\":\"uint256\"},{\"name\":\"_fee\",\"type\":\"uint256\"}],\"name\":\"deploy_plain_pool\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"name\":\"_name\",\"type\":\"string\"},{\"name\":\"_symbol\",\"type\":\"string\"},{\"name\":\"_coins\",\"type\":\"address[4]\"},{\"name\":\"_A\",\"type\":\"uint256\"},{\"name\":\"_fee\",\"type\":\"uint256\"},{\"name\":\"_asset_type\",\"type\":\"uint256\"}],\"name\":\"deploy_plain_pool\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"name\":\"_name\",\"type\":\"string\"},{\"name\":\"_symbol\",\"type\":\"string\"},{\"name\":\"_coins\",\"type\":\"address[4]\"},{\"name\":\"_A\",\"type\":\"uint256\"},{\"name\":\"_fee\",\"type\":\"uint256\"},{\"name\":\"_asset_type\",\"type\":\"uint256\"},{\"name\":\"_implementation_idx\",\"type\":\"uint256\"}],\"name\":\"deploy_plain_pool\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"name\":\"_base_pool\",\"type\":\"address\"},{\"name\":\"_name\",\"type\":\"string\"},{\"name\":\"_symbol\",\"type\":\"string\"},{\"name\":\"_coin\",\"type\":\"address\"},{\"name\":\"_A\",\"type\":\"uint256\"},{\"name\":\"_fee\",\"type\":\"uint256\"}],\"name\":\"deploy_metapool\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"name\":\"_base_pool\",\"type\":\"address\"},{\"name\":\"_name\",\"type\":\"string\"},{\"name\":\"_symbol\",\"type\":\"string\"},{\"name\":\"_coin\",\"type\":\"address\"},{\"name\":\"_A\",\"type\":\"uint256\"},{\"name\":\"_fee\",\"type\":\"uint256\"},{\"name\":\"_implementation_idx\",\"type\":\"uint256\"}],\"name\":\"deploy_metapool\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"gas\":86201,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"deploy_gauge\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"gas\":905111,\"inputs\":[{\"name\":\"_base_pool\",\"type\":\"address\"},{\"name\":\"_fee_receiver\",\"type\":\"address\"},{\"name\":\"_asset_type\",\"type\":\"uint256\"},{\"name\":\"_implementations\",\"type\":\"address[10]\"}],\"name\":\"add_base_pool\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"gas\":366472,\"inputs\":[{\"name\":\"_base_pool\",\"type\":\"address\"},{\"name\":\"_implementations\",\"type\":\"address[10]\"}],\"name\":\"set_metapool_implementations\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"gas\":365387,\"inputs\":[{\"name\":\"_n_coins\",\"type\":\"uint256\"},{\"name\":\"_implementations\",\"type\":\"address[10]\"}],\"name\":\"set_plain_implementations\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"gas\":37055,\"inputs\":[{\"name\":\"_gauge_implementation\",\"type\":\"address\"}],\"name\":\"set_gauge_implementation\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"gas\":40610,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"},{\"name\":\"_gauge\",\"type\":\"address\"}],\"name\":\"set_gauge\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"gas\":1136975,\"inputs\":[{\"name\":\"_pools\",\"type\":\"address[32]\"},{\"name\":\"_asset_types\",\"type\":\"uint256[32]\"}],\"name\":\"batch_set_pool_asset_type\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"gas\":37145,\"inputs\":[{\"name\":\"_addr\",\"type\":\"address\"}],\"name\":\"commit_transfer_ownership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"gas\":57096,\"inputs\":[],\"name\":\"accept_transfer_ownership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"gas\":38426,\"inputs\":[{\"name\":\"_manager\",\"type\":\"address\"}],\"name\":\"set_manager\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"gas\":37500,\"inputs\":[{\"name\":\"_base_pool\",\"type\":\"address\"},{\"name\":\"_fee_receiver\",\"type\":\"address\"}],\"name\":\"set_fee_receiver\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"gas\":6210,\"inputs\":[],\"name\":\"convert_metapool_fees\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"gas\":2138,\"inputs\":[],\"name\":\"admin\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":2168,\"inputs\":[],\"name\":\"future_admin\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":2198,\"inputs\":[],\"name\":\"manager\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":2273,\"inputs\":[{\"name\":\"arg0\",\"type\":\"uint256\"}],\"name\":\"pool_list\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":2258,\"inputs\":[],\"name\":\"pool_count\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":2333,\"inputs\":[{\"name\":\"arg0\",\"type\":\"uint256\"}],\"name\":\"base_pool_list\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":2318,\"inputs\":[],\"name\":\"base_pool_count\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":2508,\"inputs\":[{\"name\":\"arg0\",\"type\":\"uint256\"},{\"name\":\"arg1\",\"type\":\"uint256\"}],\"name\":\"plain_implementations\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":2378,\"inputs\":[],\"name\":\"fee_receiver\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":2408,\"inputs\":[],\"name\":\"gauge_implementation\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]"

// "

var (
	sAbi, _ = abi.JSON(strings.NewReader(ContractABI))
)

func GetFees(poolAddr *common.Address) []byte {
	data, err := sAbi.Pack("get_fees", poolAddr)
	if err != nil {
		log.Error("Error packing get_fees", "err", err)
	}
	return data
}

func UnpackGetFee(data []byte) *big.Int {
	out, err := sAbi.Unpack("get_fees", data)
	if err != nil {
		log.Error("Error unpacking get_fees", "err", err)
	}
	return out[0].(*big.Int)
}

func GetAmplificationParameter(poolAddr common.Address) []byte {
	data, err := sAbi.Pack("get_A", poolAddr)
	if err != nil {
		log.Error("Error packing get_A", "err", err)
	}
	return data
}

func GetBalances(poolAddr common.Address) []byte {
	data, err := sAbi.Pack("get_balances", poolAddr)
	if err != nil {
		log.Error("Error packing get_balances", "err", err)
	}
	return data
}

func UnpackGetBalances(data []byte) []*big.Int {
	out, err := sAbi.Unpack("get_balances", data)
	if err != nil {
		log.Error("Error unpacking get_balances", "err", err)
	}
	b := out[0].([4]*big.Int)
	return b[:]
}

func GetUnderlyingBalances(poolAddr common.Address) []byte {
	data, err := sAbi.Pack("get_underlying_balances", poolAddr)
	if err != nil {
		log.Error("Error packing get_underlying_balances", "err", err)
	}
	return data
}

func UnpackGetUnderlyingBalances(data []byte) []*big.Int {
	out, err := sAbi.Unpack("get_underlying_balances", data)
	if err != nil {
		log.Error("Error unpacking get_underlying_balances", "err", err)
	}
	b := out[0].([8]*big.Int)
	return b[:]
}

func GetCoins(poolAddr common.Address) []byte {
	data, err := sAbi.Pack("get_coins", poolAddr)
	if err != nil {
		log.Error("Error packing get_coins", "err", err)
	}
	return data
}

func UnpackGetCoins(data []byte) []common.Address {
	out, err := sAbi.Unpack("get_coins", data)
	if err != nil {
		log.Error("Error unpacking get_coins", "err", err)
	}
	c := out[0].([4]common.Address)
	return c[:]
}

func GetUnderlyingCoins(poolAddr common.Address) []byte {
	data, err := sAbi.Pack("get_underlying_coins", poolAddr)
	if err != nil {
		log.Error("Error packing get_coins", "err", err)
	}
	return data
}

func UnpackGetUnderlyingCoins(data []byte) []common.Address {
	out, err := sAbi.Unpack("get_underlying_coins", data)
	if err != nil {
		log.Error("Error unpacking get_coins", "err", err)
	}
	c := out[0].([8]common.Address)
	return c[:]
}
