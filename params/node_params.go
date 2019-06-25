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

package params

import (
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
	switch strings.ToLower(nodetype) {
	case "cn":
		return CONSENSUSNODE
	case "en":
		return ENDPOINTNODE
	case "pn":
		return PROXYNODE
	case "bn":
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
