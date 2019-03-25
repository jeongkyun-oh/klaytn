// Copyright 2018 The klaytn Authors
// Copyright 2017 AMIS Technologies
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package compose

import (
	"bytes"
	"fmt"
	"github.com/ground-x/klaytn/cmd/homi/docker/service"
	"strings"
	"text/template"
)

type Homi struct {
	IPPrefix          string
	EthStats          *service.EthStats
	Services          []*service.Validator
	PrometheusService *service.PrometheusService
	GrafanaService    *service.GrafanaService
	UseGrafana        bool
	Proxies           []*service.Validator
	UseTxGen          bool
	TxGenService      *service.TxGenService
	TxGenOpt          service.TxGenOption
}

func New(ipPrefix string, number int, secret string, addresses []string, nodeKeys []string,
	genesis string, staticCNNodes string, staticPNnodes string, dockerImageId string, useFastHttp bool, networkId int,
	useGrafana bool, proxyNodeKeys []string, enNodeKeys []string, useTxGen bool, txGenOpt service.TxGenOption) *Homi {
	ist := &Homi{
		IPPrefix:   ipPrefix,
		EthStats:   service.NewEthStats(fmt.Sprintf("%v.9", ipPrefix), secret),
		UseGrafana: useGrafana,
		UseTxGen:   useTxGen,
	}
	ist.init(number, addresses, nodeKeys, genesis, staticCNNodes, staticPNnodes, dockerImageId, useFastHttp, networkId, proxyNodeKeys, enNodeKeys, txGenOpt)
	return ist
}

func (ist *Homi) init(number int, addresses []string, nodeKeys []string, genesis string, staticCNNodes string, staticPNNodes string, dockerImageId string, useFastHttp bool, networkId int, proxyNodeKeys []string, enNodeKeys []string, txGenOpt service.TxGenOption) {
	var validatorNames []string
	for i := 0; i < number; i++ {
		s := service.NewValidator(i,
			genesis,
			addresses[i],
			nodeKeys[i],
			"",
			32323+i,
			8551+i,
			61001+i,
			ist.EthStats.Host(),
			// from subnet ip 10
			fmt.Sprintf("%v.%v", ist.IPPrefix, i+10),
			dockerImageId,
			useFastHttp,
			networkId,
			"CN",
			"cn",
			false,
		)

		staticCNNodes = strings.Replace(staticCNNodes, "0.0.0.0", s.IP, 1)
		ist.Services = append(ist.Services, s)
		validatorNames = append(validatorNames, s.Name)
	}

	numPNs := len(proxyNodeKeys)
	for i := 0; i < numPNs; i++ {
		s := service.NewValidator(i,
			genesis,
			"",
			proxyNodeKeys[i],
			"",
			32323+number+i,
			8551+number+i,
			61001+number+i,
			ist.EthStats.Host(),
			// from subnet ip 10
			fmt.Sprintf("%v.%v", ist.IPPrefix, number+i+10),
			dockerImageId,
			useFastHttp,
			networkId,
			"PN",
			"pn",
			false,
		)

		staticCNNodes = strings.Replace(staticCNNodes, "0.0.0.0", s.IP, 1)
		ist.Services = append(ist.Services, s)
		validatorNames = append(validatorNames, s.Name)
	}

	for i := 0; i < len(enNodeKeys); i++ {
		s := service.NewValidator(i,
			genesis,
			"",
			enNodeKeys[i],
			"",
			32323+number+numPNs+i,
			8551+number+numPNs+i,
			61001+number+numPNs+i,
			ist.EthStats.Host(),
			// from subnet ip 10
			fmt.Sprintf("%v.%v", ist.IPPrefix, number+numPNs+i+10),
			dockerImageId,
			useFastHttp,
			networkId,
			"EN",
			"en",
			false,
		)

		staticPNNodes = strings.Replace(staticPNNodes, "0.0.0.0", s.IP, 1)
		ist.Services = append(ist.Services, s)
		validatorNames = append(validatorNames, s.Name)
	}

	// update static nodes
	for i := range ist.Services {
		ist.Services[i].StaticNodes = staticCNNodes
	}

	ist.PrometheusService = service.NewPrometheusService(
		fmt.Sprintf("%v.%v", ist.IPPrefix, 9),
		validatorNames)

	if ist.UseGrafana {
		ist.GrafanaService = service.NewGrafanaService(fmt.Sprintf("%v.%v", ist.IPPrefix, 8))
	}

	ist.TxGenService = service.NewTxGenService(
		fmt.Sprintf("%v.%v", ist.IPPrefix, 7),
		fmt.Sprintf("http://%v.%v:8551", ist.IPPrefix, number+10),
		txGenOpt)
}

func (ist Homi) String() string {
	tmpl, err := template.New("istanbul").Parse(istanbulTemplate)
	if err != nil {
		fmt.Printf("Failed to parse template, %v", err)
		return ""
	}

	result := new(bytes.Buffer)
	err = tmpl.Execute(result, ist)
	if err != nil {
		fmt.Printf("Failed to render template, %v", err)
		return ""
	}

	return result.String()
}

var istanbulTemplate = `version: '3'
services:
  {{- range .Services }}
  {{ . }}
  {{- end }}
  {{- range .Proxies }}
  {{ . }}
  {{- end }}
  {{ .PrometheusService }}
  {{- if .UseGrafana }}
  {{ .GrafanaService }}
  {{- end }}
  {{- if .UseTxGen }}
  {{ .TxGenService }}
  {{- end }}
networks:
  app_net:
    driver: bridge
    ipam:
      driver: default
      config:
      - subnet: {{ .IPPrefix }}.0/24`
