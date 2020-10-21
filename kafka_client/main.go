package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/big"
	"reflect"
	"time"

	"github.com/klaytn/klaytn/datasync/chaindatafetcher/kafka"
	"github.com/klaytn/klaytn/node/cn"

	"github.com/Shopify/sarama"
	"github.com/klaytn/klaytn/networks/rpc"
)

//var enEndpoint = "http://localhost:8551"
var kafkaBrokers = []string{
	//"kafka:9094",
	"b-1.jk-test.axcstw.c3.kafka.ap-northeast-2.amazonaws.com:9092",
	"b-2.jk-test.axcstw.c3.kafka.ap-northeast-2.amazonaws.com:9092",
}

func isSameBlock(cli *rpc.Client, blockgroup map[string]interface{}) bool {
	numberRaw := int64(blockgroup["blockNumber"].(float64))
	blockResult := blockgroup["result"].(map[string]interface{})

	number := "0x" + big.NewInt(numberRaw).Text(16)
	var result map[string]interface{}
	err := cli.Call(&result, "klay_getBlockWithConsensusInfoByNumber", number)
	if err != nil {
		log.Fatal("get block is failed", "blockNumber", number, "err", err)
	}

	for key, val := range blockResult {
		if reflect.TypeOf(val) != reflect.TypeOf(result[key]) && val != result[key] {
			log.Fatal("blocks are not same", "key", key, "val", val, "val2", result[key], reflect.TypeOf(val), reflect.TypeOf(result[key]))
		}
	}
	log.Println("blocks are the same", "number", number)
	return true
}

func isSameTraceResult(a, b map[string]interface{}) bool {
	for key, val := range a {
		if key == "time" {
			continue
		}
		if key == "calls" {
			va, aExist := a[key]
			vb, bExist := b[key]
			if aExist != bExist {
				log.Fatal("something has missing internal calls", aExist, bExist)
			}

			vaSlice := va.([]interface{})
			vbSlice := vb.([]interface{})

			for idx, vaSliceElement := range vaSlice {
				if !isSameTraceResult(vaSliceElement.(map[string]interface{}), vbSlice[idx].(map[string]interface{})) {
					log.Fatal("internal calls are different")
				}
			}
		}

		if reflect.TypeOf(val) != reflect.TypeOf(b[key]) && val != b[key] {
			log.Fatal("different", " key ", key, " val ", val, " b[key] ", b[key], reflect.TypeOf(val), reflect.TypeOf(b[key]))
		}
	}
	return true
}

func isSameTrace(cli *rpc.Client, tracegroup map[string]interface{}) bool {
	numberRaw := int(tracegroup["blockNumber"].(float64))
	traceResult := tracegroup["result"].([]interface{})
	//number := strconv.Itoa(numberRaw)

	number := fmt.Sprintf("0x%x", int64(numberRaw))
	var result []map[string]interface{}

	fastCallTracer := "fastCallTracer"
	timeout := "1h"
	err := cli.Call(&result, "debug_traceBlockByNumber", number, cn.TraceConfig{
		Tracer:  &fastCallTracer,
		Timeout: &timeout,
	})
	if err != nil {
		log.Fatal("get trace is failed", "blockNumber", number, "err", err)
	}

	for idx, val := range result {
		r1 := val["result"].(map[string]interface{})
		r2 := traceResult[idx].(map[string]interface{})

		if !isSameTraceResult(r1, r2) {
			log.Fatal("traces are not same ", " number ", numberRaw)
		}
	}

	log.Println("traces are the same", "number", number)
	return true
}

func main() {
	consumerGroupId := flag.String("groupid", "", "consumergroupId")
	resourceName := flag.String("resource", "", "resource name")
	enEndpoint := flag.String("endpoint", "", "en endpoint (http://10.63.239.13:8551)")

	// map for comparing data
	flag.Parse()

	if *enEndpoint == "" {
		panic("no en endpoint")
	}
	if *consumerGroupId == "" {
		panic("enter consumer group id")
	}

	if *resourceName == "" {
		panic("enter resource name")
	}

	config := kafka.GetDefaultKafkaConfig()
	config.Brokers = kafkaBrokers
	config.TopicEnvironmentName = "segment-test"
	config.TopicResourceName = *resourceName
	config.SaramaConfig.Consumer.Offsets.Initial = sarama.OffsetOldest

	groupId := *consumerGroupId
	consumer, err := kafka.NewConsumer(config, groupId)
	if err != nil {
		panic(err)
	}
	defer consumer.Close()
	log.Println("created a new consumer")
	log.Println("blockgroup topic", config.GetTopicName(kafka.EventBlockGroup))
	log.Println("tracegroup topic", config.GetTopicName(kafka.EventTraceGroup))
	//
	//cli, err := rpc.Dial(enEndpoint)
	//if err != nil {
	//	panic(err)
	//}
	//var blockNumber uint64 = 0
	//var blockNumberMutex sync.RWMutex
	//err = consumer.AddTopicAndHandler(kafka.EventTraceGroup, func(message *sarama.ConsumerMessage) error {
	//	blockNumberMutex.Lock()
	//	defer blockNumberMutex.Unlock()
	//	var result map[string]interface{}
	//	json.Unmarshal(message.Value, &result)
	//	number := uint64(result["blockNumber"].(float64))
	//	for blockNumber < number {
	//		var count string
	//		err := cli.Call(&count, "klay_getBlockTransactionCountByNumber", "0x"+new(big.Int).SetUint64(blockNumber).Text(16))
	//		if err != nil {
	//			log.Println(err.Error())
	//			time.Sleep(1 * time.Second)
	//			continue
	//		}
	//
	//		if count != "0x0" {
	//			log.Fatal("there is some transactions", "count: ", count, "blockNumber", blockNumber)
	//		}
	//
	//		log.Println(blockNumber, number)
	//		blockNumber++
	//	}
	//
	//	if blockNumber == number {
	//		log.Println(blockNumber, number)
	//		blockNumber++
	//	}
	//
	//	return nil
	//})
	//if err != nil {
	//	panic(err)
	//}
	//log.Println("added tracegroup handler")

	cli, err := rpc.Dial(*enEndpoint)
	if err != nil {
		panic(err)
	}
	err = consumer.AddTopicAndHandler(kafka.EventBlockGroup, func(message *sarama.ConsumerMessage) error {
		var result map[string]interface{}
		err := json.Unmarshal(message.Value, &result)
		if err != nil {
			return err
		}
		if !isSameBlock(cli, result) {
			return fmt.Errorf("the blocks are not same. blockNumber: %v", result["number"].(string))
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	log.Println("added blockgroup handler")

	err = consumer.AddTopicAndHandler(kafka.EventTraceGroup, func(message *sarama.ConsumerMessage) error {
		var result map[string]interface{}
		err := json.Unmarshal(message.Value, &result)
		if err != nil {
			panic(err)
		}
		if !isSameTrace(cli, result) {
			return fmt.Errorf("the traces are not same. blockNumber: %v", result["number"].(string))
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	log.Println("added tracegroup handler")

	for {
		err = consumer.Subscribe(context.Background())
		if err == sarama.ErrClosedClient {
			fmt.Println("closed client")
			return
		} else {
			log.Println(err)
			time.Sleep(1 * time.Second)
		}
	}
}
