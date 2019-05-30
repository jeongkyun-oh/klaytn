package tests

import (
	"crypto/ecdsa"
	"errors"
	"github.com/ground-x/klaytn/blockchain"
	"github.com/ground-x/klaytn/blockchain/types"
	"github.com/ground-x/klaytn/blockchain/types/accountkey"
	"github.com/ground-x/klaytn/blockchain/vm"
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/common/profile"
	"github.com/ground-x/klaytn/kerrors"
	"github.com/ground-x/klaytn/params"
	"github.com/ground-x/klaytn/ser/rlp"
	"github.com/stretchr/testify/assert"
	"math/big"
	"testing"
)

type txValueMap map[types.TxValueKeyType]interface{}

type testTxType struct {
	name   string
	txType types.TxType
}

func toBasicType(txType types.TxType) types.TxType {
	return txType &^ ((1 << types.SubTxTypeBits) - 1)
}

func genMapForTxTypes(from TestAccount, to TestAccount, txType types.TxType) (txValueMap, uint64) {
	var valueMap txValueMap
	gas := uint64(0)
	gasPrice := big.NewInt(25 * params.Ston)
	newAccount, err := createDefaultAccount(accountkey.AccountKeyTypePublic)
	if err != nil {
		return nil, 0
	}
	contractAccount, err := createDefaultAccount(accountkey.AccountKeyTypeFail)
	if err != nil {
		return nil, 0
	}
	contractAccount.Addr, err = common.FromHumanReadableAddress("contract.klaytn")
	if err != nil {
		return nil, 0
	}

	// switch to basic tx type representation and generate a map
	switch toBasicType(txType) {
	case types.TxTypeLegacyTransaction:
		valueMap, gas = genMapForLegacyTransaction(from, to, gasPrice, txType)
	case types.TxTypeValueTransfer:
		valueMap, gas = genMapForValueTransfer(from, to, gasPrice, txType)
	case types.TxTypeValueTransferMemo:
		valueMap, gas = genMapForValueTransferWithMemo(from, to, gasPrice, txType)
	case types.TxTypeAccountCreation:
		valueMap, gas = genMapForCreate(from, newAccount, gasPrice, txType)
	case types.TxTypeAccountUpdate:
		valueMap, gas = genMapForUpdate(from, to, gasPrice, newAccount.AccKey, txType)
	case types.TxTypeSmartContractDeploy:
		valueMap, gas = genMapForDeploy(from, nil, gasPrice, txType)
	case types.TxTypeSmartContractExecution:
		valueMap, gas = genMapForExecution(from, contractAccount, gasPrice, txType)
	case types.TxTypeCancel:
		valueMap, gas = genMapForCancel(from, gasPrice, txType)
	case types.TxTypeChainDataAnchoring:
		valueMap, gas = genMapForChainDataAnchoring(from, gasPrice, txType)
	}

	if txType.IsFeeDelegatedTransaction() {
		valueMap[types.TxValueKeyFeePayer] = from.GetAddr()
	}

	if txType.IsFeeDelegatedWithRatioTransaction() {
		valueMap[types.TxValueKeyFeeRatioOfFeePayer] = types.FeeRatio(30)
	}

	return valueMap, gas
}

// TestValidationPoolInsert generates invalid txs which will be invalidated during txPool insert process.
func TestValidationPoolInsert(t *testing.T) {
	var testTxTypes = []testTxType{
		{"LegacyTransaction", types.TxTypeLegacyTransaction},
		{"ValueTransfer", types.TxTypeValueTransfer},
		{"ValueTransferWithMemo", types.TxTypeValueTransferMemo},
		{"AccountCreation", types.TxTypeAccountCreation},
		{"AccountUpdate", types.TxTypeAccountUpdate},
		{"SmartContractDeploy", types.TxTypeSmartContractDeploy},
		{"SmartContractExecution", types.TxTypeSmartContractExecution},
		{"Cancel", types.TxTypeCancel},
		{"ChainDataAnchoring", types.TxTypeChainDataAnchoring},
		{"FeeDelegatedValueTransfer", types.TxTypeFeeDelegatedValueTransfer},
		{"FeeDelegatedValueTransferWithMemo", types.TxTypeFeeDelegatedValueTransferMemo},
		{"FeeDelegatedAccountUpdate", types.TxTypeFeeDelegatedAccountUpdate},
		{"FeeDelegatedSmartContractDeploy", types.TxTypeFeeDelegatedSmartContractDeploy},
		{"FeeDelegatedSmartContractExecution", types.TxTypeFeeDelegatedSmartContractExecution},
		{"FeeDelegatedCancel", types.TxTypeFeeDelegatedCancel},
		{"FeeDelegatedWithRatioValueTransfer", types.TxTypeFeeDelegatedValueTransferWithRatio},
		{"FeeDelegatedWithRatioValueTransferWithMemo", types.TxTypeFeeDelegatedValueTransferMemoWithRatio},
		{"FeeDelegatedWithRatioAccountUpdate", types.TxTypeFeeDelegatedAccountUpdateWithRatio},
		{"FeeDelegatedWithRatioSmartContractDeploy", types.TxTypeFeeDelegatedSmartContractDeployWithRatio},
		{"FeeDelegatedWithRatioSmartContractExecution", types.TxTypeFeeDelegatedSmartContractExecutionWithRatio},
		{"FeeDelegatedWithRatioCancel", types.TxTypeFeeDelegatedCancelWithRatio},
	}

	var invalidCases = []struct {
		Name string
		fn   func(types.TxType, txValueMap) (txValueMap, error)
	}{
		{"invalidNonce", decreaseNonce},
		{"invalidGasLimit", decreaseGasLimit},
		{"invalidTxSize", exceedSizeLimit},
		{"invalidRecipientProgram", valueTransferToContract},
		{"invalidRecipientNotProgram", executeToEOA},
		{"invalidRecipientExisting", creationToExistingAddr},
		{"invalidCodeFormat", invalidCodeFormat},
	}

	prof := profile.NewProfiler()

	// Initialize blockchain
	bcdata, err := NewBCData(6, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer bcdata.Shutdown()

	// Initialize address-balance map for verification
	accountMap := NewAccountMap()
	if err := accountMap.Initialize(bcdata); err != nil {
		t.Fatal(err)
	}

	signer := types.NewEIP155Signer(bcdata.bc.Config().ChainID)

	// reservoir account
	reservoir := &TestAccountType{
		Addr:  *bcdata.addrs[0],
		Keys:  []*ecdsa.PrivateKey{bcdata.privKeys[0]},
		Nonce: uint64(0),
	}

	// for contract execution txs
	contract, err := createHumanReadableAccount(getRandomPrivateKeyString(t), "contract")

	// make TxPool to test validation in 'TxPool add' process
	poolSlots := 1000
	txpoolconfig := blockchain.DefaultTxPoolConfig
	txpoolconfig.Journal = ""
	txpoolconfig.ExecSlotsAccount = uint64(poolSlots)
	txpoolconfig.NonExecSlotsAccount = uint64(poolSlots)
	txpoolconfig.ExecSlotsAll = 2 * uint64(poolSlots)
	txpoolconfig.NonExecSlotsAll = 2 * uint64(poolSlots)
	txpool := blockchain.NewTxPool(txpoolconfig, bcdata.bc.Config(), bcdata.bc)

	// deploy a contract for contract execution tx type
	{
		var txs types.Transactions

		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:         reservoir.GetNonce(),
			types.TxValueKeyFrom:          reservoir.GetAddr(),
			types.TxValueKeyTo:            &(contract.Addr),
			types.TxValueKeyAmount:        big.NewInt(0),
			types.TxValueKeyGasLimit:      gasLimit,
			types.TxValueKeyGasPrice:      big.NewInt(25 * params.Ston),
			types.TxValueKeyHumanReadable: true,
			types.TxValueKeyData:          common.FromHex(code),
			types.TxValueKeyCodeFormat:    params.CodeFormatEVM,
		}

		tx, err := types.NewTransactionWithMap(types.TxTypeSmartContractDeploy, values)
		assert.Equal(t, nil, err)

		err = tx.SignWithKeys(signer, reservoir.Keys)
		assert.Equal(t, nil, err)

		txs = append(txs, tx)

		if err := bcdata.GenABlockWithTransactions(accountMap, txs, prof); err != nil {
			t.Fatal(err)
		}
		reservoir.AddNonce()
	}

	// test for all tx types
	for _, testTxType := range testTxTypes {
		txType := testTxType.txType

		// generate invalid txs and check the return error
		for _, invalidCase := range invalidCases {
			// generate a new tx and mutate it
			valueMap, _ := genMapForTxTypes(reservoir, reservoir, txType)
			invalidMap, expectedErr := invalidCase.fn(txType, valueMap)

			tx, err := types.NewTransactionWithMap(txType, invalidMap)
			assert.Equal(t, nil, err)

			err = tx.SignWithKeys(signer, reservoir.Keys)
			assert.Equal(t, nil, err)

			if txType.IsFeeDelegatedTransaction() {
				tx.SignFeePayerWithKeys(signer, reservoir.Keys)
				assert.Equal(t, nil, err)
			}

			err = txpool.AddRemote(tx)
			assert.Equal(t, expectedErr, err)
			if expectedErr == nil {
				reservoir.Nonce += 1
			}
		}
	}
}

// TestValidationBlockTx generates invalid txs which will be invalidated during block insert process.
func TestValidationBlockTx(t *testing.T) {
	var testTxTypes = []testTxType{
		{"LegacyTransaction", types.TxTypeLegacyTransaction},
		{"ValueTransfer", types.TxTypeValueTransfer},
		{"ValueTransferWithMemo", types.TxTypeValueTransferMemo},
		{"AccountCreation", types.TxTypeAccountCreation},
		{"AccountUpdate", types.TxTypeAccountUpdate},
		{"SmartContractDeploy", types.TxTypeSmartContractDeploy},
		{"SmartContractExecution", types.TxTypeSmartContractExecution},
		{"Cancel", types.TxTypeCancel},
		{"ChainDataAnchoring", types.TxTypeChainDataAnchoring},
		{"FeeDelegatedValueTransfer", types.TxTypeFeeDelegatedValueTransfer},
		{"FeeDelegatedValueTransferWithMemo", types.TxTypeFeeDelegatedValueTransferMemo},
		{"FeeDelegatedAccountUpdate", types.TxTypeFeeDelegatedAccountUpdate},
		{"FeeDelegatedSmartContractDeploy", types.TxTypeFeeDelegatedSmartContractDeploy},
		{"FeeDelegatedSmartContractExecution", types.TxTypeFeeDelegatedSmartContractExecution},
		{"FeeDelegatedCancel", types.TxTypeFeeDelegatedCancel},
		{"FeeDelegatedWithRatioValueTransfer", types.TxTypeFeeDelegatedValueTransferWithRatio},
		{"FeeDelegatedWithRatioValueTransferWithMemo", types.TxTypeFeeDelegatedValueTransferMemoWithRatio},
		{"FeeDelegatedWithRatioAccountUpdate", types.TxTypeFeeDelegatedAccountUpdateWithRatio},
		{"FeeDelegatedWithRatioSmartContractDeploy", types.TxTypeFeeDelegatedSmartContractDeployWithRatio},
		{"FeeDelegatedWithRatioSmartContractExecution", types.TxTypeFeeDelegatedSmartContractExecutionWithRatio},
		{"FeeDelegatedWithRatioCancel", types.TxTypeFeeDelegatedCancelWithRatio},
	}

	var invalidCases = []struct {
		Name string
		fn   func(types.TxType, txValueMap) (txValueMap, error)
	}{
		{"invalidNonce", decreaseNonce},
		{"invalidRecipientProgram", valueTransferToContract},
		{"invalidRecipientNotProgram", executeToEOA},
		{"invalidRecipientExisting", creationToExistingAddr},
		{"invalidCodeFormat", invalidCodeFormat},
	}

	prof := profile.NewProfiler()

	// Initialize blockchain
	bcdata, err := NewBCData(6, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer bcdata.Shutdown()

	// Initialize address-balance map for verification
	accountMap := NewAccountMap()
	if err := accountMap.Initialize(bcdata); err != nil {
		t.Fatal(err)
	}

	signer := types.NewEIP155Signer(bcdata.bc.Config().ChainID)

	// reservoir account
	reservoir := &TestAccountType{
		Addr:  *bcdata.addrs[0],
		Keys:  []*ecdsa.PrivateKey{bcdata.privKeys[0]},
		Nonce: uint64(0),
	}

	// for contract execution txs
	contract, err := createHumanReadableAccount(getRandomPrivateKeyString(t), "contract")

	// deploy a contract for contract execution tx type
	{
		var txs types.Transactions

		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:         reservoir.GetNonce(),
			types.TxValueKeyFrom:          reservoir.GetAddr(),
			types.TxValueKeyTo:            &(contract.Addr),
			types.TxValueKeyAmount:        big.NewInt(0),
			types.TxValueKeyGasLimit:      gasLimit,
			types.TxValueKeyGasPrice:      big.NewInt(25 * params.Ston),
			types.TxValueKeyHumanReadable: true,
			types.TxValueKeyData:          common.FromHex(code),
			types.TxValueKeyCodeFormat:    params.CodeFormatEVM,
		}

		tx, err := types.NewTransactionWithMap(types.TxTypeSmartContractDeploy, values)
		assert.Equal(t, nil, err)

		err = tx.SignWithKeys(signer, reservoir.Keys)
		assert.Equal(t, nil, err)

		txs = append(txs, tx)

		if err := bcdata.GenABlockWithTransactions(accountMap, txs, prof); err != nil {
			t.Fatal(err)
		}
		reservoir.AddNonce()
	}

	// test for all tx types
	for _, testTxType := range testTxTypes {
		txType := testTxType.txType

		// generate invalid txs and check the return error
		for _, invalidCase := range invalidCases {
			// generate a new tx and mutate it
			valueMap, _ := genMapForTxTypes(reservoir, reservoir, txType)
			invalidMap, expectedErr := invalidCase.fn(txType, valueMap)

			tx, err := types.NewTransactionWithMap(txType, invalidMap)
			assert.Equal(t, nil, err)

			err = tx.SignWithKeys(signer, reservoir.Keys)
			assert.Equal(t, nil, err)

			if txType.IsFeeDelegatedTransaction() {
				tx.SignFeePayerWithKeys(signer, reservoir.Keys)
				assert.Equal(t, nil, err)
			}

			receipt, _, err := applyTransaction(t, bcdata, tx)
			assert.Equal(t, expectedErr, err)
			if expectedErr == nil {
				assert.Equal(t, types.ReceiptStatusSuccessful, receipt.Status)
			}
		}
	}
}

// decreaseNonce changes nonce to zero.
func decreaseNonce(txType types.TxType, values txValueMap) (txValueMap, error) {
	values[types.TxValueKeyNonce] = uint64(0)

	return values, blockchain.ErrNonceTooLow
}

// decreaseGasLimit changes gasLimit to 12345678
func decreaseGasLimit(txType types.TxType, values txValueMap) (txValueMap, error) {
	(*big.Int).SetUint64(values[types.TxValueKeyGasPrice].(*big.Int), 12345678)

	return values, blockchain.ErrInvalidUnitPrice
}

// exceedSizeLimit assigns tx data bigger than MaxTxDataSize.
func exceedSizeLimit(txType types.TxType, values txValueMap) (txValueMap, error) {
	invalidData := make([]byte, blockchain.MaxTxDataSize+1)

	if values[types.TxValueKeyData] != nil {
		values[types.TxValueKeyData] = invalidData
		return values, blockchain.ErrOversizedData
	}

	if values[types.TxValueKeyAnchoredData] != nil {
		values[types.TxValueKeyAnchoredData] = invalidData
		return values, blockchain.ErrOversizedData
	}

	return values, nil
}

// valueTransferToContract changes recipient address of value transfer txs to the contract address, "contract.klaytn".
func valueTransferToContract(txType types.TxType, values txValueMap) (txValueMap, error) {
	programAddr, err := common.FromHumanReadableAddress("contract.klaytn")
	if err != nil {
		return nil, nil
	}

	txType = toBasicType(txType)
	if txType == types.TxTypeValueTransfer || txType == types.TxTypeValueTransferMemo {
		values[types.TxValueKeyTo] = programAddr
		return values, kerrors.ErrNotForProgramAccount
	}

	return values, nil
}

// creationToExistingAddr changes the recipient of account creating txs to the existing address, "contract.klaytn".
func creationToExistingAddr(txType types.TxType, values txValueMap) (txValueMap, error) {
	existingAddr, err := common.FromHumanReadableAddress("contract.klaytn")
	if err != nil {
		return nil, nil
	}

	if txType.IsAccountCreation() {
		values[types.TxValueKeyTo] = existingAddr
		values[types.TxValueKeyHumanReadable] = true
		return values, kerrors.ErrAccountAlreadyExists
	}

	if txType.IsContractDeploy() {
		values[types.TxValueKeyTo] = &existingAddr
		values[types.TxValueKeyHumanReadable] = true
		return values, kerrors.ErrAccountAlreadyExists
	}

	return values, nil
}

// executeToEOA changes the recipient of contract execution txs to an EOA address (the same with the sender).
func executeToEOA(txType types.TxType, values txValueMap) (txValueMap, error) {
	if toBasicType(txType) == types.TxTypeSmartContractExecution {
		values[types.TxValueKeyTo] = values[types.TxValueKeyFrom].(common.Address)
		return values, kerrors.ErrNotProgramAccount
	}

	return values, nil
}

func invalidCodeFormat(txType types.TxType, values txValueMap) (txValueMap, error) {
	if txType.IsContractDeploy() {
		values[types.TxValueKeyCodeFormat] = params.CodeFormatLast
		return values, kerrors.ErrInvalidCodeFormat
	}
	return values, nil
}

// TestValidationInvalidSig generates txs signed by an invalid sender or a fee payer.
func TestValidationInvalidSig(t *testing.T) {
	var testTxTypes = []testTxType{
		{"LegacyTransaction", types.TxTypeLegacyTransaction},
		{"ValueTransfer", types.TxTypeValueTransfer},
		{"ValueTransferWithMemo", types.TxTypeValueTransferMemo},
		{"AccountCreation", types.TxTypeAccountCreation},
		{"AccountUpdate", types.TxTypeAccountUpdate},
		{"SmartContractDeploy", types.TxTypeSmartContractDeploy},
		{"SmartContractExecution", types.TxTypeSmartContractExecution},
		{"Cancel", types.TxTypeCancel},
		{"ChainDataAnchoring", types.TxTypeChainDataAnchoring},
		{"FeeDelegatedValueTransfer", types.TxTypeFeeDelegatedValueTransfer},
		{"FeeDelegatedValueTransferWithMemo", types.TxTypeFeeDelegatedValueTransferMemo},
		{"FeeDelegatedAccountUpdate", types.TxTypeFeeDelegatedAccountUpdate},
		{"FeeDelegatedSmartContractDeploy", types.TxTypeFeeDelegatedSmartContractDeploy},
		{"FeeDelegatedSmartContractExecution", types.TxTypeFeeDelegatedSmartContractExecution},
		{"FeeDelegatedCancel", types.TxTypeFeeDelegatedCancel},
		{"FeeDelegatedWithRatioValueTransfer", types.TxTypeFeeDelegatedValueTransferWithRatio},
		{"FeeDelegatedWithRatioValueTransferWithMemo", types.TxTypeFeeDelegatedValueTransferMemoWithRatio},
		{"FeeDelegatedWithRatioAccountUpdate", types.TxTypeFeeDelegatedAccountUpdateWithRatio},
		{"FeeDelegatedWithRatioSmartContractDeploy", types.TxTypeFeeDelegatedSmartContractDeployWithRatio},
		{"FeeDelegatedWithRatioSmartContractExecution", types.TxTypeFeeDelegatedSmartContractExecutionWithRatio},
		{"FeeDelegatedWithRatioCancel", types.TxTypeFeeDelegatedCancelWithRatio},
	}

	var invalidCases = []struct {
		Name string
		fn   func(*testing.T, types.TxType, *TestAccountType, types.EIP155Signer) (*types.Transaction, error)
	}{
		{"invalidSender", testInvalidSenderSig},
		{"invalidFeePayer", testInvalidFeePayerSig},
	}

	prof := profile.NewProfiler()

	// Initialize blockchain
	bcdata, err := NewBCData(6, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer bcdata.Shutdown()

	// Initialize address-balance map for verification
	accountMap := NewAccountMap()
	if err := accountMap.Initialize(bcdata); err != nil {
		t.Fatal(err)
	}

	signer := types.NewEIP155Signer(bcdata.bc.Config().ChainID)

	// reservoir account
	reservoir := &TestAccountType{
		Addr:  *bcdata.addrs[0],
		Keys:  []*ecdsa.PrivateKey{bcdata.privKeys[0]},
		Nonce: uint64(0),
	}

	// for contract execution txs
	contract, err := createHumanReadableAccount(getRandomPrivateKeyString(t), "contract")

	// make TxPool to test validation in 'TxPool add' process
	poolSlots := 1000
	txpoolconfig := blockchain.DefaultTxPoolConfig
	txpoolconfig.Journal = ""
	txpoolconfig.ExecSlotsAccount = uint64(poolSlots)
	txpoolconfig.NonExecSlotsAccount = uint64(poolSlots)
	txpoolconfig.ExecSlotsAll = 2 * uint64(poolSlots)
	txpoolconfig.NonExecSlotsAll = 2 * uint64(poolSlots)
	txpool := blockchain.NewTxPool(txpoolconfig, bcdata.bc.Config(), bcdata.bc)

	// deploy a contract for contract execution tx type
	{
		var txs types.Transactions

		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:         reservoir.GetNonce(),
			types.TxValueKeyFrom:          reservoir.GetAddr(),
			types.TxValueKeyTo:            &(contract.Addr),
			types.TxValueKeyAmount:        big.NewInt(0),
			types.TxValueKeyGasLimit:      gasLimit,
			types.TxValueKeyGasPrice:      big.NewInt(25 * params.Ston),
			types.TxValueKeyHumanReadable: true,
			types.TxValueKeyData:          common.FromHex(code),
			types.TxValueKeyCodeFormat:    params.CodeFormatEVM,
		}

		tx, err := types.NewTransactionWithMap(types.TxTypeSmartContractDeploy, values)
		assert.Equal(t, nil, err)

		err = tx.SignWithKeys(signer, reservoir.Keys)
		assert.Equal(t, nil, err)

		txs = append(txs, tx)

		if err := bcdata.GenABlockWithTransactions(accountMap, txs, prof); err != nil {
			t.Fatal(err)
		}
		reservoir.AddNonce()
	}

	// test for all tx types
	for _, testTxType := range testTxTypes {
		txType := testTxType.txType

		for _, invalidCase := range invalidCases {
			tx, expectedErr := invalidCase.fn(t, txType, reservoir, signer)

			if tx != nil {
				// For tx pool validation test
				err = txpool.AddRemote(tx)
				assert.Equal(t, expectedErr, err)

				// For block tx validation test
				if expectedErr == blockchain.ErrInvalidFeePayer {
					expectedErr = types.ErrInvalidSigFeePayer
				}
				receipt, _, err := applyTransaction(t, bcdata, tx)
				assert.Equal(t, expectedErr, err)
				assert.Equal(t, (*types.Receipt)(nil), receipt)
			}
		}
	}
}

// testInvalidSenderSig generates invalid txs signed by an invalid sender.
func testInvalidSenderSig(t *testing.T, txType types.TxType, reservoir *TestAccountType, signer types.EIP155Signer) (*types.Transaction, error) {
	if !txType.IsLegacyTransaction() {
		newAcc, err := createDefaultAccount(accountkey.AccountKeyTypePublic)
		assert.Equal(t, nil, err)

		valueMap, _ := genMapForTxTypes(reservoir, reservoir, txType)
		tx, err := types.NewTransactionWithMap(txType, valueMap)
		assert.Equal(t, nil, err)

		err = tx.SignWithKeys(signer, newAcc.Keys)
		assert.Equal(t, nil, err)

		if txType.IsFeeDelegatedTransaction() {
			tx.SignFeePayerWithKeys(signer, reservoir.Keys)
			assert.Equal(t, nil, err)
		}
		return tx, types.ErrInvalidSigSender
	}
	return nil, nil
}

// testInvalidFeePayerSig generates invalid txs signed by an invalid fee payer.
func testInvalidFeePayerSig(t *testing.T, txType types.TxType, reservoir *TestAccountType, signer types.EIP155Signer) (*types.Transaction, error) {
	if txType.IsFeeDelegatedTransaction() {
		newAcc, err := createDefaultAccount(accountkey.AccountKeyTypePublic)
		assert.Equal(t, nil, err)

		valueMap, _ := genMapForTxTypes(reservoir, reservoir, txType)
		tx, err := types.NewTransactionWithMap(txType, valueMap)
		assert.Equal(t, nil, err)

		err = tx.SignWithKeys(signer, reservoir.Keys)
		assert.Equal(t, nil, err)

		tx.SignFeePayerWithKeys(signer, newAcc.Keys)
		assert.Equal(t, nil, err)

		return tx, blockchain.ErrInvalidFeePayer
	}
	return nil, nil
}

// TestLegacyTxFromNonLegacyAcc generates legacy tx from non-legacy account, and it will be invalidated during txPool insert process.
func TestLegacyTxFromNonLegacyAcc(t *testing.T) {
	prof := profile.NewProfiler()

	// Initialize blockchain
	bcdata, err := NewBCData(6, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer bcdata.Shutdown()

	// Initialize address-balance map for verification
	accountMap := NewAccountMap()
	if err := accountMap.Initialize(bcdata); err != nil {
		t.Fatal(err)
	}

	signer := types.NewEIP155Signer(bcdata.bc.Config().ChainID)

	// reservoir account
	reservoir := &TestAccountType{
		Addr:  *bcdata.addrs[0],
		Keys:  []*ecdsa.PrivateKey{bcdata.privKeys[0]},
		Nonce: uint64(0),
	}

	// make TxPool to test validation in 'TxPool add' process
	poolSlots := 1000
	txpoolconfig := blockchain.DefaultTxPoolConfig
	txpoolconfig.Journal = ""
	txpoolconfig.ExecSlotsAccount = uint64(poolSlots)
	txpoolconfig.NonExecSlotsAccount = uint64(poolSlots)
	txpoolconfig.ExecSlotsAll = 2 * uint64(poolSlots)
	txpoolconfig.NonExecSlotsAll = 2 * uint64(poolSlots)
	txpool := blockchain.NewTxPool(txpoolconfig, bcdata.bc.Config(), bcdata.bc)

	var txs types.Transactions
	acc1, err := createDefaultAccount(accountkey.AccountKeyTypePublic)

	valueMap, _ := genMapForTxTypes(reservoir, reservoir, types.TxTypeAccountCreation)
	valueMap[types.TxValueKeyTo] = acc1.Addr
	valueMap[types.TxValueKeyAccountKey] = acc1.AccKey

	tx, err := types.NewTransactionWithMap(types.TxTypeAccountCreation, valueMap)
	assert.Equal(t, nil, err)

	err = tx.SignWithKeys(signer, reservoir.Keys)
	assert.Equal(t, nil, err)

	txs = append(txs, tx)

	if err := bcdata.GenABlockWithTransactions(accountMap, txs, prof); err != nil {
		t.Fatal(err)
	}
	reservoir.AddNonce()

	valueMap, _ = genMapForTxTypes(acc1, reservoir, types.TxTypeLegacyTransaction)
	tx, err = types.NewTransactionWithMap(types.TxTypeLegacyTransaction, valueMap)
	assert.Equal(t, nil, err)

	err = tx.SignWithKeys(signer, acc1.Keys)
	assert.Equal(t, nil, err)

	err = txpool.AddRemote(tx)
	assert.Equal(t, kerrors.ErrLegacyTransactionMustBeWithLegacyKey, err)
}

// TestInvalidBalance generates invalid txs which don't have enough KLAY, and will be invalidated during txPool insert process.
func TestInvalidBalance(t *testing.T) {
	var testTxTypes = []testTxType{
		{"LegacyTransaction", types.TxTypeLegacyTransaction},
		{"ValueTransfer", types.TxTypeValueTransfer},
		{"ValueTransferWithMemo", types.TxTypeValueTransferMemo},
		{"AccountCreation", types.TxTypeAccountCreation},
		{"AccountUpdate", types.TxTypeAccountUpdate},
		{"SmartContractDeploy", types.TxTypeSmartContractDeploy},
		{"SmartContractExecution", types.TxTypeSmartContractExecution},
		{"Cancel", types.TxTypeCancel},
		{"ChainDataAnchoring", types.TxTypeChainDataAnchoring},
		{"FeeDelegatedValueTransfer", types.TxTypeFeeDelegatedValueTransfer},
		{"FeeDelegatedValueTransferWithMemo", types.TxTypeFeeDelegatedValueTransferMemo},
		{"FeeDelegatedAccountUpdate", types.TxTypeFeeDelegatedAccountUpdate},
		{"FeeDelegatedSmartContractDeploy", types.TxTypeFeeDelegatedSmartContractDeploy},
		{"FeeDelegatedSmartContractExecution", types.TxTypeFeeDelegatedSmartContractExecution},
		{"FeeDelegatedCancel", types.TxTypeFeeDelegatedCancel},
		{"FeeDelegatedWithRatioValueTransfer", types.TxTypeFeeDelegatedValueTransferWithRatio},
		{"FeeDelegatedWithRatioValueTransferWithMemo", types.TxTypeFeeDelegatedValueTransferMemoWithRatio},
		{"FeeDelegatedWithRatioAccountUpdate", types.TxTypeFeeDelegatedAccountUpdateWithRatio},
		{"FeeDelegatedWithRatioSmartContractDeploy", types.TxTypeFeeDelegatedSmartContractDeployWithRatio},
		{"FeeDelegatedWithRatioSmartContractExecution", types.TxTypeFeeDelegatedSmartContractExecutionWithRatio},
		{"FeeDelegatedWithRatioCancel", types.TxTypeFeeDelegatedCancelWithRatio},
	}

	prof := profile.NewProfiler()

	// Initialize blockchain
	bcdata, err := NewBCData(6, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer bcdata.Shutdown()

	// Initialize address-balance map for verification
	accountMap := NewAccountMap()
	if err := accountMap.Initialize(bcdata); err != nil {
		t.Fatal(err)
	}

	signer := types.NewEIP155Signer(bcdata.bc.Config().ChainID)

	// reservoir account
	reservoir := &TestAccountType{
		Addr:  *bcdata.addrs[0],
		Keys:  []*ecdsa.PrivateKey{bcdata.privKeys[0]},
		Nonce: uint64(0),
	}

	// for contract execution txs
	contract, err := createHumanReadableAccount(getRandomPrivateKeyString(t), "contract")

	// test account will be lack of KLAY
	testAcc, err := createDefaultAccount(accountkey.AccountKeyTypeLegacy)
	assert.Equal(t, nil, err)

	// make TxPool to test validation in 'TxPool add' process
	poolSlots := 1000
	txpoolconfig := blockchain.DefaultTxPoolConfig
	txpoolconfig.Journal = ""
	txpoolconfig.ExecSlotsAccount = uint64(poolSlots)
	txpoolconfig.NonExecSlotsAccount = uint64(poolSlots)
	txpoolconfig.ExecSlotsAll = 2 * uint64(poolSlots)
	txpoolconfig.NonExecSlotsAll = 2 * uint64(poolSlots)
	txpool := blockchain.NewTxPool(txpoolconfig, bcdata.bc.Config(), bcdata.bc)

	gasLimit := uint64(100000000000)
	gasPrice := big.NewInt(25 * params.Ston)
	amount := uint64(25 * params.Ston)
	cost := new(big.Int).Mul(new(big.Int).SetUint64(gasLimit), gasPrice)
	cost.Add(cost, new(big.Int).SetUint64(amount))

	// deploy a contract for contract execution tx type
	{
		var txs types.Transactions

		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:         reservoir.GetNonce(),
			types.TxValueKeyFrom:          reservoir.GetAddr(),
			types.TxValueKeyTo:            &(contract.Addr),
			types.TxValueKeyAmount:        big.NewInt(0),
			types.TxValueKeyGasLimit:      gasLimit,
			types.TxValueKeyGasPrice:      big.NewInt(25 * params.Ston),
			types.TxValueKeyHumanReadable: true,
			types.TxValueKeyData:          common.FromHex(code),
			types.TxValueKeyCodeFormat:    params.CodeFormatEVM,
		}

		tx, err := types.NewTransactionWithMap(types.TxTypeSmartContractDeploy, values)
		assert.Equal(t, nil, err)

		err = tx.SignWithKeys(signer, reservoir.Keys)
		assert.Equal(t, nil, err)

		txs = append(txs, tx)

		if err := bcdata.GenABlockWithTransactions(accountMap, txs, prof); err != nil {
			t.Fatal(err)
		}
		reservoir.AddNonce()
	}

	// generate a test account with a specific amount of KLAY
	{
		var txs types.Transactions

		valueMapForCreation, _ := genMapForTxTypes(reservoir, reservoir, types.TxTypeAccountCreation)
		valueMapForCreation[types.TxValueKeyTo] = testAcc.Addr
		valueMapForCreation[types.TxValueKeyAccountKey] = testAcc.AccKey
		valueMapForCreation[types.TxValueKeyAmount] = cost

		tx, err := types.NewTransactionWithMap(types.TxTypeAccountCreation, valueMapForCreation)
		assert.Equal(t, nil, err)

		err = tx.SignWithKeys(signer, reservoir.Keys)
		assert.Equal(t, nil, err)

		txs = append(txs, tx)

		if err := bcdata.GenABlockWithTransactions(accountMap, txs, prof); err != nil {
			t.Fatal(err)
		}
		reservoir.AddNonce()
	}

	// test for all tx types
	for _, testTxType := range testTxTypes {
		txType := testTxType.txType

		if !txType.IsFeeDelegatedTransaction() {
			// tx with a specific amount or a gasLimit requiring more KLAY than the sender has.
			{
				valueMap, _ := genMapForTxTypes(testAcc, reservoir, txType)
				if valueMap[types.TxValueKeyAmount] != nil {
					valueMap[types.TxValueKeyAmount] = new(big.Int).SetUint64(amount)
					valueMap[types.TxValueKeyGasLimit] = gasLimit + 1 // requires 1 more gas
				} else {
					valueMap[types.TxValueKeyGasLimit] = gasLimit + (amount / gasPrice.Uint64()) + 1 // requires 1 more gas
				}

				tx, err := types.NewTransactionWithMap(txType, valueMap)
				assert.Equal(t, nil, err)

				err = tx.SignWithKeys(signer, testAcc.Keys)
				assert.Equal(t, nil, err)

				err = txpool.AddRemote(tx)
				assert.Equal(t, blockchain.ErrInsufficientFundsFrom, err)
			}

			// tx with a specific amount or a gasLimit requiring the exact KLAY the sender has.
			{
				valueMap, _ := genMapForTxTypes(testAcc, reservoir, txType)
				if valueMap[types.TxValueKeyAmount] != nil {
					valueMap[types.TxValueKeyAmount] = new(big.Int).SetUint64(amount)
					valueMap[types.TxValueKeyGasLimit] = gasLimit
				} else {
					valueMap[types.TxValueKeyGasLimit] = gasLimit + (amount / gasPrice.Uint64())
				}

				tx, err := types.NewTransactionWithMap(txType, valueMap)
				assert.Equal(t, nil, err)

				err = tx.SignWithKeys(signer, testAcc.Keys)
				assert.Equal(t, nil, err)

				// Since `txpool.AddRemote` does not make a block,
				// the sender can send txs to txpool in multiple times (by the for loop) with limited KLAY.
				err = txpool.AddRemote(tx)
				assert.Equal(t, nil, err)
				testAcc.AddNonce()
			}
		}

		if txType.IsFeeDelegatedTransaction() && !txType.IsFeeDelegatedWithRatioTransaction() {
			// tx with a specific amount requiring more KLAY than the sender has.
			{
				valueMap, _ := genMapForTxTypes(testAcc, reservoir, txType)
				if valueMap[types.TxValueKeyAmount] != nil {
					valueMap[types.TxValueKeyFeePayer] = reservoir.Addr
					valueMap[types.TxValueKeyAmount] = new(big.Int).Add(cost, new(big.Int).SetUint64(1)) // requires 1 more amount

					tx, err := types.NewTransactionWithMap(txType, valueMap)
					assert.Equal(t, nil, err)

					err = tx.SignWithKeys(signer, testAcc.Keys)
					assert.Equal(t, nil, err)

					tx.SignFeePayerWithKeys(signer, reservoir.Keys)
					assert.Equal(t, nil, err)

					err = txpool.AddRemote(tx)
					assert.Equal(t, blockchain.ErrInsufficientFundsFrom, err)
				}
			}

			// tx with a specific gasLimit (or amount) requiring more KLAY than the feePayer has.
			{
				valueMap, _ := genMapForTxTypes(reservoir, reservoir, txType)
				valueMap[types.TxValueKeyFeePayer] = testAcc.Addr
				valueMap[types.TxValueKeyGasLimit] = gasLimit + (amount / gasPrice.Uint64()) + 1 // requires 1 more gas

				tx, err := types.NewTransactionWithMap(txType, valueMap)
				assert.Equal(t, nil, err)

				err = tx.SignWithKeys(signer, reservoir.Keys)
				assert.Equal(t, nil, err)

				tx.SignFeePayerWithKeys(signer, testAcc.Keys)
				assert.Equal(t, nil, err)

				err = txpool.AddRemote(tx)
				assert.Equal(t, blockchain.ErrInsufficientFundsFeePayer, err)
			}

			// tx with a specific amount requiring the exact KLAY the sender has.
			{
				valueMap, _ := genMapForTxTypes(testAcc, reservoir, txType)
				if valueMap[types.TxValueKeyAmount] != nil {
					valueMap[types.TxValueKeyFeePayer] = reservoir.Addr
					valueMap[types.TxValueKeyAmount] = cost

					tx, err := types.NewTransactionWithMap(txType, valueMap)
					assert.Equal(t, nil, err)

					err = tx.SignWithKeys(signer, testAcc.Keys)
					assert.Equal(t, nil, err)

					tx.SignFeePayerWithKeys(signer, reservoir.Keys)
					assert.Equal(t, nil, err)

					// Since `txpool.AddRemote` does not make a block,
					// the sender can send txs to txpool in multiple times (by the for loop) with limited KLAY.
					err = txpool.AddRemote(tx)
					assert.Equal(t, nil, err)
					testAcc.AddNonce()
				}
			}

			// tx with a specific gasLimit (or amount) requiring the exact KLAY the feePayer has.
			{
				valueMap, _ := genMapForTxTypes(reservoir, reservoir, txType)
				valueMap[types.TxValueKeyFeePayer] = testAcc.Addr
				valueMap[types.TxValueKeyGasLimit] = gasLimit + (amount / gasPrice.Uint64())

				tx, err := types.NewTransactionWithMap(txType, valueMap)
				assert.Equal(t, nil, err)

				err = tx.SignWithKeys(signer, reservoir.Keys)
				assert.Equal(t, nil, err)

				tx.SignFeePayerWithKeys(signer, testAcc.Keys)
				assert.Equal(t, nil, err)

				// Since `txpool.AddRemote` does not make a block,
				// the sender can send txs to txpool in multiple times (by the for loop) with limited KLAY.
				err = txpool.AddRemote(tx)
				assert.Equal(t, nil, err)
				reservoir.AddNonce()
			}
		}

		if txType.IsFeeDelegatedWithRatioTransaction() {
			// tx with a specific amount and a gasLimit requiring more KLAY than the sender has.
			{
				valueMap, _ := genMapForTxTypes(testAcc, reservoir, txType)
				valueMap[types.TxValueKeyFeePayer] = reservoir.Addr
				valueMap[types.TxValueKeyFeeRatioOfFeePayer] = types.FeeRatio(90)
				if valueMap[types.TxValueKeyAmount] != nil {
					valueMap[types.TxValueKeyAmount] = new(big.Int).SetUint64(amount)
					// Gas testAcc will charge = tx gasLimit * sender's feeRatio
					// = (gasLimit + 1) * 10 * (100 - 90) * 0.01 = gasLimit + 1
					valueMap[types.TxValueKeyGasLimit] = (gasLimit + 1) * 10 // requires 1 more gas
				} else {
					// Gas testAcc will charge = tx gasLimit * sender's feeRatio
					// = (gasLimit + (amount / gasPrice.Uint64()) + 1) * 10 * (100 - 90) * 0.01 = gasLimit + (amount / gasPrice.Uint64()) + 1
					valueMap[types.TxValueKeyGasLimit] = (gasLimit + (amount / gasPrice.Uint64()) + 1) * 10 // requires 1 more gas
				}

				tx, err := types.NewTransactionWithMap(txType, valueMap)
				assert.Equal(t, nil, err)

				err = tx.SignWithKeys(signer, testAcc.Keys)
				assert.Equal(t, nil, err)

				tx.SignFeePayerWithKeys(signer, reservoir.Keys)
				assert.Equal(t, nil, err)

				err = txpool.AddRemote(tx)
				assert.Equal(t, blockchain.ErrInsufficientFundsFrom, err)
			}

			// tx with a specific amount and a gasLimit requiring more KLAY than the feePayer has.
			{
				valueMap, _ := genMapForTxTypes(reservoir, reservoir, txType)
				valueMap[types.TxValueKeyFeePayer] = testAcc.Addr
				valueMap[types.TxValueKeyFeeRatioOfFeePayer] = types.FeeRatio(10)
				// Gas testAcc will charge = tx gasLimit * fee-payer's feeRatio
				// = (gasLimit + (amount / gasPrice.Uint64()) + 1) * 10 * 10 * 0.01 = gasLimit + (amount / gasPrice.Uint64()) + 1
				valueMap[types.TxValueKeyGasLimit] = (gasLimit + (amount / gasPrice.Uint64()) + 1) * 10 // requires 1 more gas

				tx, err := types.NewTransactionWithMap(txType, valueMap)
				assert.Equal(t, nil, err)

				err = tx.SignWithKeys(signer, reservoir.Keys)
				assert.Equal(t, nil, err)

				tx.SignFeePayerWithKeys(signer, testAcc.Keys)
				assert.Equal(t, nil, err)

				err = txpool.AddRemote(tx)
				assert.Equal(t, blockchain.ErrInsufficientFundsFeePayer, err)
			}

			// tx with a specific amount and a gasLimit requiring the exact KLAY the sender has.
			{
				valueMap, _ := genMapForTxTypes(testAcc, reservoir, txType)
				valueMap[types.TxValueKeyFeePayer] = reservoir.Addr
				valueMap[types.TxValueKeyFeeRatioOfFeePayer] = types.FeeRatio(90)
				if valueMap[types.TxValueKeyAmount] != nil {
					valueMap[types.TxValueKeyAmount] = new(big.Int).SetUint64(amount)
					// Gas testAcc will charge = tx gasLimit * sender's feeRatio
					// = gasLimit * 10 * (100 - 90) * 0.01 = gasLimit
					valueMap[types.TxValueKeyGasLimit] = gasLimit * 10
				} else {
					// Gas testAcc will charge = tx gasLimit * sender's feeRatio
					// = (gasLimit + (amount / gasPrice.Uint64())) * 10 * (100 - 90) * 0.01 = gasLimit + (amount / gasPrice.Uint64())
					valueMap[types.TxValueKeyGasLimit] = (gasLimit + (amount / gasPrice.Uint64())) * 10
				}

				tx, err := types.NewTransactionWithMap(txType, valueMap)
				assert.Equal(t, nil, err)

				err = tx.SignWithKeys(signer, testAcc.Keys)
				assert.Equal(t, nil, err)

				tx.SignFeePayerWithKeys(signer, reservoir.Keys)
				assert.Equal(t, nil, err)

				// Since `txpool.AddRemote` does not make a block,
				// the sender can send txs to txpool in multiple times (by the for loop) with limited KLAY.
				err = txpool.AddRemote(tx)
				assert.Equal(t, nil, err)
				testAcc.AddNonce()
			}

			// tx with a specific amount and a gasLimit requiring the exact KLAY the feePayer has.
			{
				valueMap, _ := genMapForTxTypes(reservoir, reservoir, txType)
				valueMap[types.TxValueKeyFeePayer] = testAcc.Addr
				valueMap[types.TxValueKeyFeeRatioOfFeePayer] = types.FeeRatio(10)
				// Gas testAcc will charge = tx gasLimit * fee-payer's feeRatio
				// = (gasLimit + (amount / gasPrice.Uint64())) * 10 * 10 * 0.01 = gasLimit + (amount / gasPrice.Uint64())
				valueMap[types.TxValueKeyGasLimit] = (gasLimit + (amount / gasPrice.Uint64())) * 10

				tx, err := types.NewTransactionWithMap(txType, valueMap)
				assert.Equal(t, nil, err)

				err = tx.SignWithKeys(signer, reservoir.Keys)
				assert.Equal(t, nil, err)

				tx.SignFeePayerWithKeys(signer, testAcc.Keys)
				assert.Equal(t, nil, err)

				// Since `txpool.AddRemote` does not make a block,
				// the sender can send txs to txpool in multiple times (by the for loop) with limited KLAY.
				err = txpool.AddRemote(tx)
				assert.Equal(t, nil, err)
				reservoir.AddNonce()
			}
		}
	}
}

// TestInvalidBalanceBlockTx generates invalid txs which don't have enough KLAY, and will be invalidated during block insert process.
func TestInvalidBalanceBlockTx(t *testing.T) {
	var testTxTypes = []testTxType{
		{"LegacyTransaction", types.TxTypeLegacyTransaction},
		{"ValueTransfer", types.TxTypeValueTransfer},
		{"ValueTransferWithMemo", types.TxTypeValueTransferMemo},
		{"AccountCreation", types.TxTypeAccountCreation},
		{"AccountUpdate", types.TxTypeAccountUpdate},
		{"SmartContractDeploy", types.TxTypeSmartContractDeploy},
		{"SmartContractExecution", types.TxTypeSmartContractExecution},
		{"Cancel", types.TxTypeCancel},
		{"ChainDataAnchoring", types.TxTypeChainDataAnchoring},
		{"FeeDelegatedValueTransfer", types.TxTypeFeeDelegatedValueTransfer},
		{"FeeDelegatedValueTransferWithMemo", types.TxTypeFeeDelegatedValueTransferMemo},
		{"FeeDelegatedAccountUpdate", types.TxTypeFeeDelegatedAccountUpdate},
		{"FeeDelegatedSmartContractDeploy", types.TxTypeFeeDelegatedSmartContractDeploy},
		{"FeeDelegatedSmartContractExecution", types.TxTypeFeeDelegatedSmartContractExecution},
		{"FeeDelegatedCancel", types.TxTypeFeeDelegatedCancel},
		{"FeeDelegatedWithRatioValueTransfer", types.TxTypeFeeDelegatedValueTransferWithRatio},
		{"FeeDelegatedWithRatioValueTransferWithMemo", types.TxTypeFeeDelegatedValueTransferMemoWithRatio},
		{"FeeDelegatedWithRatioAccountUpdate", types.TxTypeFeeDelegatedAccountUpdateWithRatio},
		{"FeeDelegatedWithRatioSmartContractDeploy", types.TxTypeFeeDelegatedSmartContractDeployWithRatio},
		{"FeeDelegatedWithRatioSmartContractExecution", types.TxTypeFeeDelegatedSmartContractExecutionWithRatio},
		{"FeeDelegatedWithRatioCancel", types.TxTypeFeeDelegatedCancelWithRatio},
	}
	// re-declare errors since those errors are private variables in 'blockchain' package.
	errInsufficientBalanceForGas := errors.New("insufficient balance of the sender to pay for gas")
	errInsufficientBalanceForGasFeePayer := errors.New("insufficient balance of the fee payer to pay for gas")

	prof := profile.NewProfiler()

	// Initialize blockchain
	bcdata, err := NewBCData(6, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer bcdata.Shutdown()

	// Initialize address-balance map for verification
	accountMap := NewAccountMap()
	if err := accountMap.Initialize(bcdata); err != nil {
		t.Fatal(err)
	}

	signer := types.NewEIP155Signer(bcdata.bc.Config().ChainID)

	// reservoir account
	reservoir := &TestAccountType{
		Addr:  *bcdata.addrs[0],
		Keys:  []*ecdsa.PrivateKey{bcdata.privKeys[0]},
		Nonce: uint64(0),
	}

	// for contract execution txs
	contract, err := createHumanReadableAccount(getRandomPrivateKeyString(t), "contract")

	// test account will be lack of KLAY
	testAcc, err := createDefaultAccount(accountkey.AccountKeyTypeLegacy)
	assert.Equal(t, nil, err)

	gasLimit := uint64(100000000000)
	gasPrice := big.NewInt(25 * params.Ston)
	amount := uint64(25 * params.Ston)
	cost := new(big.Int).Mul(new(big.Int).SetUint64(gasLimit), gasPrice)
	cost.Add(cost, new(big.Int).SetUint64(amount))

	// deploy a contract for contract execution tx type
	{
		var txs types.Transactions

		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:         reservoir.GetNonce(),
			types.TxValueKeyFrom:          reservoir.GetAddr(),
			types.TxValueKeyTo:            &(contract.Addr),
			types.TxValueKeyAmount:        big.NewInt(0),
			types.TxValueKeyGasLimit:      gasLimit,
			types.TxValueKeyGasPrice:      big.NewInt(25 * params.Ston),
			types.TxValueKeyHumanReadable: true,
			types.TxValueKeyData:          common.FromHex(code),
			types.TxValueKeyCodeFormat:    params.CodeFormatEVM,
		}

		tx, err := types.NewTransactionWithMap(types.TxTypeSmartContractDeploy, values)
		assert.Equal(t, nil, err)

		err = tx.SignWithKeys(signer, reservoir.Keys)
		assert.Equal(t, nil, err)

		txs = append(txs, tx)

		if err := bcdata.GenABlockWithTransactions(accountMap, txs, prof); err != nil {
			t.Fatal(err)
		}
		reservoir.AddNonce()
	}

	// generate a test account with a specific amount of KLAY
	{
		var txs types.Transactions

		valueMapForCreation, _ := genMapForTxTypes(reservoir, reservoir, types.TxTypeAccountCreation)
		valueMapForCreation[types.TxValueKeyTo] = testAcc.Addr
		valueMapForCreation[types.TxValueKeyAccountKey] = testAcc.AccKey
		valueMapForCreation[types.TxValueKeyAmount] = cost

		tx, err := types.NewTransactionWithMap(types.TxTypeAccountCreation, valueMapForCreation)
		assert.Equal(t, nil, err)

		err = tx.SignWithKeys(signer, reservoir.Keys)
		assert.Equal(t, nil, err)

		txs = append(txs, tx)

		if err := bcdata.GenABlockWithTransactions(accountMap, txs, prof); err != nil {
			t.Fatal(err)
		}
		reservoir.AddNonce()
	}

	// test for all tx types
	for _, testTxType := range testTxTypes {
		txType := testTxType.txType

		if !txType.IsFeeDelegatedTransaction() {
			// tx with a specific amount or a gasLimit requiring more KLAY than the sender has.
			{
				var expectedErr error

				valueMap, _ := genMapForTxTypes(testAcc, reservoir, txType)
				if valueMap[types.TxValueKeyAmount] != nil {
					valueMap[types.TxValueKeyAmount] = new(big.Int).SetUint64(amount)
					valueMap[types.TxValueKeyGasLimit] = gasLimit + 1 // requires 1 more gas
					// The tx will be failed in vm since it can buy gas but cannot send enough value
					expectedErr = vm.ErrInsufficientBalance
				} else {
					valueMap[types.TxValueKeyGasLimit] = gasLimit + (amount / gasPrice.Uint64()) + 1 // requires 1 more gas
					// The tx will be failed in buyGas() since it cannot buy enough gas
					expectedErr = errInsufficientBalanceForGasFeePayer
				}

				tx, err := types.NewTransactionWithMap(txType, valueMap)
				assert.Equal(t, nil, err)

				err = tx.SignWithKeys(signer, testAcc.Keys)
				assert.Equal(t, nil, err)

				receipt, _, err := applyTransaction(t, bcdata, tx)
				assert.Equal(t, expectedErr, err)
				assert.Equal(t, (*types.Receipt)(nil), receipt)
			}

			// tx with a specific amount or a gasLimit requiring the exact KLAY the sender has.
			{
				valueMap, _ := genMapForTxTypes(testAcc, reservoir, txType)
				if valueMap[types.TxValueKeyAmount] != nil {
					valueMap[types.TxValueKeyAmount] = new(big.Int).SetUint64(amount)
					valueMap[types.TxValueKeyGasLimit] = gasLimit
				} else {
					valueMap[types.TxValueKeyGasLimit] = gasLimit + (amount / gasPrice.Uint64())
				}

				tx, err := types.NewTransactionWithMap(txType, valueMap)
				assert.Equal(t, nil, err)

				err = tx.SignWithKeys(signer, testAcc.Keys)
				assert.Equal(t, nil, err)

				receipt, _, err := applyTransaction(t, bcdata, tx)
				assert.Equal(t, nil, err)
				// contract deploy tx with non-zero value will be failed in vm because test functions do not support it.
				if txType.IsContractDeploy() {
					assert.Equal(t, types.ReceiptStatusErrExecutionReverted, receipt.Status)
				} else {
					assert.Equal(t, types.ReceiptStatusSuccessful, receipt.Status)
				}
			}
		}

		if txType.IsFeeDelegatedTransaction() && !txType.IsFeeDelegatedWithRatioTransaction() {
			// tx with a specific amount requiring more KLAY than the sender has.
			{
				valueMap, _ := genMapForTxTypes(testAcc, reservoir, txType)
				if valueMap[types.TxValueKeyAmount] != nil {
					valueMap[types.TxValueKeyFeePayer] = reservoir.Addr
					valueMap[types.TxValueKeyAmount] = new(big.Int).Add(cost, new(big.Int).SetUint64(1)) // requires 1 more amount

					tx, err := types.NewTransactionWithMap(txType, valueMap)
					assert.Equal(t, nil, err)

					err = tx.SignWithKeys(signer, testAcc.Keys)
					assert.Equal(t, nil, err)

					tx.SignFeePayerWithKeys(signer, reservoir.Keys)
					assert.Equal(t, nil, err)

					receipt, _, err := applyTransaction(t, bcdata, tx)
					assert.Equal(t, vm.ErrInsufficientBalance, err)
					assert.Equal(t, (*types.Receipt)(nil), receipt)
				}
			}

			// tx with a specific gasLimit (or amount) requiring more KLAY than the feePayer has.
			{
				valueMap, _ := genMapForTxTypes(reservoir, reservoir, txType)
				valueMap[types.TxValueKeyFeePayer] = testAcc.Addr
				valueMap[types.TxValueKeyGasLimit] = gasLimit + (amount / gasPrice.Uint64()) + 1 // requires 1 more gas

				tx, err := types.NewTransactionWithMap(txType, valueMap)
				assert.Equal(t, nil, err)

				err = tx.SignWithKeys(signer, reservoir.Keys)
				assert.Equal(t, nil, err)

				tx.SignFeePayerWithKeys(signer, testAcc.Keys)
				assert.Equal(t, nil, err)

				receipt, _, err := applyTransaction(t, bcdata, tx)
				assert.Equal(t, errInsufficientBalanceForGasFeePayer, err)
				assert.Equal(t, (*types.Receipt)(nil), receipt)
			}

			// tx with a specific amount requiring the exact KLAY the sender has.
			{
				valueMap, _ := genMapForTxTypes(testAcc, reservoir, txType)
				if valueMap[types.TxValueKeyAmount] != nil {
					valueMap[types.TxValueKeyFeePayer] = reservoir.Addr
					valueMap[types.TxValueKeyAmount] = cost

					tx, err := types.NewTransactionWithMap(txType, valueMap)
					assert.Equal(t, nil, err)

					err = tx.SignWithKeys(signer, testAcc.Keys)
					assert.Equal(t, nil, err)

					tx.SignFeePayerWithKeys(signer, reservoir.Keys)
					assert.Equal(t, nil, err)

					receipt, _, err := applyTransaction(t, bcdata, tx)
					assert.Equal(t, nil, err)
					// contract deploy tx with non-zero value will be failed in vm because test functions do not support it.
					if txType.IsContractDeploy() {
						assert.Equal(t, types.ReceiptStatusErrExecutionReverted, receipt.Status)
					} else {
						assert.Equal(t, types.ReceiptStatusSuccessful, receipt.Status)
					}
				}
			}

			// tx with a specific gasLimit (or amount) requiring the exact KLAY the feePayer has.
			{
				valueMap, _ := genMapForTxTypes(reservoir, reservoir, txType)
				valueMap[types.TxValueKeyFeePayer] = testAcc.Addr
				valueMap[types.TxValueKeyGasLimit] = gasLimit + (amount / gasPrice.Uint64())

				tx, err := types.NewTransactionWithMap(txType, valueMap)
				assert.Equal(t, nil, err)

				err = tx.SignWithKeys(signer, reservoir.Keys)
				assert.Equal(t, nil, err)

				tx.SignFeePayerWithKeys(signer, testAcc.Keys)
				assert.Equal(t, nil, err)

				receipt, _, err := applyTransaction(t, bcdata, tx)
				assert.Equal(t, nil, err)
				assert.Equal(t, types.ReceiptStatusSuccessful, receipt.Status)
			}
		}

		if txType.IsFeeDelegatedWithRatioTransaction() {
			// tx with a specific amount and a gasLimit requiring more KLAY than the sender has.
			{
				var expectedErr error
				valueMap, _ := genMapForTxTypes(testAcc, reservoir, txType)
				valueMap[types.TxValueKeyFeePayer] = reservoir.Addr
				valueMap[types.TxValueKeyFeeRatioOfFeePayer] = types.FeeRatio(90)
				if valueMap[types.TxValueKeyAmount] != nil {
					valueMap[types.TxValueKeyAmount] = new(big.Int).SetUint64(amount)
					// Gas testAcc will charge = tx gasLimit * sender's feeRatio
					// = (gasLimit + 1) * 10 * (100 - 90) * 0.01 = gasLimit + 1
					valueMap[types.TxValueKeyGasLimit] = (gasLimit + 1) * 10 // requires 1 more gas
					// The tx will be failed in vm since it can buy gas but cannot send enough value
					expectedErr = vm.ErrInsufficientBalance
				} else {
					// Gas testAcc will charge = tx gasLimit * sender's feeRatio
					// = (gasLimit + (amount / gasPrice.Uint64()) + 1) * 10 * (100 - 90) * 0.01 = gasLimit + (amount / gasPrice.Uint64()) + 1
					valueMap[types.TxValueKeyGasLimit] = (gasLimit + (amount / gasPrice.Uint64()) + 1) * 10 // requires 1 more gas
					// The tx will be failed in buyGas() since it cannot buy enough gas
					expectedErr = errInsufficientBalanceForGas
				}

				tx, err := types.NewTransactionWithMap(txType, valueMap)
				assert.Equal(t, nil, err)

				err = tx.SignWithKeys(signer, testAcc.Keys)
				assert.Equal(t, nil, err)

				tx.SignFeePayerWithKeys(signer, reservoir.Keys)
				assert.Equal(t, nil, err)

				receipt, _, err := applyTransaction(t, bcdata, tx)
				assert.Equal(t, expectedErr, err)
				assert.Equal(t, (*types.Receipt)(nil), receipt)
			}

			// tx with a specific amount and a gasLimit requiring more KLAY than the feePayer has.
			{
				valueMap, _ := genMapForTxTypes(reservoir, reservoir, txType)
				valueMap[types.TxValueKeyFeePayer] = testAcc.Addr
				valueMap[types.TxValueKeyFeeRatioOfFeePayer] = types.FeeRatio(10)
				// Gas testAcc will charge = tx gasLimit * fee-payer's feeRatio
				// = (gasLimit + (amount / gasPrice.Uint64()) + 1) * 10 * 10 * 0.01 = gasLimit + (amount / gasPrice.Uint64()) + 1
				valueMap[types.TxValueKeyGasLimit] = (gasLimit + (amount / gasPrice.Uint64()) + 1) * 10 // requires 1 more gas

				tx, err := types.NewTransactionWithMap(txType, valueMap)
				assert.Equal(t, nil, err)

				err = tx.SignWithKeys(signer, reservoir.Keys)
				assert.Equal(t, nil, err)

				tx.SignFeePayerWithKeys(signer, testAcc.Keys)
				assert.Equal(t, nil, err)

				receipt, _, err := applyTransaction(t, bcdata, tx)
				assert.Equal(t, errInsufficientBalanceForGasFeePayer, err)
				assert.Equal(t, (*types.Receipt)(nil), receipt)
			}

			// tx with a specific amount and a gasLimit requiring the exact KLAY the sender has.
			{
				valueMap, _ := genMapForTxTypes(testAcc, reservoir, txType)
				valueMap[types.TxValueKeyFeePayer] = reservoir.Addr
				valueMap[types.TxValueKeyFeeRatioOfFeePayer] = types.FeeRatio(90)
				if valueMap[types.TxValueKeyAmount] != nil {
					valueMap[types.TxValueKeyAmount] = new(big.Int).SetUint64(amount)
					// Gas testAcc will charge = tx gasLimit * sender's feeRatio
					// = gasLimit * 10 * (100 - 90) * 0.01 = gasLimit
					valueMap[types.TxValueKeyGasLimit] = gasLimit * 10
				} else {
					// Gas testAcc will charge = tx gasLimit * sender's feeRatio
					// = (gasLimit + (amount / gasPrice.Uint64())) * 10 * (100 - 90) * 0.01 = gasLimit + (amount / gasPrice.Uint64())
					valueMap[types.TxValueKeyGasLimit] = (gasLimit + (amount / gasPrice.Uint64())) * 10
				}

				tx, err := types.NewTransactionWithMap(txType, valueMap)
				assert.Equal(t, nil, err)

				err = tx.SignWithKeys(signer, testAcc.Keys)
				assert.Equal(t, nil, err)

				tx.SignFeePayerWithKeys(signer, reservoir.Keys)
				assert.Equal(t, nil, err)

				receipt, _, err := applyTransaction(t, bcdata, tx)
				assert.Equal(t, nil, err)
				// contract deploy tx with non-zero value will be failed in vm because test functions do not support it.
				if txType.IsContractDeploy() {
					assert.Equal(t, types.ReceiptStatusErrExecutionReverted, receipt.Status)
				} else {
					assert.Equal(t, types.ReceiptStatusSuccessful, receipt.Status)
				}
			}

			// tx with a specific amount and a gasLimit requiring the exact KLAY the feePayer has.
			{
				valueMap, _ := genMapForTxTypes(reservoir, reservoir, txType)
				valueMap[types.TxValueKeyFeePayer] = testAcc.Addr
				valueMap[types.TxValueKeyFeeRatioOfFeePayer] = types.FeeRatio(10)
				// Gas testAcc will charge = tx gasLimit * fee-payer's feeRatio
				// = (gasLimit + (amount / gasPrice.Uint64())) * 10 * 10 * 0.01 = gasLimit + (amount / gasPrice.Uint64())
				valueMap[types.TxValueKeyGasLimit] = (gasLimit + (amount / gasPrice.Uint64())) * 10

				tx, err := types.NewTransactionWithMap(txType, valueMap)
				assert.Equal(t, nil, err)

				err = tx.SignWithKeys(signer, reservoir.Keys)
				assert.Equal(t, nil, err)

				tx.SignFeePayerWithKeys(signer, testAcc.Keys)
				assert.Equal(t, nil, err)

				receipt, _, err := applyTransaction(t, bcdata, tx)
				assert.Equal(t, nil, err)
				assert.Equal(t, types.ReceiptStatusSuccessful, receipt.Status)
			}
		}
	}
}

// TestValidationTxSizeAfterRLP tests tx size validation during txPool insert process.
// Since the size is RLP encoded tx size, the test also includes RLP encoding/decoding process which may raise an issue.
func TestValidationTxSizeAfterRLP(t *testing.T) {
	var testTxTypes = []types.TxType{
		types.TxTypeLegacyTransaction,
		types.TxTypeValueTransferMemo,
		types.TxTypeSmartContractDeploy,
		types.TxTypeSmartContractExecution,
		types.TxTypeChainDataAnchoring,
		types.TxTypeFeeDelegatedValueTransferMemo,
		types.TxTypeFeeDelegatedSmartContractDeploy,
		types.TxTypeFeeDelegatedSmartContractExecution,
		types.TxTypeFeeDelegatedValueTransferMemoWithRatio,
		types.TxTypeFeeDelegatedSmartContractDeployWithRatio,
		types.TxTypeFeeDelegatedSmartContractExecutionWithRatio,
	}

	prof := profile.NewProfiler()

	// Initialize blockchain
	bcdata, err := NewBCData(6, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer bcdata.Shutdown()

	// Initialize address-balance map for verification
	accountMap := NewAccountMap()
	if err := accountMap.Initialize(bcdata); err != nil {
		t.Fatal(err)
	}

	signer := types.NewEIP155Signer(bcdata.bc.Config().ChainID)

	// reservoir account
	reservoir := &TestAccountType{
		Addr:  *bcdata.addrs[0],
		Keys:  []*ecdsa.PrivateKey{bcdata.privKeys[0]},
		Nonce: uint64(0),
	}

	// for contract execution txs
	contract, err := createHumanReadableAccount(getRandomPrivateKeyString(t), "contract")

	// make TxPool to test validation in 'TxPool add' process
	poolSlots := 1000
	txpoolconfig := blockchain.DefaultTxPoolConfig
	txpoolconfig.Journal = ""
	txpoolconfig.ExecSlotsAccount = uint64(poolSlots)
	txpoolconfig.NonExecSlotsAccount = uint64(poolSlots)
	txpoolconfig.ExecSlotsAll = 2 * uint64(poolSlots)
	txpoolconfig.NonExecSlotsAll = 2 * uint64(poolSlots)
	txpool := blockchain.NewTxPool(txpoolconfig, bcdata.bc.Config(), bcdata.bc)

	// deploy a contract for contract execution tx type
	{
		var txs types.Transactions

		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:         reservoir.GetNonce(),
			types.TxValueKeyFrom:          reservoir.GetAddr(),
			types.TxValueKeyTo:            &(contract.Addr),
			types.TxValueKeyAmount:        big.NewInt(0),
			types.TxValueKeyGasLimit:      gasLimit,
			types.TxValueKeyGasPrice:      big.NewInt(25 * params.Ston),
			types.TxValueKeyHumanReadable: true,
			types.TxValueKeyData:          common.FromHex(code),
			types.TxValueKeyCodeFormat:    params.CodeFormatEVM,
		}

		tx, err := types.NewTransactionWithMap(types.TxTypeSmartContractDeploy, values)
		assert.Equal(t, nil, err)

		err = tx.SignWithKeys(signer, reservoir.Keys)
		assert.Equal(t, nil, err)

		txs = append(txs, tx)

		if err := bcdata.GenABlockWithTransactions(accountMap, txs, prof); err != nil {
			t.Fatal(err)
		}
		reservoir.AddNonce()
	}

	// test for all tx types
	for _, txType := range testTxTypes {
		// test for invalid tx size
		{
			// generate invalid txs which size is around (32 * 1024) ~ (33 * 1024)
			valueMap, _ := genMapForTxTypes(reservoir, reservoir, txType)
			valueMap, _ = exceedSizeLimit(txType, valueMap)

			tx, err := types.NewTransactionWithMap(txType, valueMap)
			assert.Equal(t, nil, err)

			err = tx.SignWithKeys(signer, reservoir.Keys)
			assert.Equal(t, nil, err)

			if txType.IsFeeDelegatedTransaction() {
				tx.SignFeePayerWithKeys(signer, reservoir.Keys)
				assert.Equal(t, nil, err)
			}

			// check the rlp encoded tx size
			encodedTx, err := rlp.EncodeToBytes(tx)
			if len(encodedTx) < blockchain.MaxTxDataSize {
				t.Fatalf("test data size is smaller than MaxTxDataSize")
			}

			// RLP decode and re-generate the tx
			newTx := &types.Transaction{}
			err = rlp.DecodeBytes(encodedTx, newTx)

			// test for tx pool insert validation
			err = txpool.AddRemote(newTx)
			assert.Equal(t, blockchain.ErrOversizedData, err)
		}

		// test for valid tx size
		{
			// generate valid txs which size is around (31 * 1024) ~ (32 * 1024)
			valueMap, _ := genMapForTxTypes(reservoir, reservoir, txType)
			validData := make([]byte, blockchain.MaxTxDataSize-1024)

			if valueMap[types.TxValueKeyData] != nil {
				valueMap[types.TxValueKeyData] = validData
			}

			if valueMap[types.TxValueKeyAnchoredData] != nil {
				valueMap[types.TxValueKeyAnchoredData] = validData
			}

			tx, err := types.NewTransactionWithMap(txType, valueMap)
			assert.Equal(t, nil, err)

			err = tx.SignWithKeys(signer, reservoir.Keys)
			assert.Equal(t, nil, err)

			if txType.IsFeeDelegatedTransaction() {
				tx.SignFeePayerWithKeys(signer, reservoir.Keys)
				assert.Equal(t, nil, err)
			}

			// check the rlp encoded tx size
			encodedTx, err := rlp.EncodeToBytes(tx)
			if len(encodedTx) > blockchain.MaxTxDataSize {
				t.Fatalf("test data size is bigger than MaxTxDataSize")
			}

			// RLP decode and re-generate the tx
			newTx := &types.Transaction{}
			err = rlp.DecodeBytes(encodedTx, newTx)

			// test for tx pool insert validation
			err = txpool.AddRemote(newTx)
			assert.Equal(t, nil, err)
			reservoir.AddNonce()
		}
	}
}

// TestValidationPoolResetAfterSenderKeyChange puts txs in the pending pool and generates a block only with the first tx.
// Since the tx changes the sender's account key, all rest txs should drop from the pending pool.
func TestValidationPoolResetAfterSenderKeyChange(t *testing.T) {
	txTypes := []types.TxType{
		types.TxTypeLegacyTransaction,

		types.TxTypeValueTransfer,
		types.TxTypeValueTransferMemo,
		types.TxTypeSmartContractDeploy,
		types.TxTypeSmartContractExecution,
		types.TxTypeAccountCreation,
		types.TxTypeAccountUpdate,
		types.TxTypeCancel,

		types.TxTypeFeeDelegatedValueTransfer,
		types.TxTypeFeeDelegatedValueTransferMemo,
		types.TxTypeFeeDelegatedSmartContractDeploy,
		types.TxTypeFeeDelegatedSmartContractExecution,
		types.TxTypeFeeDelegatedAccountUpdate,
		types.TxTypeFeeDelegatedCancel,

		types.TxTypeFeeDelegatedValueTransferWithRatio,
		types.TxTypeFeeDelegatedValueTransferMemoWithRatio,
		types.TxTypeFeeDelegatedSmartContractDeployWithRatio,
		types.TxTypeFeeDelegatedSmartContractExecutionWithRatio,
		types.TxTypeFeeDelegatedAccountUpdateWithRatio,
		types.TxTypeFeeDelegatedCancelWithRatio,

		types.TxTypeChainDataAnchoring,
	}

	prof := profile.NewProfiler()

	// Initialize blockchain
	bcdata, err := NewBCData(6, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer bcdata.Shutdown()

	// Initialize address-balance map for verification
	accountMap := NewAccountMap()
	if err := accountMap.Initialize(bcdata); err != nil {
		t.Fatal(err)
	}

	signer := types.NewEIP155Signer(bcdata.bc.Config().ChainID)

	// reservoir account
	reservoir := &TestAccountType{
		Addr:  *bcdata.addrs[0],
		Keys:  []*ecdsa.PrivateKey{bcdata.privKeys[0]},
		Nonce: uint64(0),
	}

	// for contract execution txs
	contract, err := createHumanReadableAccount(getRandomPrivateKeyString(t), "contract")
	assert.Equal(t, nil, err)

	// make TxPool to test validation in 'TxPool add' process
	poolSlots := 1000
	txpoolconfig := blockchain.DefaultTxPoolConfig
	txpoolconfig.Journal = ""
	txpoolconfig.ExecSlotsAccount = uint64(poolSlots)
	txpoolconfig.NonExecSlotsAccount = uint64(poolSlots)
	txpoolconfig.ExecSlotsAll = 2 * uint64(poolSlots)
	txpoolconfig.NonExecSlotsAll = 2 * uint64(poolSlots)
	txpool := blockchain.NewTxPool(txpoolconfig, bcdata.bc.Config(), bcdata.bc)

	// deploy a contract for contract execution tx type
	{
		var txs types.Transactions

		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:         reservoir.GetNonce(),
			types.TxValueKeyFrom:          reservoir.GetAddr(),
			types.TxValueKeyTo:            &(contract.Addr),
			types.TxValueKeyAmount:        big.NewInt(0),
			types.TxValueKeyGasLimit:      gasLimit,
			types.TxValueKeyGasPrice:      big.NewInt(25 * params.Ston),
			types.TxValueKeyHumanReadable: true,
			types.TxValueKeyData:          common.FromHex(code),
			types.TxValueKeyCodeFormat:    params.CodeFormatEVM,
		}

		tx, err := types.NewTransactionWithMap(types.TxTypeSmartContractDeploy, values)
		assert.Equal(t, nil, err)

		err = tx.SignWithKeys(signer, reservoir.Keys)
		assert.Equal(t, nil, err)

		txs = append(txs, tx)

		if err := bcdata.GenABlockWithTransactions(accountMap, txs, prof); err != nil {
			t.Fatal(err)
		}
		reservoir.AddNonce()
	}

	// state changing tx which will invalidate other txs when it is contained in a block.
	var txs types.Transactions
	{
		valueMap, _ := genMapForTxTypes(reservoir, reservoir, types.TxTypeAccountUpdate)
		tx, err := types.NewTransactionWithMap(types.TxTypeAccountUpdate, valueMap)

		assert.Equal(t, nil, err)

		err = tx.SignWithKeys(signer, reservoir.Keys)
		assert.Equal(t, nil, err)

		txs = append(txs, tx)

		err = txpool.AddRemote(tx)
		assert.Equal(t, nil, err)
		reservoir.AddNonce()
	}

	// generate valid txs with all tx types.
	for _, txType := range txTypes {
		txType := txType

		valueMap, _ := genMapForTxTypes(reservoir, reservoir, txType)
		tx, err := types.NewTransactionWithMap(txType, valueMap)
		assert.Equal(t, nil, err)

		err = tx.SignWithKeys(signer, reservoir.Keys)
		assert.Equal(t, nil, err)

		if txType.IsFeeDelegatedTransaction() {
			tx.SignFeePayerWithKeys(signer, reservoir.Keys)
			assert.Equal(t, nil, err)
		}

		err = txpool.AddRemote(tx)
		assert.Equal(t, nil, err)
		reservoir.AddNonce()
	}

	// check pending whether it contains all txs
	pendingLen, _ := txpool.Stats()
	assert.Equal(t, len(txTypes)+1, pendingLen)

	// generate a block with a state changing tx
	if err := bcdata.GenABlockWithTransactions(accountMap, txs, prof); err != nil {
		t.Fatal(err)
	}

	// check pending whether it contains zero tx
	pendingLen, _ = txpool.Stats()
	assert.Equal(t, 0, pendingLen)
}

// TestValidationPoolResetAfterFeePayerKeyChange puts txs in the pending pool and generates a block only with the first tx.
// Since the tx changes the fee payer's account key, all rest txs should drop from the pending pool.
func TestValidationPoolResetAfterFeePayerKeyChange(t *testing.T) {
	txTypes := []types.TxType{
		types.TxTypeFeeDelegatedValueTransfer,
		types.TxTypeFeeDelegatedValueTransferMemo,
		types.TxTypeFeeDelegatedSmartContractDeploy,
		types.TxTypeFeeDelegatedSmartContractExecution,
		types.TxTypeFeeDelegatedAccountUpdate,
		types.TxTypeFeeDelegatedCancel,

		types.TxTypeFeeDelegatedValueTransferWithRatio,
		types.TxTypeFeeDelegatedValueTransferMemoWithRatio,
		types.TxTypeFeeDelegatedSmartContractDeployWithRatio,
		types.TxTypeFeeDelegatedSmartContractExecutionWithRatio,
		types.TxTypeFeeDelegatedAccountUpdateWithRatio,
		types.TxTypeFeeDelegatedCancelWithRatio,
	}

	prof := profile.NewProfiler()

	// Initialize blockchain
	bcdata, err := NewBCData(6, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer bcdata.Shutdown()

	// Initialize address-balance map for verification
	accountMap := NewAccountMap()
	if err := accountMap.Initialize(bcdata); err != nil {
		t.Fatal(err)
	}

	signer := types.NewEIP155Signer(bcdata.bc.Config().ChainID)

	// reservoir account
	reservoir := &TestAccountType{
		Addr:  *bcdata.addrs[0],
		Keys:  []*ecdsa.PrivateKey{bcdata.privKeys[0]},
		Nonce: uint64(0),
	}

	// for contract execution txs
	contract, err := createHumanReadableAccount(getRandomPrivateKeyString(t), "contract")
	assert.Equal(t, nil, err)

	// fee payer account
	feePayer, err := createDefaultAccount(accountkey.AccountKeyTypePublic)
	assert.Equal(t, nil, err)

	// make TxPool to test validation in 'TxPool add' process
	poolSlots := 1000
	txpoolconfig := blockchain.DefaultTxPoolConfig
	txpoolconfig.Journal = ""
	txpoolconfig.ExecSlotsAccount = uint64(poolSlots)
	txpoolconfig.NonExecSlotsAccount = uint64(poolSlots)
	txpoolconfig.ExecSlotsAll = 2 * uint64(poolSlots)
	txpoolconfig.NonExecSlotsAll = 2 * uint64(poolSlots)
	txpool := blockchain.NewTxPool(txpoolconfig, bcdata.bc.Config(), bcdata.bc)

	// deploy a contract for contract execution tx type
	{
		var txs types.Transactions

		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:         reservoir.GetNonce(),
			types.TxValueKeyFrom:          reservoir.GetAddr(),
			types.TxValueKeyTo:            &(contract.Addr),
			types.TxValueKeyAmount:        big.NewInt(0),
			types.TxValueKeyGasLimit:      gasLimit,
			types.TxValueKeyGasPrice:      big.NewInt(25 * params.Ston),
			types.TxValueKeyHumanReadable: true,
			types.TxValueKeyData:          common.FromHex(code),
			types.TxValueKeyCodeFormat:    params.CodeFormatEVM,
		}

		tx, err := types.NewTransactionWithMap(types.TxTypeSmartContractDeploy, values)
		assert.Equal(t, nil, err)

		err = tx.SignWithKeys(signer, reservoir.Keys)
		assert.Equal(t, nil, err)

		txs = append(txs, tx)

		if err := bcdata.GenABlockWithTransactions(accountMap, txs, prof); err != nil {
			t.Fatal(err)
		}
		reservoir.AddNonce()
	}

	// deploy a contract for contract execution tx type
	{
		var txs types.Transactions

		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:         reservoir.GetNonce(),
			types.TxValueKeyFrom:          reservoir.GetAddr(),
			types.TxValueKeyTo:            feePayer.Addr,
			types.TxValueKeyAmount:        new(big.Int).Mul(big.NewInt(params.KLAY), big.NewInt(100000)),
			types.TxValueKeyGasLimit:      gasLimit,
			types.TxValueKeyGasPrice:      big.NewInt(25 * params.Ston),
			types.TxValueKeyHumanReadable: false,
			types.TxValueKeyAccountKey:    feePayer.AccKey,
		}

		tx, err := types.NewTransactionWithMap(types.TxTypeAccountCreation, values)
		assert.Equal(t, nil, err)

		err = tx.SignWithKeys(signer, reservoir.Keys)
		assert.Equal(t, nil, err)

		txs = append(txs, tx)

		if err := bcdata.GenABlockWithTransactions(accountMap, txs, prof); err != nil {
			t.Fatal(err)
		}
		reservoir.AddNonce()
	}

	// state changing tx which will invalidate other txs when it is contained in a block.
	var txs types.Transactions
	{
		valueMap, _ := genMapForTxTypes(feePayer, feePayer, types.TxTypeAccountUpdate)
		tx, err := types.NewTransactionWithMap(types.TxTypeAccountUpdate, valueMap)

		assert.Equal(t, nil, err)

		err = tx.SignWithKeys(signer, feePayer.Keys)
		assert.Equal(t, nil, err)

		txs = append(txs, tx)

		err = txpool.AddRemote(tx)
		assert.Equal(t, nil, err)
		feePayer.AddNonce()
	}

	// generate valid txs with all tx fee delegation types.
	for _, txType := range txTypes {
		txType := txType

		valueMap, _ := genMapForTxTypes(reservoir, reservoir, txType)
		valueMap[types.TxValueKeyFeePayer] = feePayer.Addr

		tx, err := types.NewTransactionWithMap(txType, valueMap)
		assert.Equal(t, nil, err)

		err = tx.SignWithKeys(signer, reservoir.Keys)
		assert.Equal(t, nil, err)

		tx.SignFeePayerWithKeys(signer, feePayer.Keys)
		assert.Equal(t, nil, err)

		err = txpool.AddRemote(tx)
		assert.Equal(t, nil, err)
		reservoir.AddNonce()
	}

	// check pending whether it contains all txs
	pendingLen, _ := txpool.Stats()
	assert.Equal(t, len(txTypes)+1, pendingLen)

	// generate a block with a state changing tx
	if err := bcdata.GenABlockWithTransactions(accountMap, txs, prof); err != nil {
		t.Fatal(err)
	}

	// check pending whether it contains zero tx
	pendingLen, _ = txpool.Stats()
	assert.Equal(t, 0, pendingLen)
}
