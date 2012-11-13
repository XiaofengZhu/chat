package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type packet struct {
	Pname       string
	Pconnection net.Conn
	Ptime       time.Time
	Pmessage    string
	Ptype       int
}

const TYPE_LOGIN int = 0
const TYPE_MESSAGE int = 1
const TYPE_LOGOUT int = 2

func handlemessage(message string, connections map[string]packet) {
	for _, user := range connections {
		output := bufio.NewWriter(user.Pconnection)
		output.WriteString(message)
		output.Flush()
	}
}

func handleconnection(c net.Conn, packetchan chan packet) {
	var user packet
	input := bufio.NewReader(c)
	output := bufio.NewWriter(c)
	output.WriteString("What is your name? ")
	output.Flush()
	name, _ := input.ReadString('\n')

	user.Pname = strings.TrimSpace(name)
	user.Pconnection = c
	user.Ptime = time.Now()
	user.Ptype = TYPE_LOGIN
	packetchan <- user

	for {
		line, err := input.ReadString('\n')
		if err != nil {
			break
		} else if len(line) > 2 {
			user.Pmessage = user.Pname + "> " + line
			user.Ptype = TYPE_MESSAGE
			user.Ptime = time.Now()
			packetchan <- user
		}
	}

	user.Ptype = TYPE_LOGOUT
	user.Ptime = time.Now()
	packetchan <- user

	user.Pconnection.Close()
}

func handlepacket(packetchan chan packet) {
	connections := make(map[string]packet)
	for {
		packetrec := <- packetchan
		switch packetrec.Ptype {
		case TYPE_LOGIN:
			connections[packetrec.Pname] = packetrec

		case TYPE_MESSAGE:
			go handlemessage(packetrec.Pmessage, connections)

		case TYPE_LOGOUT:
			delete(connections, packetrec.Pname)

		default:
			// nothing
		}
	}
}

func accept(packetchan chan packet, listen net.Listener) {
	for {
		connection, err := listen.Accept()
		if err != nil {
			fmt.Println(err)
			fmt.Println("Exiting...")
			os.Exit(2)
		}

		go handleconnection(connection, packetchan)
	}
}

func start(packetchan chan packet) {
	listen, err := net.Listen("tcp", ":2000")
	if err != nil {
		fmt.Println(err)
		fmt.Println("Exiting...")
		os.Exit(1)
	}

	go handlepacket(packetchan)
	go accept(packetchan, listen)
}

// signal handling (and sending true through done)
func signals(donechan chan bool, packetchan chan packet) {
	sigschan := make(chan os.Signal, 1)
	signal.Notify(sigschan, syscall.SIGINT, syscall.SIGTERM)

	var serveruser packet
	serveruser.Pname = "Server"
	serveruser.Ptime = time.Now()
	serveruser.Pmessage = serveruser.Pname + "> Server going down in 10 seconds!\n"
	serveruser.Ptype = TYPE_MESSAGE

	<- sigschan
	packetchan <- serveruser
	time.Sleep(time.Second * 10)
	donechan <- true
}

// pulls everything together
func main() {
	donechan := make(chan bool, 1)
	packetchan := make(chan packet, 100)

	go signals(donechan, packetchan)
	go start(packetchan)

	fmt.Println("Server running now!")
	<-donechan
	fmt.Println()
	fmt.Println("Server exiting now!")
}
