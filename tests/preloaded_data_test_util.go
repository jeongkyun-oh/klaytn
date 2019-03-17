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

package tests

import (
	"bufio"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/crypto"
	"os"
	"path"
	"strconv"
	"sync"
)

const (
	addressDirectory     = "addrs"
	privateKeyDirectory  = "privatekeys"
	addressFilePrefix    = "addrs_"
	privateKeyFilePrefix = "privateKeys_"
)

func writeToFile(addrs []*common.Address, privKeys []*ecdsa.PrivateKey, num int) error {
	addrsFile, err := os.Create(addressFilePrefix + strconv.Itoa(num))
	if err != nil {
		return err
	}

	privateKeysFile, err := os.Create(privateKeyFilePrefix + strconv.Itoa(num))
	if err != nil {
		return err
	}

	wg := sync.WaitGroup{}

	wg.Add(2)

	syncSize := len(addrs) / 2

	go func() {
		for i, b := range addrs {
			addrsFile.WriteString(b.String() + "\n")
			if (i+1)%syncSize == 0 {
				addrsFile.Sync()
			}
		}

		addrsFile.Close()
		wg.Done()
	}()

	go func() {
		for i, key := range privKeys {
			privateKeysFile.WriteString(hex.EncodeToString(crypto.FromECDSA(key)) + "\n")
			if (i+1)%syncSize == 0 {
				privateKeysFile.Sync()
			}
		}

		privateKeysFile.Close()
		wg.Done()
	}()

	wg.Wait()
	return nil
}

func readAddrsFromFile(dir string, num int) ([]*common.Address, error) {
	var addrs []*common.Address

	addrsFile, err := os.Open(path.Join(dir, addressDirectory, addressFilePrefix+strconv.Itoa(num)))
	if err != nil {
		return nil, err
	}

	defer addrsFile.Close()

	scanner := bufio.NewScanner(addrsFile)
	for scanner.Scan() {
		keyStr := scanner.Text()
		addr := common.HexToAddress(keyStr)
		addrs = append(addrs, &addr)
	}

	return addrs, nil
}

func readPrivateKeysFromFile(dir string, num int) ([]*ecdsa.PrivateKey, error) {
	var privKeys []*ecdsa.PrivateKey
	privateKeysFile, err := os.Open(path.Join(dir, privateKeyDirectory, privateKeyFilePrefix+strconv.Itoa(num)))
	if err != nil {
		return nil, err
	}

	defer privateKeysFile.Close()

	scanner := bufio.NewScanner(privateKeysFile)
	for scanner.Scan() {
		keyStr := scanner.Text()

		key, err := hex.DecodeString(keyStr)
		if err != nil {
			return nil, fmt.Errorf("%v", err)
		}

		if pk, err := crypto.ToECDSA(key); err != nil {
			return nil, fmt.Errorf("%v", err)
		} else {
			privKeys = append(privKeys, pk)
		}
	}

	return privKeys, nil
}

func readAddrsAndPrivateKeysFromFile(dir string, num int) ([]*common.Address, []*ecdsa.PrivateKey, error) {
	addrs, err := readAddrsFromFile(dir, num)
	if err != nil {
		return nil, nil, err
	}

	privateKeys, err := readPrivateKeysFromFile(dir, num)
	if err != nil {
		return nil, nil, err
	}

	return addrs, privateKeys, nil
}