// Copyright 2019 The klaytn Authors
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

package discover

import (
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/common/math"
	"github.com/ground-x/klaytn/networks/p2p/netutil"
	"sync"
	"time"
)

type simpleStorage struct {
	tab        *Table
	targetType NodeType
	nodes      []*Node
	noDiscover bool // if noDiscover is true, don't lookup new node.
	nodesMutex sync.Mutex
}

func (s *simpleStorage) init() {
	// TODO
}

func (s *simpleStorage) lookup(targetID NodeID, refreshIfEmpty bool, targetType NodeType) []*Node {
	// check exist alive bn
	var seeds []*Node
	s.nodesMutex.Lock()
	for _, n := range s.nodes {
		if n.NType == NodeTypeBN {
			seeds = append(seeds, n)
		}
	}
	s.nodesMutex.Unlock()

	if len(seeds) == 0 {
		seeds := append([]*Node{}, s.tab.nursery...)
		seeds = s.tab.bondall(seeds)
		for _, n := range seeds {
			s.add(n)
		}
	}
	return s.tab.findNewNode(&nodesByDistance{entries: seeds}, targetID, targetType, false)
}

func (s *simpleStorage) doRevalidate() {
	s.nodesMutex.Lock()
	defer s.nodesMutex.Unlock()

	if len(s.nodes) == 0 {
		return
	}

	oldest := s.nodes[len(s.nodes)-1]

	holdingTime := s.tab.db.bondTime(oldest.ID).Add(10 * time.Second) // TODO-Klaytn-Node Make sleep time as configurable
	if time.Now().Before(holdingTime) {
		return
	}

	err := s.tab.ping(oldest.ID, oldest.addr())

	if err != nil {
		logger.Info("Removed dead node", "name", s.name(), "ID", oldest.ID, "NodeType", oldest.NType)
		s.deleteWithoutLock(oldest)
		return
	}
	copy(s.nodes[1:], s.nodes[:len(s.nodes)-1])
	s.nodes[0] = oldest
}

func (s *simpleStorage) setTargetNodeType(tType NodeType) {
	s.targetType = tType
}

func (s *simpleStorage) doRefresh() {
	if s.noDiscover {
		return
	}
	s.lookup(s.tab.self.ID, false, s.targetType)
}

func (s *simpleStorage) nodeAll() []*Node {
	s.nodesMutex.Lock()
	defer s.nodesMutex.Unlock()
	return s.nodes
}

func (s *simpleStorage) len() (n int) {
	s.nodesMutex.Lock()
	defer s.nodesMutex.Unlock()
	return len(s.nodes)
}

func (s *simpleStorage) copyBondedNodes() {
	s.nodesMutex.Lock()
	defer s.nodesMutex.Unlock()
	for _, n := range s.nodes {
		s.tab.db.updateNode(n)
	}
}

func (s *simpleStorage) getBucketEntries() []*Node {
	s.nodesMutex.Lock()
	defer s.nodesMutex.Unlock()

	var ret []*Node
	for _, n := range s.nodes {
		ret = append(ret, n)
	}
	return ret
}

// The caller must not hold tab.mutex.
func (s *simpleStorage) stuff(nodes []*Node) {
	panic("implement me")
}

// The caller must hold s.nodesMutex.
func (s *simpleStorage) delete(n *Node) {
	s.deleteWithLock(n)
}

func (s *simpleStorage) deleteWithLock(n *Node) {
	s.nodesMutex.Lock()
	defer s.nodesMutex.Unlock()
	s.deleteWithoutLock(n)
}

func (s *simpleStorage) deleteWithoutLock(n *Node) {
	s.nodes = deleteNode(s.nodes, n)

	s.tab.db.deleteNode(n.ID)
	if netutil.IsLAN(n.IP) {
		return
	}
	s.tab.ips.Remove(n.IP)
}

func (s *simpleStorage) closest(target common.Hash, nresults int) *nodesByDistance {
	s.nodesMutex.Lock()
	defer s.nodesMutex.Unlock()
	// TODO-Klaytn-Node nodesByDistance is not suitable for SimpleStorage. Because there is no concept for distance
	// in the SimpleStorage. Change it
	cNodes := &nodesByDistance{target: target}
	for _, n := range s.nodes {
		cNodes.push(n, nresults)
	}
	return cNodes
}

func (s *simpleStorage) setTable(t *Table) {
	s.tab = t
}

func (s *simpleStorage) readRandomNodes(buf []*Node) (n int) {
	panic("implement me")
}

func (s *simpleStorage) add(n *Node) {
	s.nodesMutex.Lock()
	s.bumpOrAdd(n)
	s.nodesMutex.Unlock()
}

// The caller must hold s.nodesMutex.
func (s *simpleStorage) bumpOrAdd(n *Node) bool {
	if s.bump(n) {
		logger.Debug("SimpleStorage-Add(Bumped)", "name", s.name(), "node", n)
		return true
	}

	logger.Debug("SimpleStorage-Add(New)", "name", s.name(), "node", n)
	s.nodes, _ = pushNode(s.nodes, n, math.MaxInt64) // TODO-Klaytn-Node Change Max value for more reasonable one.
	n.addedAt = time.Now()
	if s.tab.nodeAddedHook != nil {
		s.tab.nodeAddedHook(n)
	}
	return true
}

// The caller must hold s.nodesMutex.
func (s *simpleStorage) bump(n *Node) bool {
	for i := range s.nodes {
		if s.nodes[i].ID == n.ID {
			// move it to the front
			copy(s.nodes[1:], s.nodes[:i])
			s.nodes[0] = n
			return true
		}
	}
	return false
}

func (s *simpleStorage) name() string {
	return nodeTypeName(s.targetType)
}