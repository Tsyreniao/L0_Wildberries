package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"time"

	"l0_tsybikov/structs"

	"github.com/nats-io/nats.go"
)

var (
	byteData []byte
	rg       = rand.New(rand.NewSource(time.Now().Unix()))
	order    = new(structs.Order)
	err      error
)

func readJSON() {
	jsonFile, err := os.Open("../model.json")
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println("Successfully Opened model.json")
	}
	defer jsonFile.Close()

	byteData, _ = io.ReadAll(jsonFile)
	if err := json.Unmarshal(byteData, order); err != nil {
		panic(err)
	}
}

func createData() {
	for i := 1; i < len(order.Items); i++ {
		order.Items = append(order.Items[:0], order.Items[0+1:]...)
	}

	randNum := rg.Intn(10000000)
	randString := string(fmt.Sprintf("String%vString", randNum))
	order.UUID = randString
	order.Payment.Transaction = randString

	item := order.Items[0]
	item.ChrtId = randNum
	n := rg.Intn(5)
	order.Items = order.Items[:1]
	for i := 0; i < n; i++ {
		order.Items = append(order.Items, item)
	}

}

func orderToByte() {
	byteData, err = json.Marshal(order)
	if err != nil {
		panic(err)
	}
}

func main() {
	url := "nats://localhost:4222"
	nc, err := nats.Connect(url)
	if err != nil {
		panic(err)
	}
	defer nc.Close()

	readJSON()

	for {
		//Заполненеи некоторых полей рандомными данными и добавление item
		createData()
		orderToByte()
		nc.Publish("subject", byteData)

		time.Sleep(5 * time.Second)
	}
}
