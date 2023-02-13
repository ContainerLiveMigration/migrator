package main

import (
	"log"
	"cr/client/cmd"
)

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Ltime | log.Lmicroseconds)

}

func main() {
	cmd.Execute()
}
