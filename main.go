package main

import (
	"context"
	"github.com/joho/godotenv"
	"net/http"
	"ws-sfu-server/pkg/connections"
	"ws-sfu-server/pkg/db"
	"ws-sfu-server/pkg/sfu"
)

var database *db.Db

type emailStruct struct {
	emailId string
}

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		panic("Error loading .env file" + err.Error())
		return
	}
	ctx := context.Background()
	database = &db.Db{}
	database.InitDbConnection(ctx)
	http.HandleFunc("/ws", connections.HandleInitConnection)
	http.HandleFunc("/sfu", sfu.HandleInitConnection)
	http.HandleFunc("/sfu-many", sfu.HandleInitConnection)
	err = http.ListenAndServe(":8000", nil)
	if err != nil {
		panic(err.Error())
		return
	}
	//if err != nil {
	//	fmt.Println("Error connecting to database" + err.Error())
	//}
	defer database.Close()
	//database.ExecSQL(`
	//	SELECT email from learners;
	//`)
	//rows := database.Query(`SELECT email from learners;`)
	//var emails emailStruct
	//////var email string
	//rows.Next()
	//rows.Scan(&emails.emailId)
	//rows.Close()
	//learner := database.FetchLearner("johndoe")
	//learner2 := database.FetchLearner("johndoe1")
	//fmt.Println(learner.Email, learner2.Email)
	//for rows.Next() {
	//	err := rows.Scan(&email)
	//	if err != nil {
	//		panic(err.Error())
	//		return
	//	}
	//	emails = append(emails, email)
	//}
	//fmt.Println((emails.emailId))

}
