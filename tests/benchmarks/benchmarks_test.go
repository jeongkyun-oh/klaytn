// Copyright 2018 The go-klaytn Authors
//
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

package benchmarks

import (
	"testing"

	"github.com/ground-x/go-gxplatform/common"
)


func BenchmarkInterpreterMload100000(bench *testing.B) {
	//
	// Test code
	//       Initialize memory with memory write (PUSH PUSH MSTORE)
	//       Loop 10000 times for below code
	//              memory read 10 times //  (PUSH MLOAD POP) x 10
	//
	code := common.Hex2Bytes("60ca60205260005b612710811015630000004557602051506020515060205150602051506020515060205150602051506020515060205150602051506001016300000007565b00")
	intrp, contract := prepareInterpreterAndContract(code)

	bench.ResetTimer()
	for i := 0; i < bench.N; i++ {
		intrp.Run(contract, nil)
	}
}
