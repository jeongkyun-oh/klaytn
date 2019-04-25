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
	"net"
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

func (b *BN) Resolve(target discover.NodeID) *discover.Node {
	return b.ntab.Resolve(target)
}

func (b *BN) Lookup(target discover.NodeID) []*discover.Node {
	return b.ntab.Lookup(target)
}

func (b *BN) ReadRandomNodes(buf []*discover.Node) int {
	return b.ntab.ReadRandomNodes(buf)
}

func (b *BN) CreateUpdateNode(id discover.NodeID, ip net.IP, udpPort, tcpPort uint16) error {
	node := discover.NewNode(id, ip, udpPort, tcpPort)
	return b.ntab.CreateUpdateNode(node)
}

func (b *BN) GetNode(id discover.NodeID) (*discover.Node, error) {
	return b.ntab.GetNode(id)
}

func (b *BN) GetTableEntries() []*discover.Node {
	return b.ntab.GetBucketEntries()
}

func (b *BN) GetTableReplacements() []*discover.Node {
	return b.ntab.GetReplacements()
}

func (b *BN) DeleteNode(id discover.NodeID) error {
	return b.ntab.DeleteNode(id)
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
