package rpc

import (
	"cr/migrator"
	"log"
	"net/http"
	"net/rpc"
	"os"
	"sync"
)

func LaunchServer(port string, wg *sync.WaitGroup) {
	// launch a rpc server, serves on port 1234
	m := new(migrator.Migrator)
	if len(os.Args) > 2 && os.Args[1] == "--no-shared-fs" {
		m.IsSharedFS = false
	} else {
		m.IsSharedFS = true
	}
	err := rpc.Register(m)
	if err != nil {
		log.Fatal("Register error: ", err)
	}
	rpc.HandleHTTP()

	err = http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
	wg.Done()
}
