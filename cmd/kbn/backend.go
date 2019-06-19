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

package main

import (
	"github.com/ground-x/klaytn/networks/p2p/discover"
	"github.com/ground-x/klaytn/networks/rpc"
)

type BN struct {
	ntab discover.Discovery
}

func NewBN(t discover.Discovery) *BN {
	return &BN{ntab: t}
}

func (b *BN) Name() string {
	return b.ntab.Name()
}

func (b *BN) Resolve(target discover.NodeID, targetType discover.NodeType) *discover.Node {
	return b.ntab.Resolve(target, targetType)
}

func (b *BN) Lookup(target discover.NodeID, targetType discover.NodeType) []*discover.Node {
	return b.ntab.Lookup(target, targetType)
}

func (b *BN) ReadRandomNodes(buf []*discover.Node, nType discover.NodeType) int {
	return b.ntab.ReadRandomNodes(buf, nType)
}

func (b *BN) CreateUpdateNodeOnDB(nodekni string) error {
	node, err := discover.ParseNode(nodekni)
	if err != nil {
		return err
	}
	return b.ntab.CreateUpdateNodeOnDB(node)
}

func (b *BN) CreateUpdateNodeOnTable(nodekni string) error {
	node, err := discover.ParseNode(nodekni)
	if err != nil {
		return nil
	}
	return b.ntab.CreateUpdateNodeOnTable(node)
}

func (b *BN) GetNodeFromDB(id discover.NodeID) (*discover.Node, error) {
	return b.ntab.GetNodeFromDB(id)
}

func (b *BN) GetTableEntries() []*discover.Node {
	return b.ntab.GetBucketEntries()
}

func (b *BN) GetTableReplacements() []*discover.Node {
	return b.ntab.GetReplacements()
}

func (b *BN) DeleteNodeFromDB(nodekni string) error {
	node, err := discover.ParseNode(nodekni)
	if err != nil {
		return err
	}
	return b.ntab.DeleteNodeFromDB(node)
}

func (b *BN) DeleteNodeFromTable(nodekni string) error {
	node, err := discover.ParseNode(nodekni)
	if err != nil {
		return err
	}
	return b.ntab.DeleteNodeFromTable(node)
}

func (b *BN) APIs() []rpc.API {
	return []rpc.API{
		{
			Namespace: "bootnode",
			Version:   "1.0",
			Service:   NewBootnodeAPI(b),
			Public:    true,
		},
	}
}
