package main

import (
	"cr/migrator"
	"cr/server/file"
	"cr/server/rpc"
	"log"
)

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Ltime | log.Lmicroseconds)
}

func main() {
	// launch a rpc server
	rpc.LaunchServer(migrator.RPCPort)

	// launch a file receive server
	file.LaunchFileReceiveServer(migrator.FilePort)
}
