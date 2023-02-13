package main

import (
	"cr/client/cmd"
	"log"
)

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Ltime | log.Lmicroseconds)

}

func main() {
	cmd.Execute()
}
