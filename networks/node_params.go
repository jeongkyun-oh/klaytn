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

package networks

import (
	"fmt"
	"github.com/ground-x/klaytn/networks/p2p/discover"
	"strings"
)

type ConnType int

const (
	CONSENSUSNODE = iota
	ENDPOINTNODE
	PROXYNODE
	BOOTNODE
	UNKNOWNNODE
)

func ConvertStringToNodeType(nodetype string) ConnType {
	switch strings.ToUpper(nodetype) {
	case "CN":
		return CONSENSUSNODE
	case "EN":
		return ENDPOINTNODE
	case "PN":
		return PROXYNODE
	case "BN":
		return BOOTNODE
	default:
		return UNKNOWNNODE
	}
}

func ConvertNodeTypeToString(nodetype ConnType) string {
	switch nodetype {
	case CONSENSUSNODE:
		return "CN"
	case ENDPOINTNODE:
		return "EN"
	case PROXYNODE:
		return "PN"
	case BOOTNODE:
		return "BN"
	default:
		return "UNKNOWN"
	}
}

func ConvertNodeType(ct ConnType) discover.NodeType {
	switch ct {
	case CONSENSUSNODE:
		return discover.NodeTypeCN
	case PROXYNODE:
		return discover.NodeTypePN
	case ENDPOINTNODE:
		return discover.NodeTypeEN
	case BOOTNODE:
		return discover.NodeTypeBN
	default:
		return discover.NodeTypeUnknown // TODO-Klaytn-Node Maybe, call panic() func or Crit()
	}
}

func ConvertConnType(nt discover.NodeType) ConnType {
	switch nt {
	case discover.NodeTypeCN:
		return CONSENSUSNODE
	case discover.NodeTypePN:
		return PROXYNODE
	case discover.NodeTypeEN:
		return ENDPOINTNODE
	case discover.NodeTypeBN:
		return BOOTNODE
	default:
		return UNKNOWNNODE
	}
}

func (ct ConnType) Valid() bool {
	if int(ct) > 255 {
		return false
	}
	return true
}

func (c ConnType) String() string {
	s := fmt.Sprintf("%d", int(c))
	return s
}
