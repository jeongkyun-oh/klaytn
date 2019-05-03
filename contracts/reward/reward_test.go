// Copyright 2018 The klaytn Authors
// This file is part of the klaytn library.
//
// The klaytn library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The klaytn library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the klaytn library. If not, see <http://www.gnu.org/licenses/>.

package reward

import (
	"fmt"
	"github.com/ground-x/klaytn/blockchain/types"
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/governance"
	"github.com/ground-x/klaytn/params"
	"github.com/stretchr/testify/assert"
	"math"
	"math/big"
	"testing"
)

type testAccount struct {
	balance *big.Int
}

type testAccounts struct {
	accounts map[common.Address]*testAccount
}

func (ta *testAccounts) AddBalance(addr common.Address, v *big.Int) {
	if account, ok := ta.accounts[addr]; ok {
		account.balance.Add(account.balance, v)
	} else {
		ta.accounts[addr] = &testAccount{new(big.Int).Set(v)}
	}
}

func (ta *testAccounts) GetBalance(addr common.Address) *big.Int {
	account := ta.accounts[addr]
	if account != nil {
		return account.balance
	} else {
		return nil
	}
}

func newTestAccounts() *testAccounts {
	return &testAccounts{
		accounts: make(map[common.Address]*testAccount),
	}
}

var (
	addr1 = common.HexToAddress("0xac5e047d39692be8c81d0724543d5de721d0dd54")
)

func TestParseRewardRatio(t *testing.T) {
	testCases := []struct {
		s       string
		cn      int
		poc     int
		kir     int
		success bool
	}{
		// defaults
		{"34/54/12", 34, 54, 12, true},
		{"3/3/3", 3, 3, 3, true},
		{"10/20/30", 10, 20, 30, true},
		{"///", 0, 0, 0, false},
		{"1//", 0, 0, 0, false},
		{"/1/", 0, 0, 0, false},
		{"//1", 0, 0, 0, false},
		{"1/2/3/4/", 0, 0, 0, false},
		{"3.3/3.3/3.3", 0, 0, 0, false},
	}

	for i := 0; i < len(testCases); i++ {
		cn, poc, kir, error := parseRewardRatio(testCases[i].s)

		// check if the error is nil. It should be same as testCase.success. If not, the test fail
		if (error == nil) != testCases[i].success || cn != testCases[i].cn ||
			poc != testCases[i].poc || kir != testCases[i].kir {
			t.Errorf("test case %v fail. The result is diffrent", testCases[i].s)
			t.Errorf("The parsed cn. Result : %v, Expected : %v", cn, testCases[i].cn)
			t.Errorf("The parsed poc. Result : %v, Expected : %v", poc, testCases[i].poc)
			t.Errorf("The parsed kir. Result : %v, Expected : %v", kir, testCases[i].kir)
		}
	}

}

func TestGetRewardGovernanceParameter(t *testing.T) {
	testCases := []struct {
		blockNumber  int64
		epoch        uint64
		ratio        string
		mintinAmount uint64
	}{
		// defaults
		{1, 30, "34/54/12", 9600000000},
		{365, 360, "60/20/20", 10000},
		{700000, 604800, "30/40/30", 1234567890},
	}

	for i := 0; i < len(testCases); i++ {
		header := &types.Header{Number: big.NewInt(testCases[i].blockNumber)}

		config := &params.ChainConfig{Istanbul: governance.GetDefaultIstanbulConfig(), Governance: governance.GetDefaultGovernanceConfig(params.UseIstanbul)}
		config.Istanbul.Epoch = testCases[i].epoch
		config.Governance.Reward.Ratio = testCases[i].ratio
		config.Governance.Reward.MintingAmount = new(big.Int).SetUint64(testCases[i].mintinAmount)

		governanceParameter := getRewardGovernanceParameters(config, header)

		expectedBlockNumber := uint64(testCases[i].blockNumber) - (uint64(testCases[i].blockNumber) % testCases[i].epoch)
		if governanceParameter.blockNum != expectedBlockNumber {
			t.Errorf("The block number of governanceParameter is diffrent. Result : %v, Expected : %v", governanceParameter.blockNum, expectedBlockNumber)
		}

		cn, poc, kir, _ := parseRewardRatio(testCases[i].ratio)
		cnRatio := big.NewInt(int64(cn))
		pocRatio := big.NewInt(int64(poc))
		kirRatio := big.NewInt(int64(kir))
		totalRatio := big.NewInt(0)
		totalRatio = totalRatio.Add(cnRatio, pocRatio)
		totalRatio = totalRatio.Add(totalRatio, kirRatio)

		if governanceParameter.cnRewardRatio.Cmp(cnRatio) != 0 || governanceParameter.pocRatio.Cmp(pocRatio) != 0 ||
			governanceParameter.kirRatio.Cmp(kirRatio) != 0 || governanceParameter.totalRatio.Cmp(totalRatio) != 0 {
			t.Errorf("The reward ratio in governance parameter is diffrent ")
			t.Errorf("The cn reward ratio. Result : %v, Expected : %v", governanceParameter.cnRewardRatio, cnRatio)
			t.Errorf("The poc reward ratio. Result : %v, Expected : %v", governanceParameter.pocRatio, pocRatio)
			t.Errorf("The kir reward ratio. Result : %v, Expected : %v", governanceParameter.kirRatio, kirRatio)
			t.Errorf("The total reward ratio. Result : %v, Expected : %v", governanceParameter.totalRatio, totalRatio)
		}
	}
}

func TestUpdateGovernanceParameterByEpoch(t *testing.T) {
	// This test is for testing weather governanceParameter is updated by epoch
	// when the block number doesn't pass the epoch from last updated block number,
	// the governanceParameter shouldn't be updated.
	// it is tested by following step
	// 1. update governanceParameter with block number 1 and check if it is updated well
	// 2. update governance parameter with block number before epoch(29 in this test), it should not be updated
	// 3. update governance parameter with block number after epoch(30 in this test), it should be updated

	blockNumber := int64(1)
	epoch := uint64(30)
	cnRatio := new(big.Int).SetUint64(34)
	pocRatio := new(big.Int).SetUint64(54)
	kirRatio := new(big.Int).SetUint64(12)
	mintingAmount := uint64(9600000000)

	//1. update governanceParameter with block number 1 and check if it is updated well
	header := &types.Header{Number: big.NewInt(blockNumber)}
	config := &params.ChainConfig{Istanbul: governance.GetDefaultIstanbulConfig(), Governance: governance.GetDefaultGovernanceConfig(params.UseIstanbul)}

	config.Istanbul.Epoch = epoch
	config.Governance.Reward.Ratio = "34/54/12"
	config.Governance.Reward.MintingAmount = new(big.Int).SetUint64(mintingAmount)

	governanceParameter := getRewardGovernanceParameters(config, header)

	if governanceParameter.blockNum != 0 || governanceParameter.cnRewardRatio.Cmp(cnRatio) != 0 ||
		governanceParameter.pocRatio.Cmp(pocRatio) != 0 || governanceParameter.kirRatio.Cmp(kirRatio) != 0 {
		t.Errorf("GovernanceParameter is diffrent")
	}

	// 2. update governance parameter with block number before epoch(29 in this test), it should not be updated
	header = &types.Header{Number: big.NewInt(29)}
	config.Governance.Reward.Ratio = "1/2/3"

	governanceParameter = getRewardGovernanceParameters(config, header)

	if governanceParameter.blockNum != 0 || governanceParameter.cnRewardRatio.Cmp(cnRatio) != 0 ||
		governanceParameter.pocRatio.Cmp(pocRatio) != 0 || governanceParameter.kirRatio.Cmp(kirRatio) != 0 {
		t.Errorf("GovernanceParameter is diffrent")
	}

	// 3. update governance parameter with block number after epoch(30 in this test), it should be updated
	blockNumber = int64(30)
	cnRatio = new(big.Int).SetUint64(40)
	pocRatio = new(big.Int).SetUint64(30)
	kirRatio = new(big.Int).SetUint64(30)
	mintingAmount = uint64(3000000000)

	header = &types.Header{Number: big.NewInt(blockNumber)}
	config.Governance.Reward.Ratio = "40/30/30"
	config.Governance.Reward.MintingAmount = new(big.Int).SetUint64(mintingAmount)

	governanceParameter = getRewardGovernanceParameters(config, header)

	if governanceParameter.blockNum != 30 || governanceParameter.cnRewardRatio.Cmp(cnRatio) != 0 ||
		governanceParameter.pocRatio.Cmp(pocRatio) != 0 || governanceParameter.kirRatio.Cmp(kirRatio) != 0 {
		t.Errorf("GovernanceParameter is diffrent")
	}
}

// TestBlockRewardWithDefaultGovernance1 tests DistributeBlockReward with DefaultGovernanceConfig.
func TestBlockRewardWithDefaultGovernance(t *testing.T) {
	// 1. DefaultGovernance
	allocBlockRewardCache()
	accounts := newTestAccounts()

	// header
	header := &types.Header{Number: big.NewInt(0)}
	proposerAddr := addr1
	header.Rewardbase = proposerAddr

	// chain config
	config := &params.ChainConfig{Istanbul: governance.GetDefaultIstanbulConfig(), Governance: governance.GetDefaultGovernanceConfig(params.UseIstanbul)}
	DistributeBlockReward(accounts, header, common.Address{}, common.Address{}, config)

	balance := accounts.GetBalance(proposerAddr)
	if balance == nil {
		t.Errorf("Fail to get balance from addr(%s)", proposerAddr.String())
	} else {
		assert.Equal(t, balance, config.Governance.Reward.MintingAmount)
	}

	// 2. DefaultGovernance and when there is used gas in block
	allocBlockRewardCache()
	accounts = newTestAccounts()

	// header
	header = &types.Header{Number: big.NewInt(0)}
	proposerAddr = addr1
	header.Rewardbase = proposerAddr
	header.GasUsed = uint64(100000)

	// chain config
	config = &params.ChainConfig{}
	config.Governance = governance.GetDefaultGovernanceConfig(params.UseIstanbul)
	config.Istanbul = governance.GetDefaultIstanbulConfig()

	DistributeBlockReward(accounts, header, common.Address{}, common.Address{}, config)

	balance = accounts.GetBalance(proposerAddr)
	if balance == nil {
		t.Errorf("Fail to get balance from addr(%s)", proposerAddr.String())
	} else {
		expectedBalance := config.Governance.Reward.MintingAmount
		assert.Equal(t, expectedBalance, balance)
	}
}

// TestBlockRewardWithDeferredTxFeeEnabled tests DistributeBlockReward when DeferredTxFee is true
func TestBlockRewardWithDeferredTxFeeEnabled(t *testing.T) {
	// 1. DefaultGovernance + header.GasUsed + DeferredTxFee True
	allocBlockRewardCache()
	accounts := newTestAccounts()

	// header
	header := &types.Header{Number: big.NewInt(0)}
	proposerAddr := addr1
	header.Rewardbase = proposerAddr
	header.GasUsed = uint64(100000)

	// chain config
	config := &params.ChainConfig{Istanbul: governance.GetDefaultIstanbulConfig(), Governance: governance.GetDefaultGovernanceConfig(params.UseIstanbul)}

	config.Governance.Reward.DeferredTxFee = true

	DistributeBlockReward(accounts, header, common.Address{}, common.Address{}, config)

	balance := accounts.GetBalance(proposerAddr)
	if balance == nil {
		t.Errorf("Fail to get balance from addr(%s)", proposerAddr.String())
	} else {
		gasUsed := new(big.Int).SetUint64(header.GasUsed)
		unitPrice := new(big.Int).SetUint64(config.Governance.UnitPrice)
		tmpInt := new(big.Int).Mul(gasUsed, unitPrice)
		expectedBalance := tmpInt.Add(tmpInt, config.Governance.Reward.MintingAmount)

		assert.Equal(t, expectedBalance, balance)
	}

	// 2. DefaultGovernance + header.GasUsed + DeferredTxFee True + params.DefaultMintedKLAY
	accounts = newTestAccounts()
	allocBlockRewardCache()

	// header
	header = &types.Header{Number: big.NewInt(0)}
	proposerAddr = addr1
	header.Rewardbase = proposerAddr
	header.GasUsed = uint64(100000)

	// chain config
	config = &params.ChainConfig{Istanbul: governance.GetDefaultIstanbulConfig(), Governance: governance.GetDefaultGovernanceConfig(params.UseIstanbul)}

	config.Governance.Reward.DeferredTxFee = true
	config.Governance.Reward.MintingAmount = params.DefaultMintedKLAY

	DistributeBlockReward(accounts, header, common.Address{}, common.Address{}, config)

	balance = accounts.GetBalance(proposerAddr)
	if balance == nil {
		t.Errorf("Fail to get balance from addr(%s)", proposerAddr.String())
	} else {
		gasUsed := new(big.Int).SetUint64(header.GasUsed)
		unitPrice := new(big.Int).SetUint64(config.Governance.UnitPrice)
		tmpInt := new(big.Int).Mul(gasUsed, unitPrice)
		expectedBalance := tmpInt.Add(tmpInt, config.Governance.Reward.MintingAmount)

		assert.Equal(t, expectedBalance, balance)
	}
}

func TestPocKirRewardDistribute(t *testing.T) {
	allocBlockRewardCache()

	accounts := newTestAccounts()
	header := &types.Header{Number: big.NewInt(0)}
	proposerAddr := addr1
	header.Rewardbase = proposerAddr
	header.GasUsed = uint64(100000)
	mintingAmount := big.NewInt(int64(1000000000))

	// chain config
	config := &params.ChainConfig{Istanbul: governance.GetDefaultIstanbulConfig(), Governance: governance.GetDefaultGovernanceConfig(params.UseIstanbul)}
	config.Governance.Reward.MintingAmount = mintingAmount
	config.Governance.Reward.Ratio = "70/20/10"

	pocAddr := common.StringToAddress("1111111111111111111111111111111111111111")
	kirAddr := common.StringToAddress("2222222222222222222222222222222222222222")

	distributeBlockReward(accounts, header, big.NewInt(0), pocAddr, kirAddr, config)

	cnBalance := accounts.GetBalance(proposerAddr)
	pocBalance := accounts.GetBalance(pocAddr)
	kirBalance := accounts.GetBalance(kirAddr)

	expectedKirBalance := big.NewInt(0).Div(mintingAmount, big.NewInt(10))     // 10%
	expectedPocBalance := big.NewInt(0).Mul(expectedKirBalance, big.NewInt(2)) // 20%
	expectedCnBalance := big.NewInt(0).Mul(expectedKirBalance, big.NewInt(7))  // 70%

	if expectedCnBalance.Cmp(cnBalance) != 0 || pocBalance.Cmp(expectedPocBalance) != 0 || kirBalance.Cmp(expectedKirBalance) != 0 {
		t.Errorf("balances are calculated incorrectly. CN Balance : %v, PoC Balance : %v, KIR Balance : %v, ratio : %v",
			cnBalance, pocBalance, kirBalance, config.Governance.Reward.Ratio)
	}

	totalBalance := big.NewInt(0).Add(cnBalance, pocBalance)
	totalBalance = big.NewInt(0).Add(totalBalance, kirBalance)

	if mintingAmount.Cmp(totalBalance) != 0 {
		t.Errorf("The sum of balance is diffrent from mintingAmount. totalBalance : %v, mintingAmount : %v", totalBalance, mintingAmount)
	}
}

// TestBlockRewardWithCustomRewardRatio tests DistributeBlockReward with reward ratio defined in params package.
func TestBlockRewardWithCustomRewardRatio(t *testing.T) {
	// 1. DefaultGovernance + header.GasUsed + DeferredTxFee True + params.DefaultMintedKLAY + DefaultCNRatio/DefaultKIRRewardRatio/DefaultPocRewardRatio
	accounts := newTestAccounts()
	allocBlockRewardCache()

	// header
	header := &types.Header{Number: big.NewInt(0)}
	proposerAddr := addr1
	header.Rewardbase = proposerAddr
	header.GasUsed = uint64(100000)

	// chain config
	config := &params.ChainConfig{Istanbul: governance.GetDefaultIstanbulConfig(), Governance: governance.GetDefaultGovernanceConfig(params.UseIstanbul)}

	config.Governance.Reward.DeferredTxFee = true
	config.Governance.Reward.MintingAmount = params.DefaultMintedKLAY
	config.Governance.Reward.Ratio = fmt.Sprintf("%d/%d/%d", params.DefaultCNRewardRatio, params.DefaultKIRRewardRatio, params.DefaultPoCRewardRatio)

	DistributeBlockReward(accounts, header, common.Address{}, common.Address{}, config)

	balance := accounts.GetBalance(proposerAddr)
	if balance == nil {
		t.Errorf("Fail to get balance from addr(%s)", proposerAddr.String())
	} else {
		gasUsed := new(big.Int).SetUint64(header.GasUsed)
		unitPrice := new(big.Int).SetUint64(config.Governance.UnitPrice)
		tmpInt := new(big.Int).Mul(gasUsed, unitPrice)
		expectedBalance := tmpInt.Add(tmpInt, config.Governance.Reward.MintingAmount)

		assert.Equal(t, expectedBalance, balance)
	}
}

func TestStakingInfoCache_Add(t *testing.T) {
	initStakingCache()

	// test cache limit
	for i := 1; i <= 10; i++ {
		testStakingInfo, _ := newEmptyStakingInfo(nil, uint64(i))
		stakingCache.add(testStakingInfo)

		if len(stakingCache.cells) > maxStakingCache {
			t.Errorf("over the max limit of staking cache. Current Len : %v, MaxStakingCache : %v", len(stakingCache.cells), maxStakingCache)
		}
	}

	// test adding same block number
	initStakingCache()
	testStakingInfo1, _ := newEmptyStakingInfo(nil, uint64(1))
	testStakingInfo2, _ := newEmptyStakingInfo(nil, uint64(1))
	stakingCache.add(testStakingInfo1)
	stakingCache.add(testStakingInfo2)

	if len(stakingCache.cells) > 1 {
		t.Errorf("StakingInfo with Same block number is saved to the cache stakingCache. result : %v, expected : %v ", len(stakingCache.cells), maxStakingCache)
	}

	// test minBlockNum
	initStakingCache()
	for i := 1; i < 5; i++ {
		testStakingInfo, _ := newEmptyStakingInfo(nil, uint64(i))
		stakingCache.add(testStakingInfo)
	}

	testStakingInfo, _ := newEmptyStakingInfo(nil, uint64(5))
	stakingCache.add(testStakingInfo) // blockNum 1 should be deleted
	if stakingCache.minBlockNum != 2 {
		t.Errorf("minBlockNum of staking cache is diffrent from expected blocknum. result : %v, expected : %v", stakingCache.minBlockNum, 2)
	}

	testStakingInfo, _ = newEmptyStakingInfo(nil, uint64(6))
	stakingCache.add(testStakingInfo) // blockNum 2 should be deleted
	if stakingCache.minBlockNum != 3 {
		t.Errorf("minBlockNum of staking cache is diffrent from expected blocknum. result : %v, expected : %v", stakingCache.minBlockNum, 3)
	}
}

func TestStakingInfoCache_Get(t *testing.T) {
	initStakingCache()

	for i := 1; i <= 4; i++ {
		testStakingInfo, _ := newEmptyStakingInfo(nil, uint64(i))
		stakingCache.add(testStakingInfo)
	}

	// should find correct stakingInfo with given block number
	for i := uint64(1); i <= 4; i++ {
		testStakingInfo := stakingCache.get(i)

		if testStakingInfo.BlockNum != i {
			t.Errorf("The block number of gotten staking info is diffrent. result : %v, expected : %v", testStakingInfo.BlockNum, i)
		}
	}

	// nothing should be found as no matched block number is in cache
	for i := uint64(5); i < 10; i++ {
		testStakingInfo := stakingCache.get(i)

		if testStakingInfo != nil {
			t.Errorf("The result should be nil. result : %v", testStakingInfo)
		}
	}
}

func TestCalcGiniCoefficient(t *testing.T) {
	testCase := []struct {
		testdata []*big.Int
		result   float64
	}{
		{[]*big.Int{big.NewInt(1), big.NewInt(1), big.NewInt(1)}, 0.0},
		{[]*big.Int{big.NewInt(0), big.NewInt(8), big.NewInt(0), big.NewInt(0), big.NewInt(0)}, 0.8},
		{[]*big.Int{big.NewInt(5), big.NewInt(4), big.NewInt(3), big.NewInt(2), big.NewInt(1)}, 0.27},
	}

	for i := 0; i < len(testCase); i++ {
		result := calcGiniCoefficient(testCase[i].testdata)

		if result != testCase[i].result {
			t.Errorf("The result is diffrent from the expected result. result : %v, expected : %v", result, testCase[i].result)
		}
	}
}

func TestGiniReflectToExpectedCCO(t *testing.T) {
	testCase := []struct {
		ccoToken        []*big.Int
		beforeReflected []float64
		adjustment      []float64
		afterReflected  []float64
	}{
		{[]*big.Int{big.NewInt(66666667), big.NewInt(233333333), big.NewInt(5000000), big.NewInt(5000000), big.NewInt(5000000),
			big.NewInt(77777778), big.NewInt(5000000), big.NewInt(33333333), big.NewInt(20000000), big.NewInt(16666667),
			big.NewInt(10000000), big.NewInt(5000000), big.NewInt(5000000), big.NewInt(5000000), big.NewInt(5000000),
			big.NewInt(5000000), big.NewInt(5000000), big.NewInt(5000000), big.NewInt(5000000), big.NewInt(5000000),
			big.NewInt(5000000),
		},
			[]float64{13, 44, 1, 1, 1, 15, 1, 6, 4, 3, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
			[]float64{42612, 89426, 9202, 9202, 9202, 46682, 9202, 28275, 20900, 18762, 13868, 9202, 9202, 9202, 9202, 9202, 9202, 9202, 9202, 9202, 9202},
			[]float64{11, 23, 2, 2, 2, 12, 2, 7, 5, 5, 4, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2},
		},
		{[]*big.Int{big.NewInt(400000000), big.NewInt(233333333), big.NewInt(233333333), big.NewInt(150000000), big.NewInt(108333333),
			big.NewInt(83333333), big.NewInt(66666667), big.NewInt(33333333), big.NewInt(20000000), big.NewInt(16666667),
			big.NewInt(10000000), big.NewInt(5000000), big.NewInt(5000000), big.NewInt(5000000), big.NewInt(5000000),
			big.NewInt(5000000), big.NewInt(5000000), big.NewInt(5000000), big.NewInt(5000000), big.NewInt(5000000),
			big.NewInt(5000000),
		},
			[]float64{28, 17, 17, 11, 8, 6, 5, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
			[]float64{123020, 89426, 89426, 68853, 56793, 48627, 42612, 28275, 20900, 18762, 13868, 9202, 9202, 9202, 9202, 9202, 9202, 9202, 9202, 9202, 9202},
			[]float64{18, 13, 13, 10, 8, 7, 6, 4, 3, 3, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		},
	}
	for i := 0; i < len(testCase); i++ {
		stakingInfo, _ := newEmptyStakingInfo(nil, uint64(1))
		stakingInfo.CouncilStakingAmounts = testCase[i].ccoToken
		for j := 0; j < len(stakingInfo.CouncilStakingAmounts); j++ {
			stakingInfo.CouncilStakingAmounts[j].Mul(stakingInfo.CouncilStakingAmounts[j], big.NewInt(params.KLAY))
		}
		stakingInfo.Gini = calcGiniCoefficient(testCase[i].ccoToken)

		stakingAmounts, totalAmount := stakingInfo.GetStakingAmountsAndTotalStaking()
		for j := 0; j < len(testCase[i].ccoToken); j++ {
			stakingAmounts[j] = math.Round(stakingAmounts[j] * 100 / totalAmount)
			if stakingAmounts[j] < 1 {
				stakingAmounts[j] = 1
			}
			if stakingAmounts[j] != testCase[i].beforeReflected[j] {
				t.Errorf("normal weight is incorrect. result : %v expected : %v", stakingAmounts[j], testCase[i].beforeReflected[j])
			}
		}

		stakingInfo.useGini = true
		stakingAmountsGiniReflected, totalAmountGiniReflected := stakingInfo.GetStakingAmountsAndTotalStaking()

		for j := 0; j < len(testCase[i].ccoToken); j++ {
			if stakingAmountsGiniReflected[j] != testCase[i].adjustment[j] {
				t.Errorf("staking amount reflected gini is diffrent. result : %v expected : %v", stakingAmountsGiniReflected[j], testCase[i].adjustment[j])
			}
		}

		for j := 0; j < len(testCase[i].ccoToken); j++ {
			stakingAmountsGiniReflected[j] = math.Round(stakingAmountsGiniReflected[j] * 100 / totalAmountGiniReflected)
			if stakingAmountsGiniReflected[j] != testCase[i].afterReflected[j] {
				t.Errorf("weight reflected gini is diffrent. result : %v expected : %v", stakingAmountsGiniReflected[j], testCase[i].afterReflected[j])
			}
		}
	}
}
