package main

import (
	"cr/migrator"
	"log"
	"net/http"
	"net/rpc"

)

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Ltime | log.Lmicroseconds)
}

func main() {

	// launch a rpc server, serves on port 1234
	m := new(migrator.Migrator)
	rpc.Register(m)
	rpc.HandleHTTP()

	err := http.ListenAndServe(migrator.Port, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}

}
