package main

import (
	"bufio"
	"encoding/gob"
	"fmt"
	"log"
	"net"
	"os"
	"time"
)

type Message struct {
	Text           string
	Time           time.Time
	IsNotification bool
}

func main() {
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		log.Fatal("Error calling for server: ", err)
	}
	defer conn.Close()
	fmt.Println("Established connection to the server at: localhost:8080")
	enc := gob.NewEncoder(conn)
	dec := gob.NewDecoder(conn)
	scanner := bufio.NewScanner(os.Stdin)
	go receiveMessages(dec)

	fmt.Println("Print /help for commands available.")

	for scanner.Scan() {
		msgstr := scanner.Text()
		msg := Message{Text: msgstr, Time: time.Now(), IsNotification: false}
		err := enc.Encode(msg)
		if err != nil {
			log.Println("Error sending message: ", err)
			continue
		}
	}
	if err := scanner.Err(); err != nil {
		log.Println("Error scanning input: ", err)
	}
}

func receiveMessages(dec *gob.Decoder) {
	for {
		var msg Message
		err := dec.Decode(&msg)
		if err != nil {
			log.Println("Error decoding message: ", err)
			return
		}
		if msg.IsNotification {
			fmt.Printf("%s\n", msg.Text)
		} else {
			fmt.Printf("%s Anonymous: %s\n", msg.Time.Format("15:04"), msg.Text)
		}
	}
}
