package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

// only needed below for sample processing

func main() {

	fmt.Println("Launching tcp server")

	// listen on all interfaces
	ln, _ := net.Listen("tcp", "0.0.0.0:9593")

	for {

		// accept connection on port
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println(err)
		}

		// will listen for message to process ending in newline (\n)
		message, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			fmt.Println(err)
		}

		temp := strings.TrimSpace(string(message))
		if temp == "STOP" {
			break
		}

		// output message received
		fmt.Print("Message Received:", string(message))
		// sample process for string received

		newmessage := strings.ToUpper(message)

		// send new string back to client
		conn.Write([]byte(newmessage + "\n"))

		conn.Close()
	}
}
