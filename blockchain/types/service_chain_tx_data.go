// Copyright 2018 The go-klaytn Authors
// This file is part of the go-klaytn library.
//
// The go-klaytn library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-klaytn library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-klaytn library. If not, see <http://www.gnu.org/licenses/>.

package types

import (
	"github.com/ground-x/go-gxplatform/common"
	"math/big"
)

type ServiceChainTxData struct {
	BlockHash     common.Hash
	TxHash        common.Hash
	ParentHash    common.Hash
	ReceiptHash   common.Hash
	StateRootHash common.Hash
	BlockNumber   *big.Int
}

func NewServiceChainTxData(block *Block) *ServiceChainTxData {
	return &ServiceChainTxData{block.Hash(), block.Header().TxHash,
		block.Header().ParentHash, block.Header().ReceiptHash,
		block.Header().Root, block.Header().Number}
}
