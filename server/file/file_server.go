package file

import (
	"cr/util"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os/exec"
	"path/filepath"
)

func LaunchFileReceiveServer(port string) {
	// launch a file receive server
	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("Listen error: %v", err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatalf("Accept error: %v", err)
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	// 1. read 4 bytes to integer from conn as the length of the file name
	var fileNameLength int32
	err := binary.Read(conn, binary.LittleEndian, &fileNameLength)
	if err != nil {
		log.Printf("failed to read file name length: %v", err)
		return
	}
	// 2. read the file name
	// TODO: decode the file name
	filePathBuf := make([]byte, fileNameLength)
	n, err := io.ReadFull(conn, filePathBuf)
	if err != nil {
		log.Printf("failed to read file name: %v", err)
		return
	}
	if n != int(fileNameLength) {
		log.Printf("failed to read file name, read %d chars, excepted %d", n, fileNameLength)
		return
	}

	filePath := string(filePathBuf)

	// 3. read the file content
	err = util.ReceiveFile(conn, filePath)
	log.Printf("received file %s", filePath)
	if err != nil {
		log.Printf("failed to receive file: %v", err)
		return
	}
	// 4. unzip tarball
	fileDir := filepath.Dir(filePath)
	fileName := filePath[len(fileDir)+1:]
	cmd := exec.Command("tar", "-zvxf", fileName)
	cmd.Dir = fileDir
	fmt.Printf("untar file %s to %s", filePath, fileDir)
	err = cmd.Run()
	if err != nil {
		log.Printf("failed to untar file: %v", err)
		return
	}
	// 5. TODO: delete tarball
}
