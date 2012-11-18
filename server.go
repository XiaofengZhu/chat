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

// the packet to hold information
type packet struct {
	Pname       string
	Pconnection net.Conn
	Poutput     *bufio.Writer
	Ptime       time.Time
	Pmessage    string
	Ptype       int
	Ptimestamp  bool
}

// the types of packages
const (
	TYPE_LOGIN = iota
	TYPE_MESSAGE
	TYPE_LOGOUT
	TYPE_OPTION
)

// this will check if the name is already used before adding them to the list.
// - only 1 name may be used, this is to make whispers work
func logincheck(packetrec packet, connectionsmap *map[string]packet) {
	nameused := false
	for _, users := range *connectionsmap {
		usertest := strings.ToLower(users.Pname)
		packtest := strings.ToLower(packetrec.Pname)
		if strings.Contains(usertest, packtest) && len(usertest) == len(packtest) {
			nameused = true
			break
		}
	}

	if nameused {
		packetrec.Poutput.WriteString("Name [" + packetrec.Pname)
		packetrec.Poutput.WriteString("] already in use!\n")
		packetrec.Poutput.Flush()
		packetrec.Pconnection.Close()
	} else {
		(*connectionsmap)[packetrec.Pconnection.RemoteAddr().String()] = packetrec
	}
}

// send a message to all connections
func handlemessage(message packet, connections map[string]packet) {
	for _, user := range connections {
		if user.Ptimestamp {
			timestamp := message.Ptime.Format("2006-01-02 15:04:05")
			user.Poutput.WriteString("[" + timestamp + "] ")
		}
		user.Poutput.WriteString(message.Pname + "> ")
		user.Poutput.WriteString(message.Pmessage)
		user.Poutput.Flush()
	}
}

// handle the connection
// - get the user's name
// - setup the packet information (does not change much)
// - take all user input, until the user leaves
// - log the user out and close the connection
func handleconnection(c net.Conn, packetchan chan packet) {
	var user packet
	input := bufio.NewReader(c)
	output := bufio.NewWriter(c)
	output.WriteString("What is your name? ")
	output.Flush()
	name, _ := input.ReadString('\n')

	user.Pname = strings.TrimSpace(name)
	user.Pconnection = c
	user.Poutput = output
	user.Ptime = time.Now()
	user.Ptype = TYPE_LOGIN
	packetchan <- user

	for {
		line, err := input.ReadString('\n')
		if err != nil {
			break
		} else if len(line) > 2 {
			user.Pmessage = line
			if line[0] == '/' {
				user.Ptype = TYPE_OPTION
			} else {
				user.Ptype = TYPE_MESSAGE
			}
			user.Ptime = time.Now()
			packetchan <- user
		}
	}

	user.Ptype = TYPE_LOGOUT
	user.Ptime = time.Now()
	packetchan <- user
	user.Pconnection.Close()
}

// handles any option (/ commands)
func handleoption(packetrec packet, connectionsmap *map[string]packet) {
	connections := *connectionsmap
	if strings.Contains(packetrec.Pmessage, "/timestamp") {
		user := connections[packetrec.Pconnection.RemoteAddr().String()]
		if user.Ptimestamp == false {
			user.Ptimestamp = true
		} else {
			user.Ptimestamp = false
		}
		(*connectionsmap)[packetrec.Pconnection.RemoteAddr().String()] = user
	}

	if strings.Contains(packetrec.Pmessage, "/listusers") {
		packetrec.Poutput.WriteString("Current users:\n")
		for _, user := range connections {
			packetrec.Poutput.WriteString("- " + user.Pname + "\n")
		}
		packetrec.Poutput.Flush()
	}

	if strings.Contains(packetrec.Pmessage, "/whisper") && strings.Count(packetrec.Pmessage, "\"") == 2 {
		tempslice := strings.Split(packetrec.Pmessage, "\"")
		present := false
		var user packet
		for _, usertest := range connections {
			if strings.Contains(strings.ToLower(tempslice[1]), strings.ToLower(usertest.Pname)) {
				present = true
				user = usertest
				break
			}
		}
		if present {
			user.Poutput.WriteString("[Whisper] " + packetrec.Pname)
			user.Poutput.WriteString("> " + strings.TrimSpace(tempslice[2]) + "\n")
			user.Poutput.Flush()
		} else {
			packetrec.Poutput.WriteString("# Cannot find user")
			packetrec.Poutput.WriteString(" [" + tempslice[1] + "]!\n")
			packetrec.Poutput.Flush()
		}
	}

	if strings.Contains(packetrec.Pmessage, "/help") {
		message := "Commands:\n"
		message = message + "    /timestamp\n"
		message = message + "    /listusers\n"
		message = message + "    /whisper \"<user>\" <message>\n"
		message = message + "    /help\n"
		message = message + "    /quit\n"
		packetrec.Poutput.WriteString(message)
		packetrec.Poutput.Flush()
	}

	if strings.Contains(packetrec.Pmessage, "/quit") {
		packetrec.Ptype = TYPE_LOGOUT
		packetrec.Ptime = time.Now()
		delete(*connectionsmap, packetrec.Pconnection.RemoteAddr().String())
		packetrec.Pconnection.Close()
	}
}

// handle all packets
// - user login: stores user in with the rest of the connections
// - message: send the message to all connected users
// - user logout: remove the user from the connection list
func handlepacket(packetchan chan packet) {
	connections := make(map[string]packet)
	for {
		packetrec := <-packetchan

		switch packetrec.Ptype {
		case TYPE_LOGIN:
			go logincheck(packetrec, &connections)

		case TYPE_MESSAGE:
			go handlemessage(packetrec, connections)

		case TYPE_LOGOUT:
			delete(connections, packetrec.Pconnection.RemoteAddr().String())

		case TYPE_OPTION:
			go handleoption(packetrec, &connections)

		default:
			// nothing
		}
	}
}

// accept all connections and start a go routine to handle them
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

// start sets up the listening information,
//  starts to handle all packets,
//  finally starts accepting connections
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

// signal handling
// - when a signal is received send a message from the server
// - send information to main to stop the blocking receive
func signals(donechan chan bool, packetchan chan packet) {
	sigschan := make(chan os.Signal, 1)
	signal.Notify(sigschan, syscall.SIGINT, syscall.SIGTERM)

	var serveruser packet
	serveruser.Pname = "Server"
	serveruser.Ptime = time.Now()
	serveruser.Poutput = bufio.NewWriter(os.Stdout)
	serveruser.Pmessage = serveruser.Pname + "> Server going down in 5 seconds!\n"
	serveruser.Ptype = TYPE_MESSAGE

	<-sigschan
	packetchan <- serveruser
	time.Sleep(time.Second * 5)
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
