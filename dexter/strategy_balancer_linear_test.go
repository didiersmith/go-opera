package dexter

import (
	"fmt"
	"math/big"
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

	t.Run("load Json", func(t *testing.T) {
		assert := assert.New(t)
		assert.Greater(len(b.interestedPools), 0, "No intereseted pools")
		assert.Greater(len(b.routeCache.Routes), 0, "No routes in routeCache")
		assert.Greater(len(b.routeCache.PoolToRouteIdxs), 0, "No poolToRouteIdxs in routeCache")
	})

	t.Run("getAmountOutBalancer", func(t *testing.T) {
		oneFtm := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
		pool := &PoolInfo{
			Reserves: []*big.Int{new(big.Int).Mul(big.NewInt(1000),
				oneFtm), new(big.Int).Mul(big.NewInt(2000), oneFtm)},
			Tokens: []common.Address{common.HexToAddress("0x100"), common.HexToAddress("0x200")},
			Weights: []*big.Int{new(big.Int).Div(oneFtm, big.NewInt(2)),
				new(big.Int).Div(oneFtm, big.NewInt(2))},
			FeeNumerator: big.NewInt(997000),
		}
		amountOut := b.getAmountOutBalancer(
			new(big.Int).Exp(big.NewInt(10), big.NewInt(10), nil),
			pool.Reserves[0], pool.Reserves[1], pool.Weights[0], pool.Weights[1], pool.FeeNumerator)
		fmt.Printf("AmountOut: %s\n", amountOut)
	})

}
