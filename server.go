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

// connections are handled here
func connections(incomingtext chan string) {
    // setup the protocol and port to listen on
    listen, err := net.Listen("tcp", ":2000")
    if err != nil {
        fmt.Println(err)
        fmt.Println("Exiting...")
        os.Exit(1)
    }

    incomingconn := make(chan *bufio.Writer, 100)

    // this go routine handles all connections and chatting!
    // i could not get lists to work for some reason, so i went
    //  with a slice for connections (cannot remove connections so
    //  will probably die badly when someone leaves early)
    //  and all text is sent to every current connection when received
    //  (again, through it's own go routine)
    go func() {
        connslice := make([]*bufio.Writer, 0)
        for {
            select {
                case newconnect := <- incomingconn:
                    connslice = append(connslice, newconnect)
                case newtext := <- incomingtext:
                    go func(cs []*bufio.Writer, nt string) {
                        for _, v := range cs {
                            v.WriteString(nt)
                            v.Flush()
                        }
                    } (connslice, newtext)
            }
        }
    } ()

    // start accepting connections in a go routine so we can
    //  handle signals without being blocked
    go func() {
        for {
            connection, err := listen.Accept()
            if err != nil {
                fmt.Println(err)
                fmt.Println("Exiting...")
                os.Exit(2)
            }

            // handling each connection in a go routine
            //  so we can accept more connections
            go func(c net.Conn) {
                input := bufio.NewReader(c)
                output := bufio.NewWriter(c)
                output.WriteString("What is your name? ")
                output.Flush()
                name, _ := input.ReadString('\n')
                name = strings.TrimSpace(name)
                incomingconn <- output

                for {
                    line, err := input.ReadString('\n')
                    if err != nil {
                        break
                    } else if len(line) > 2 {
                        incomingtext <- name + "> " + line
                    }
                }

                c.Close()
            } (connection)
        }
    } ()
}

// signal handling (and sending true through done)
func signals(done chan bool, it chan string) {
    sigs := make(chan os.Signal, 1)

    // these are the signals we will respawn to with "sigs" channel
    signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

    // start a go routine so we don't stop execution
    go func() {
        <-sigs       // blocking until we get a signal
        it <- "!! Server> Server going down in 10 seconds!"
        time.Sleep(time.Second * 10)
        done <- true // tell that we are done because of signals
    } ()
}

// pulls everything together
func main() {
    // done channel for signals, and incoming text (so we can send
    //  server information because of shutdown)
    done := make(chan bool, 1)
    incomingtext := make(chan string, 100)

    // setup signal handling then accept connections
    signals(done, incomingtext)
    connections(incomingtext)

    // we are fully up and running now!
    fmt.Println("Server running now!")

    <-done

    fmt.Println()
    fmt.Println("Server exiting now!")
}
