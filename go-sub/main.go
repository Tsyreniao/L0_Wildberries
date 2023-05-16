package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"text/template"
	"time"

	"l0_tsybikov/structs"

	_ "github.com/lib/pq"
	"github.com/nats-io/nats.go"
)

const (
	portNumber = ":8080"

	natsUrl = "nats://localhost:4222"

	host     = "localhost"
	port     = 5432
	user     = "postgres"
	password = "1234"
	dbname   = "postgres"
)

type Answer struct {
	IsFirstTime bool
	IsExist     bool
	UID         int
	Order       structs.Order
}

var Orders map[int]string = map[int]string{}
var order structs.Order
var ans Answer
var err error
var db *sql.DB

func indexHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("../templates/index.html", "../templates/header.html")
	if err != nil {
		panic(err)
	}
	tmpl.ExecuteTemplate(w, "index", ans)
}

func orderHandler(w http.ResponseWriter, r *http.Request) {
	var order string
	var ok bool
	ans.UID, err = strconv.Atoi(r.FormValue("orderID"))
	if err != nil {
		panic(err)
	}

	if order, ok = findOrderInCache(ans.UID); ok {
		fmt.Println("ORDER FOUND FROM CACHE WITH UID = ", ans.UID)
		ans.Order = orderStringToStruct(order)
	} else if order, ok = findOrderInDB(ans.UID); ok {
		fmt.Println("ORDER FOUND FROM DB WITH UID = ", ans.UID)
		ans.Order = orderStringToStruct(order)
	} else {
		fmt.Println("NO ORDER FIND WITH ID = ", ans.UID)
	}

	ans.IsFirstTime = false
	ans.IsExist = ok
	http.Redirect(w, r, "/", http.StatusFound)
}

func findOrderInCache(uid int) (string, bool) {
	for k, v := range Orders {
		if k == uid {
			return v, true
		}
	}
	return "", false
}

func findOrderInDB(uid int) (string, bool) {
	var data string
	query := `SELECT data FROM orders WHERE uid = ($1) LIMIT 1`
	err = db.QueryRow(query, ans.UID).Scan(&data)
	if err != nil {
		fmt.Println("ERROR: ", err)
		return "", false
	}
	return data, true
}

func orderStringToStruct(s string) structs.Order {
	byteData := []byte(s)
	if err := json.Unmarshal(byteData, &ans.Order); err != nil {
		panic(err)
	}
	return ans.Order
}

func recoverCacheFormDB() {
	var uid int
	var data string
	//Для выбора последних 10 добавленных строк
	//"SELECT * FROM orders ORDER BY uid DESC LIMIT 10"
	rows, err := db.Query("SELECT * FROM orders") //Сохранение свей БД
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&uid, &data)
		if err != nil {
			panic(err)
		}

		if _, ok := Orders[uid]; !ok {
			Orders[uid] = data
		}
	}
	fmt.Println("Cache was recovered from DB!")
}

func server() {
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/order", orderHandler)
	fmt.Printf("Starting application on port %v!\n", portNumber)
	http.ListenAndServe(portNumber, nil)
}

func main() {
	//Подключение к postgresql DB
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)
	db, err = sql.Open("postgres", psqlInfo)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	fmt.Println("Successfully connected to postgres!")

	//Востановленеи Кеша из БД
	recoverCacheFormDB()

	//Подключение к nats server
	nc, err := nats.Connect(natsUrl)
	if err != nil {
		panic(err)
	}
	defer nc.Close()
	fmt.Println("Successfully connected to nats server!")

	//Подписка на subject
	sub, err := nc.SubscribeSync("subject")
	if err != nil {
		panic(err)
	}
	defer sub.Unsubscribe()

	//http server
	ans.IsFirstTime = true
	go server()

	//Получение сообщений
	fmt.Println("Waiting for massages!")
	for {
		if msg, _ := sub.NextMsg(120 * time.Second); msg != nil {
			//Десериализация и сериализация
			//Должны избавить от проблемы с рандомными получеными даннами
			json.Unmarshal(msg.Data, &order)
			byteOrder, err := json.Marshal(order)
			if err != nil {
				panic(err)
			}

			query := `INSERT INTO orders (data) VALUES ($1) RETURNING uid`
			var uid int
			db.QueryRow(query, byteOrder).Scan(&uid) //Добавленеи данных в БД
			Orders[uid] = string(byteOrder)          //Добавленеи данных в Кеш
			fmt.Printf("Order added to DB and Cache with:\n UID: %v\n Data: %v\n", uid, string(byteOrder))
		} else {
			fmt.Println("ERROR: ", err)
			break
		}
	}
}
