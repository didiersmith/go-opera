package curve_registry

// abigen --abi ~/dexter/contract/contracts/external/curve/curve-pool-registry/build/contracts/RegistryABI.json --pkg curve_registry --out curve_registry/curve_registry.go
import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

const ContractABI = "[{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"pool\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"rate_method_id\",\"type\":\"bytes\"}],\"name\":\"PoolAdded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"pool\",\"type\":\"address\"}],\"name\":\"PoolRemoved\",\"type\":\"event\"},{\"inputs\":[{\"name\":\"_address_provider\",\"type\":\"address\"},{\"name\":\"_gauge_controller\",\"type\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"inputs\":[{\"name\":\"_from\",\"type\":\"address\"},{\"name\":\"_to\",\"type\":\"address\"}],\"name\":\"find_pool_for_coins\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"name\":\"_from\",\"type\":\"address\"},{\"name\":\"_to\",\"type\":\"address\"},{\"name\":\"i\",\"type\":\"uint256\"}],\"name\":\"find_pool_for_coins\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":1521,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_n_coins\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256[2]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":12102,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_coins\",\"outputs\":[{\"name\":\"\",\"type\":\"address[8]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":12194,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_underlying_coins\",\"outputs\":[{\"name\":\"\",\"type\":\"address[8]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":7874,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_decimals\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256[8]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":7966,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_underlying_decimals\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256[8]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":36992,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_rates\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256[8]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":20157,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_gauges\",\"outputs\":[{\"name\":\"\",\"type\":\"address[10]\"},{\"name\":\"\",\"type\":\"int128[10]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":16583,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_balances\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256[8]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":162842,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_underlying_balances\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256[8]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":1927,\"inputs\":[{\"name\":\"_token\",\"type\":\"address\"}],\"name\":\"get_virtual_price_from_lp_token\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":1045,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_A\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":6305,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_parameters\",\"outputs\":[{\"name\":\"A\",\"type\":\"uint256\"},{\"name\":\"future_A\",\"type\":\"uint256\"},{\"name\":\"fee\",\"type\":\"uint256\"},{\"name\":\"admin_fee\",\"type\":\"uint256\"},{\"name\":\"future_fee\",\"type\":\"uint256\"},{\"name\":\"future_admin_fee\",\"type\":\"uint256\"},{\"name\":\"future_owner\",\"type\":\"address\"},{\"name\":\"initial_A\",\"type\":\"uint256\"},{\"name\":\"initial_A_time\",\"type\":\"uint256\"},{\"name\":\"future_A_time\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":1450,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_fees\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256[2]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":36454,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_admin_balances\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256[8]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":27131,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"},{\"name\":\"_from\",\"type\":\"address\"},{\"name\":\"_to\",\"type\":\"address\"}],\"name\":\"get_coin_indices\",\"outputs\":[{\"name\":\"\",\"type\":\"int128\"},{\"name\":\"\",\"type\":\"int128\"},{\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":32004,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"},{\"name\":\"_from\",\"type\":\"address\"},{\"name\":\"_to\",\"type\":\"address\"}],\"name\":\"estimate_gas_used\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":1900,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"is_meta\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":8323,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_pool_name\",\"outputs\":[{\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":1951,\"inputs\":[{\"name\":\"_coin\",\"type\":\"address\"}],\"name\":\"get_coin_swap_count\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":2090,\"inputs\":[{\"name\":\"_coin\",\"type\":\"address\"},{\"name\":\"_index\",\"type\":\"uint256\"}],\"name\":\"get_coin_swap_complement\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":2011,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"get_pool_asset_type\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":61485845,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"},{\"name\":\"_n_coins\",\"type\":\"uint256\"},{\"name\":\"_lp_token\",\"type\":\"address\"},{\"name\":\"_rate_info\",\"type\":\"bytes32\"},{\"name\":\"_decimals\",\"type\":\"uint256\"},{\"name\":\"_underlying_decimals\",\"type\":\"uint256\"},{\"name\":\"_has_initial_A\",\"type\":\"bool\"},{\"name\":\"_is_v1\",\"type\":\"bool\"},{\"name\":\"_name\",\"type\":\"string\"}],\"name\":\"add_pool\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"gas\":31306062,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"},{\"name\":\"_n_coins\",\"type\":\"uint256\"},{\"name\":\"_lp_token\",\"type\":\"address\"},{\"name\":\"_rate_info\",\"type\":\"bytes32\"},{\"name\":\"_decimals\",\"type\":\"uint256\"},{\"name\":\"_use_rates\",\"type\":\"uint256\"},{\"name\":\"_has_initial_A\",\"type\":\"bool\"},{\"name\":\"_is_v1\",\"type\":\"bool\"},{\"name\":\"_name\",\"type\":\"string\"}],\"name\":\"add_pool_without_underlying\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"},{\"name\":\"_n_coins\",\"type\":\"uint256\"},{\"name\":\"_lp_token\",\"type\":\"address\"},{\"name\":\"_decimals\",\"type\":\"uint256\"},{\"name\":\"_name\",\"type\":\"string\"}],\"name\":\"add_metapool\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"},{\"name\":\"_n_coins\",\"type\":\"uint256\"},{\"name\":\"_lp_token\",\"type\":\"address\"},{\"name\":\"_decimals\",\"type\":\"uint256\"},{\"name\":\"_name\",\"type\":\"string\"},{\"name\":\"_base_pool\",\"type\":\"address\"}],\"name\":\"add_metapool\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"gas\":779731418758,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"}],\"name\":\"remove_pool\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"gas\":390460,\"inputs\":[{\"name\":\"_addr\",\"type\":\"address[5]\"},{\"name\":\"_amount\",\"type\":\"uint256[2][5]\"}],\"name\":\"set_pool_gas_estimates\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"gas\":392047,\"inputs\":[{\"name\":\"_addr\",\"type\":\"address[10]\"},{\"name\":\"_amount\",\"type\":\"uint256[10]\"}],\"name\":\"set_coin_gas_estimates\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"gas\":72629,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"},{\"name\":\"_estimator\",\"type\":\"address\"}],\"name\":\"set_gas_estimate_contract\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"gas\":400675,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"},{\"name\":\"_liquidity_gauges\",\"type\":\"address[10]\"}],\"name\":\"set_liquidity_gauges\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"gas\":72667,\"inputs\":[{\"name\":\"_pool\",\"type\":\"address\"},{\"name\":\"_asset_type\",\"type\":\"uint256\"}],\"name\":\"set_pool_asset_type\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"gas\":1173447,\"inputs\":[{\"name\":\"_pools\",\"type\":\"address[32]\"},{\"name\":\"_asset_types\",\"type\":\"uint256[32]\"}],\"name\":\"batch_set_pool_asset_type\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"gas\":2048,\"inputs\":[],\"name\":\"address_provider\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":2078,\"inputs\":[],\"name\":\"gauge_controller\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":2217,\"inputs\":[{\"name\":\"arg0\",\"type\":\"uint256\"}],\"name\":\"pool_list\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":2138,\"inputs\":[],\"name\":\"pool_count\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":2168,\"inputs\":[],\"name\":\"coin_count\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":2307,\"inputs\":[{\"name\":\"arg0\",\"type\":\"uint256\"}],\"name\":\"get_coin\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":2443,\"inputs\":[{\"name\":\"arg0\",\"type\":\"address\"}],\"name\":\"get_pool_from_lp_token\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":2473,\"inputs\":[{\"name\":\"arg0\",\"type\":\"address\"}],\"name\":\"get_lp_token\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"gas\":2288,\"inputs\":[],\"name\":\"last_updated\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]"

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
	// log.Info("fee", "fee", out[0].([]*big.Int)[0])
	return out[0].([2]*big.Int)[0]
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
	c := out[0].([8]common.Address)
	return c[:]
}
