package main

import (
	"cr/migrator"
	"cr/server/file"
	"cr/server/rpc"
	"log"
	"sync"
)

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Ltime | log.Lmicroseconds)
}

func main() {
	var wg sync.WaitGroup
	wg.Add(2)
	// launch a rpc server
	go rpc.LaunchServer(migrator.RPCPort, &wg)
	log.Printf("rpc server launched on port %s", migrator.RPCPort)
	// launch a file receive server
	go file.LaunchFileReceiveServer(migrator.FilePort, &wg)
	log.Printf("file receive server launched on port %s", migrator.FilePort)

	wg.Wait()
}
