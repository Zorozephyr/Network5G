package lab0

import (
	"fmt"
	"io"
	"net"
)

type listenerInterface func(string, int, handlerInterface)

type handlerInterface func(conn net.Conn)

func TCPListener(host string, port int, handler handlerInterface) {
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		fmt.Println("The error is ", err)
		return
	}
	fmt.Printf("Listening in port %s:%d\n", host, port)
	defer ln.Close()

	for {
		conn, err := ln.Accept()

		if err != nil {
			fmt.Println("Error while accepting connection\n", err)
			continue
		}
		fmt.Println("Accepted Connection")

		go handler(conn)
	}
}

func TCPHandler(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			// Check for normal client disconnects to avoid spamming the logs
			if err == io.EOF {
				fmt.Println("Client disconnected")
			} else {
				fmt.Println("Read error:", err)
			}
			return // CRITICAL: Exit the goroutine to prevent infinite loops
		}

		fmt.Print(string(buf[:n]))
		_, err = conn.Write(buf[:n])
		if err != nil {
			fmt.Println("Write error:", err)
			return // CRITICAL: Exit the goroutine
		}
	}

}
