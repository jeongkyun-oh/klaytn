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

package setup

import (
	"encoding/json"
	"fmt"
	"github.com/ground-x/klaytn/accounts/keystore"
	"github.com/ground-x/klaytn/cmd/homi/docker/compose"
	"github.com/ground-x/klaytn/cmd/homi/docker/service"
	"github.com/ground-x/klaytn/cmd/homi/genesis"
	"github.com/ground-x/klaytn/cmd/utils"
	"github.com/ground-x/klaytn/params"
	"gopkg.in/urfave/cli.v1"
	"io/ioutil"
	"math/big"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"crypto/ecdsa"
	istcommon "github.com/ground-x/klaytn/cmd/homi/common"
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/networks/p2p/discover"
)

type ValidatorInfo struct {
	Address  common.Address
	Nodekey  string
	NodeInfo string
}

type GrafanaFile struct {
	url  string
	name string
}

var (
	SetupCommand = cli.Command{
		Name:  "setup",
		Usage: "Generate klaytn CN's init files",
		Description: `This tool helps generate:
		* Genesis Block (genesis.json)
		* Static nodes for all CNs(Consensus Node)
		* CN details
		* Docker-compose

		for Klaytn Consensus Node.

Args :
		type : [local | remote | deploy | docker (default)]
`,
		Action: gen,
		Flags: []cli.Flag{
			cliqueFlag,
			numOfCNsFlag,
			unitPriceFlag,
			deriveShaImplFlag,
			fundingAddrFlag,
			outputPathFlag,
			dockerImageIdFlag,
			fasthttpFlag,
			networkIdFlag,
			nografanaFlag,
			numOfPNsFlag,
			useTxGenFlag,
			txGenRateFlag,
			txGenThFlag,
			txGenConnFlag,
			txGenDurFlag,
			rpcPortFlag,
			wsPortFlag,
			p2pPortFlag,
			dataDirFlag,
			logDirFlag,
			govModeFlag,
			governingNodeFlag,
			govUnitPriceFlag,
			rewardMintAmountFlag,
			rewardRatioFlag,
			rewardGiniCoeffFlag,
			rewardDeferredTxFeeFlag,
			istEpochFlag,
			istProposerPolicyFlag,
			istSubGroupFlag,
			cliqueEpochFlag,
			cliquePeriodFlag,
		},
		ArgsUsage: "type",
	}
)

const (
	DefaultTcpPort uint16 = 32323
	TypeNotDefined        = -1
	TypeDocker            = 0
	TypeLocal             = 1
	TypeRemote            = 2
	TypeDeploy            = 3
	DirScript             = "scripts"
	DirKeys               = "keys"
	CNIpNetwork           = "10.11.2"
	PNIpNetwork1          = "10.11.10"
	PNIpNetwork2          = "10.11.11"
)

var Types = [4]string{"docker", "local", "remote", "deploy"}

var GrafanaFiles = [...]GrafanaFile{
	{
		url:  "https://s3.ap-northeast-2.amazonaws.com/klaytn-tools/Klaytn.json",
		name: "Klaytn.json",
	},
	{
		url:  "https://s3.ap-northeast-2.amazonaws.com/klaytn-tools/klaytn_txpool.json",
		name: "Klaytn_txpool.json",
	},
}

var lastIssuedPortNum = DefaultTcpPort

func genRewardConfig(ctx *cli.Context) *params.RewardConfig {
	mintingAmount := ctx.Int64(rewardMintAmountFlag.Name)
	ratio := ctx.String(rewardRatioFlag.Name)
	giniCoeff := ctx.Bool(rewardGiniCoeffFlag.Name)
	deferredTxFee := ctx.Bool(rewardDeferredTxFeeFlag.Name)

	return &params.RewardConfig{
		MintingAmount: big.NewInt(mintingAmount),
		Ratio:         ratio,
		UseGiniCoeff:  giniCoeff,
		DeferredTxFee: deferredTxFee,
	}
}

func genIstanbulConfig(ctx *cli.Context) *params.IstanbulConfig {
	epoch := ctx.Uint64(istEpochFlag.Name)
	policy := ctx.Uint64(istProposerPolicyFlag.Name)
	subGroup := ctx.Uint64(istSubGroupFlag.Name)

	return &params.IstanbulConfig{
		Epoch:          epoch,
		ProposerPolicy: policy,
		SubGroupSize:   subGroup,
	}
}

func genGovernanceConfig(ctx *cli.Context) *params.GovernanceConfig {
	govMode := ctx.String(govModeFlag.Name)
	governingNode := ctx.String(governingNodeFlag.Name)
	if !common.IsHexAddress(governingNode) {
		utils.Fatalf("Governing Node is invalid hex address", "value", governingNode)
	}
	govUnitPrice := ctx.Uint64(govUnitPriceFlag.Name)

	return &params.GovernanceConfig{
		GoverningNode:  common.HexToAddress(governingNode),
		GovernanceMode: govMode,
		Reward:         genRewardConfig(ctx),
		Istanbul:       genIstanbulConfig(ctx),
		UnitPrice:      govUnitPrice,
	}
}

func genCliqueConfig(ctx *cli.Context) *params.CliqueConfig {
	epoch := ctx.Uint64(cliqueEpochFlag.Name)
	period := ctx.Uint64(cliquePeriodFlag.Name)

	return &params.CliqueConfig{
		Period: period,
		Epoch:  epoch,
	}
}

func RandStringRunes(n int) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789~!@#$%^&*()_+{}|[]")

	rand.Seed(time.Now().UnixNano())

	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func gen(ctx *cli.Context) error {
	genType := findGenType(ctx)

	cliqueFlag := ctx.Bool(cliqueFlag.Name)
	num := ctx.Int(numOfCNsFlag.Name)
	proxyNum := ctx.Int(numOfPNsFlag.Name)
	unitPrice := ctx.Uint64(unitPriceFlag.Name)
	deriveShaImpl := ctx.Int(deriveShaImplFlag.Name)
	stakingInterval := ctx.Uint64(stakingFlag.Name)
	proposerInterval := ctx.Uint64(proposerFlag.Name)

	privKeys, nodeKeys, nodeAddrs := istcommon.GenerateKeys(num)

	var genesisJsonBytes []byte
	if cliqueFlag {
		config := genCliqueConfig(ctx)
		// Clique
		genesisJsonBytes, _ = json.MarshalIndent(genesis.NewClique(
			genesis.ValidatorsOfClique(nodeAddrs...),
			genesis.Alloc(nodeAddrs, new(big.Int).Exp(big.NewInt(10), big.NewInt(50), nil)),
			genesis.UnitPrice(unitPrice),
			genesis.Clique(config),
			genesis.StakingInterval(stakingInterval),
			genesis.ProposerInterval(proposerInterval),
		), "", "    ")

		path := path.Join(outputPath, DirKeys)
		ks := keystore.NewKeyStore(path, keystore.StandardScryptN, keystore.StandardScryptP)

		for i, pk := range privKeys {
			pwdStr := RandStringRunes(100)
			ks.ImportECDSA(pk, pwdStr)
			writeFile([]byte(pwdStr), DirKeys, "passwd"+strconv.Itoa(i+1))
		}
	} else {
		config := genGovernanceConfig(ctx)
		// Istanbul BFT
		if len(nodeAddrs) > 0 && config.GoverningNode.String() == params.DefaultGoverningNode {
			config.GoverningNode = nodeAddrs[0]
		}
		genesisJsonBytes, _ = json.MarshalIndent(genesis.New(
			genesis.Validators(nodeAddrs...),
			genesis.Alloc(nodeAddrs, new(big.Int).Exp(big.NewInt(10), big.NewInt(50), nil)),
			genesis.UnitPrice(unitPrice),
			genesis.DeriveShaImpl(deriveShaImpl),
			genesis.Governance(config),
			genesis.StakingInterval(stakingInterval),
			genesis.ProposerInterval(proposerInterval),
		), "", "    ")
	}

	switch genType {
	case TypeDocker:
		validators := makeValidators(num, false, nodeAddrs, nodeKeys, privKeys)
		_, proxyNodeKeys := makeProxys(proxyNum, false)
		nodeInfos := filterNodeInfo(validators)
		staticNodesJsonBytes, _ := json.MarshalIndent(nodeInfos, "", "\t")
		compose := compose.New(
			"172.16.239",
			num,
			"bb98a0b6442386d0cdf8a31b267892c1",
			nodeKeys,
			removeSpacesAndLines(genesisJsonBytes),
			removeSpacesAndLines(staticNodesJsonBytes),
			ctx.String(dockerImageIdFlag.Name),
			ctx.Bool(fasthttpFlag.Name),
			ctx.Int(networkIdFlag.Name),
			!ctx.BoolT(nografanaFlag.Name),
			proxyNodeKeys,
			ctx.Bool(useTxGenFlag.Name),
			service.TxGenOption{
				TxGenRate:       ctx.Int(txGenRateFlag.Name),
				TxGenThreadSize: ctx.Int(txGenThFlag.Name),
				TxGenConnSize:   ctx.Int(txGenConnFlag.Name),
				TxGenDuration:   ctx.String(txGenDurFlag.Name),
			})
		os.MkdirAll(outputPath, os.ModePerm)
		ioutil.WriteFile(path.Join(outputPath, "docker-compose.yml"), []byte(compose.String()), os.ModePerm)
		fmt.Println("Created : ", path.Join(outputPath, "docker-compose.yml"))
		ioutil.WriteFile(path.Join(outputPath, "prometheus.yml"), []byte(compose.PrometheusService.Config.String()), os.ModePerm)
		fmt.Println("Created : ", path.Join(outputPath, "prometheus.yml"))
		downLoadGrafanaJson()
	case TypeLocal:
		writeNodeFiles(true, num, nodeAddrs, nodeKeys, privKeys, genesisJsonBytes)
		downLoadGrafanaJson()
	case TypeRemote:
		writeNodeFiles(false, num, nodeAddrs, nodeKeys, privKeys, genesisJsonBytes)
		downLoadGrafanaJson()
	case TypeDeploy:
		writeCNInfoKey(num, nodeAddrs, nodeKeys, privKeys, genesisJsonBytes)
		writeKlayConfig(ctx.Int(networkIdFlag.Name), ctx.Int(rpcPortFlag.Name), ctx.Int(wsPortFlag.Name), ctx.Int(p2pPortFlag.Name),
			ctx.String(dataDirFlag.Name), ctx.String(logDirFlag.Name), "CN")
		writeKlayConfig(ctx.Int(networkIdFlag.Name), ctx.Int(rpcPortFlag.Name), ctx.Int(wsPortFlag.Name), ctx.Int(p2pPortFlag.Name),
			ctx.String(dataDirFlag.Name), ctx.String(logDirFlag.Name), "PN")
		writePNInfoKey(ctx.Int(numOfPNsFlag.Name))
		writePrometheusConfig(num, ctx.Int(numOfPNsFlag.Name))
	}

	return nil
}

func downLoadGrafanaJson() {
	for _, file := range GrafanaFiles {
		resp, err := http.Get(file.url)
		if err != nil || resp.StatusCode != 200 {
			fmt.Printf("Failed to download the imgs dashboard file(%s) [%s] - %v\n", file.url,
				resp.Status, err)
		} else {
			bytes, e := ioutil.ReadAll(resp.Body)
			if e != nil {
				fmt.Println("Failed to read http response", e)
			} else {
				fileName := file.name
				ioutil.WriteFile(path.Join(outputPath, fileName), bytes, os.ModePerm)
				fmt.Println("Created : ", path.Join(outputPath, fileName))
			}
			resp.Body.Close()
		}
	}
}

func writeCNInfoKey(num int, nodeAddrs []common.Address, nodeKeys []string, privKeys []*ecdsa.PrivateKey,
	genesisJsonBytes []byte) {
	const DirCommon = "common"
	writeFile(genesisJsonBytes, DirCommon, "genesis.json")

	validators := makeValidatorsWithIp(num, false, nodeAddrs, nodeKeys, privKeys, []string{CNIpNetwork})
	staticNodesJsonBytes, _ := json.MarshalIndent(filterNodeInfo(validators), "", "\t")
	writeFile(staticNodesJsonBytes, DirCommon, "static-nodes.json")

	for i, v := range validators {
		parentDir := fmt.Sprintf("cn%02d", i+1)
		writeFile([]byte(nodeKeys[i]), parentDir, "nodekey")
		str, _ := json.MarshalIndent(v, "", "\t")
		writeFile([]byte(str), parentDir, "validator")
	}
}

func writePNInfoKey(num int) {
	privKeys, nodeKeys, nodeAddrs := istcommon.GenerateKeys(num)
	validators := makeValidatorsWithIp(num, false, nodeAddrs, nodeKeys, privKeys, []string{PNIpNetwork1, PNIpNetwork2})
	for i, v := range validators {
		parentDir := fmt.Sprintf("pn%02d", i+1)
		writeFile([]byte(nodeKeys[i]), parentDir, "nodekey")
		str, _ := json.MarshalIndent(v, "", "\t")
		writeFile([]byte(str), parentDir, "validator")
	}
}

func writeKlayConfig(networkId int, rpcPort int, wsPort int, p2pPort int, dataDir string, logDir string, nodeType string) {
	kConfig := KlaytnConfig{
		networkId,
		rpcPort,
		wsPort,
		p2pPort,
		dataDir,
		logDir,
		"/var/run/klay",
		nodeType,
	}
	writeFile([]byte(kConfig.String()), strings.ToLower(nodeType), "klay.conf")
}

func writePrometheusConfig(cnNum int, pnNum int) {
	pConf := NewPrometheusConfig(cnNum, CNIpNetwork, pnNum, PNIpNetwork1, PNIpNetwork2)
	writeFile([]byte(pConf.String()), "monitoring", "prometheus.yml")
}

func writeNodeFiles(isWorkOnSingleHost bool, num int, nodeAddrs []common.Address, nodeKeys []string,
	privKeys []*ecdsa.PrivateKey, genesisJsonBytes []byte) {
	validators := makeValidators(num, isWorkOnSingleHost, nodeAddrs, nodeKeys, privKeys)
	nodeInfos := filterNodeInfo(validators)
	staticNodesJsonBytes, _ := json.MarshalIndent(nodeInfos, "", "\t")
	writeFile(genesisJsonBytes, DirScript, "genesis.json")
	writeValidatorsAndNodesToFile(validators, DirKeys, nodeKeys)
	writeFile(staticNodesJsonBytes, DirScript, "static-nodes.json")
}

func filterNodeInfo(validatorInfos []*ValidatorInfo) []string {
	var nodes []string
	for _, v := range validatorInfos {
		nodes = append(nodes, string(v.NodeInfo))
	}
	return nodes
}

func makeValidators(num int, isWorkOnSingleHost bool, nodeAddrs []common.Address, nodeKeys []string,
	keys []*ecdsa.PrivateKey) []*ValidatorInfo {
	var validatorPort uint16
	var validators []*ValidatorInfo
	for i := 0; i < num; i++ {
		if isWorkOnSingleHost {
			validatorPort = lastIssuedPortNum
			lastIssuedPortNum++
		} else {
			validatorPort = DefaultTcpPort
		}

		v := &ValidatorInfo{
			Address: nodeAddrs[i],
			Nodekey: nodeKeys[i],
			NodeInfo: discover.NewNode(
				discover.PubkeyID(&keys[i].PublicKey),
				net.ParseIP("0.0.0.0"),
				0,
				validatorPort).String(),
		}
		validators = append(validators, v)
	}
	return validators
}

func makeValidatorsWithIp(num int, isWorkOnSingleHost bool, nodeAddrs []common.Address, nodeKeys []string,
	keys []*ecdsa.PrivateKey, networkIds []string) []*ValidatorInfo {
	var validatorPort uint16
	var validators []*ValidatorInfo
	for i := 0; i < num; i++ {
		if isWorkOnSingleHost {
			validatorPort = lastIssuedPortNum
			lastIssuedPortNum++
		} else {
			validatorPort = DefaultTcpPort
		}

		nn := len(networkIds)
		idx := (i + 1) % nn
		if nn > 1 {
			if idx == 0 {
				idx = nn - 1
			} else { // idx > 0
				idx = idx - 1
			}
		}
		v := &ValidatorInfo{
			Address: nodeAddrs[i],
			Nodekey: nodeKeys[i],
			NodeInfo: discover.NewNode(
				discover.PubkeyID(&keys[i].PublicKey),
				net.ParseIP(fmt.Sprintf("%s.%d", networkIds[idx], 100+(i/nn)+1)),
				0,
				validatorPort).String(),
		}
		validators = append(validators, v)
	}
	return validators
}

func makeProxys(num int, isWorkOnSingleHost bool) ([]*ValidatorInfo, []string) {
	privKeys, nodeKeys, nodeAddrs := istcommon.GenerateKeys(num)

	var p2pPort uint16
	var proxies []*ValidatorInfo
	var proxyNodeKeys []string
	for i := 0; i < num; i++ {
		if isWorkOnSingleHost {
			p2pPort = lastIssuedPortNum
			lastIssuedPortNum++
		} else {
			p2pPort = DefaultTcpPort
		}

		v := &ValidatorInfo{
			Address: nodeAddrs[i],
			Nodekey: nodeKeys[i],
			NodeInfo: discover.NewNode(
				discover.PubkeyID(&privKeys[i].PublicKey),
				net.ParseIP("0.0.0.0"),
				0,
				p2pPort).String(),
		}
		proxies = append(proxies, v)
		proxyNodeKeys = append(proxyNodeKeys, v.Nodekey)
	}
	return proxies, proxyNodeKeys
}

func writeValidatorsAndNodesToFile(validators []*ValidatorInfo, parentDir string, nodekeys []string) {
	for i, v := range validators {
		parentPath := path.Join(outputPath, parentDir)
		os.MkdirAll(parentPath, os.ModePerm)

		nodeKeyFilePath := path.Join(parentPath, "nodekey"+strconv.Itoa(i+1))
		ioutil.WriteFile(nodeKeyFilePath, []byte(nodekeys[i]), os.ModePerm)
		fmt.Println("Created : ", nodeKeyFilePath)

		str, _ := json.MarshalIndent(v, "", "\t")
		validatorInfoFilePath := path.Join(parentPath, "validator"+strconv.Itoa(i+1))
		ioutil.WriteFile(validatorInfoFilePath, []byte(str), os.ModePerm)
		fmt.Println("Created : ", validatorInfoFilePath)
	}
}

func writeFile(content []byte, parentFolder string, fileName string) {
	filePath := path.Join(outputPath, parentFolder, fileName)
	os.MkdirAll(path.Dir(filePath), os.ModePerm)
	ioutil.WriteFile(filePath, content, os.ModePerm)
	fmt.Println("Created : ", filePath)
}

func findGenType(ctx *cli.Context) int {
	var genType = TypeNotDefined
	if len(ctx.Args()) >= 1 {
		for i, t := range Types {
			if t == ctx.Args()[0] {
				genType = i
				break
			}
		}
		if genType == TypeNotDefined {
			fmt.Printf("Wrong Type : %s\nSupported Types : [docker, local, remote, deploy]\n\n", ctx.Args()[0])
			cli.ShowSubcommandHelp(ctx)
			os.Exit(1)
		}
	} else {
		genType = TypeDocker
	}
	return genType
}

func removeSpacesAndLines(b []byte) string {
	out := string(b)
	out = strings.Replace(out, " ", "", -1)
	out = strings.Replace(out, "\t", "", -1)
	out = strings.Replace(out, "\n", "", -1)
	return out
}
