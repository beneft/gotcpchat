package main

import (
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

type Message struct {
	Text           string
	Time           time.Time
	IsNotification bool
}

type Client struct {
	conn  net.Conn
	enc   *gob.Encoder
	lobby string
}

var clients []*Client
var lobbies []string
var mutex sync.Mutex

func main() {
	logFile, err := os.OpenFile("server.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal("Error opening log file: ", err)
	}
	defer logFile.Close()

	log.SetOutput(io.MultiWriter(os.Stdout, logFile))

	log.Printf("%s Server is starting...", time.Now().Format("01-02 15:04:05"))

	listener, err := net.Listen("tcp", "localhost:8080")
	if err != nil {
		log.Fatal("Server listener initialization error: ", err)
	}
	defer listener.Close()

	fmt.Println("Server is listening to the port 8080 at localhost:8080")
	log.Printf("%s Server is listening to the port 8080 at localhost:8080", time.Now().Format("01-02 15:04:05"))

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("%s Error accepting connection: %s", time.Now().Format("01-02 15:04:05"), err)
			continue
		}
		client := &Client{
			conn: conn,
			enc:  gob.NewEncoder(conn),
		}
		mutex.Lock()
		clients = append(clients, client)
		mutex.Unlock()
		log.Printf("%s A new client has connected: %s", time.Now().Format("01-02 15:04:05"), conn.RemoteAddr())
		go handleClient(client)
	}
}

func handleClient(client *Client) {

	dec := gob.NewDecoder(client.conn)

	for {
		var msg Message
		err := dec.Decode(&msg)
		if err != nil {
			log.Println("Error decoding message: ", err)
			return
		}
		log.Printf("%s A message '%s' has been received from: %s", time.Now().Format("01-02 15:04:05"), msg.Text, client.conn.RemoteAddr())

		if strings.HasPrefix(msg.Text, "/") {
			handleCommand(client, msg.Text)
		} else if client.lobby == "" {
			sendServerNotification(client, "You have not joined to any lobbies. Try /list to get lobby list.")
		} else {
			broadcastMsg(client, msg)
		}
	}
}

func handleCommand(client *Client, message string) {
	split := strings.Split(message, " ")
	command := split[0]
	switch command {
	case "/exit":
		sendServerNotification(client, "You have left the server. Rerun to rejoin.")
		removeClient(client)
	case "/create":
		lobby := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(message, "/create")))
		if lobby == "" {
			sendServerNotification(client, "Please specify a lobby name.")
			return
		}
		for _, element := range lobbies {
			if element == lobby {
				sendServerNotification(client, "Lobby with this name already exists. Try another name or join it.")
				return
			}
		}
		lobbies = append(lobbies, lobby)
		client.lobby = lobby
		log.Printf("%s A client %s has created new lobby: %s", time.Now().Format("01-02 15:04:05"), client.conn.RemoteAddr(), lobby)
		log.Printf("%s A client %s has joined the lobby: %s", time.Now().Format("01-02 15:04:05"), client.conn.RemoteAddr(), lobby)
		sendServerNotification(client, fmt.Sprintf("You have created and joined the lobby '%s'.", lobby))
	case "/join":
		lobby := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(message, "/join")))
		if lobby == "" {
			sendServerNotification(client, "Please specify a lobby name.")
			return
		}
		flag := false
		for _, element := range lobbies {
			if element == lobby {
				flag = true
			}
		}
		count := 0
		for _, c := range clients {
			if c.lobby == lobby {
				count++
			}
		}
		if !flag {
			sendServerNotification(client, "Lobby is not found. Try another or create new.")
			return
		}
		client.lobby = lobby
		log.Printf("%s A client %s has joined the lobby: %s", time.Now().Format("01-02 15:04:05"), client.conn.RemoteAddr(), lobby)
		sendServerNotification(client, fmt.Sprintf("You have joined the lobby '%s' with %d users.", lobby, count))
		sendServerNotification(client, "Ready to chat.")
		mutex.Lock()
		defer mutex.Unlock()
		log.Printf("%s Broadcasting...", time.Now().Format("01-02 15:04:05"))
		for _, c := range clients {
			if c != client && c.lobby == client.lobby {
				msg := fmt.Sprintf("Someone has joined the lobby! (Now %d users in here)", count+1)
				sendServerNotification(c, msg)
			}
		}
	case "/disconnect":
		lobby := client.lobby
		if client.lobby == "" {
			sendServerNotification(client, "You have not joined any lobbies yet.")
			return
		}
		client.lobby = ""
		log.Printf("%s A client %s has left the lobby", time.Now().Format("01-02 15:04:05"), client.conn.RemoteAddr())
		sendServerNotification(client, "You have disconnected from the lobby.")
		count := 0
		for _, c := range clients {
			if c.lobby == lobby {
				count++
			}
		}
		mutex.Lock()
		defer mutex.Unlock()
		log.Printf("%s Broadcasting...", time.Now().Format("01-02 15:04:05"))
		for _, c := range clients {
			if c != client && c.lobby == lobby {
				msg := fmt.Sprintf("Someone has left the lobby! :( (Now %d users in here)", count)
				sendServerNotification(c, msg)
			}
		}
	case "/help":
		helpmsg := "/help - to see this message\n/exit - disconnect from the server\n/create - create a new lobby\n/join - join an existing lobby\n/disconnect - leave current lobby\n/list - get the list of existing lobbies"
		sendServerNotification(client, helpmsg)
	case "/list":
		var lobbyusers []string
		for _, lobby := range lobbies {
			count := 0
			for _, c := range clients {
				if c.lobby == lobby {
					count++
				}
			}
			lobbyusers = append(lobbyusers, fmt.Sprintf("%s (%d users)", lobby, count))
		}
		list := fmt.Sprintf("Available lobbies:\n%s", strings.Join(lobbyusers, ", "))
		sendServerNotification(client, list)
	default:
		log.Printf("%s Unknown command '%s' received from client: %s", time.Now().Format("01-02 15:04:05"), command, client.conn.RemoteAddr())
	}
}

func sendServerNotification(client *Client, text string) {
	serverNotification := Message{Text: text, Time: time.Now(), IsNotification: true}
	err := client.enc.Encode(serverNotification)
	if err != nil {
		log.Println("Error encoding server notification: ", err)
	}
}

func removeClient(client *Client) {
	mutex.Lock()
	defer mutex.Unlock()
	for i, c := range clients {
		if c == client {
			clients = append(clients[:i], clients[i+1:]...)
			break
		}
	}
	log.Printf("%s Client %s has left the server.", time.Now().Format("01-02 15:04:05"), client.conn.RemoteAddr())
}

func broadcastMsg(sender *Client, message Message) {
	mutex.Lock()
	defer mutex.Unlock()
	log.Printf("%s Broadcasting...", time.Now().Format("01-02 15:04:05"))
	for _, c := range clients {
		if c != sender && c.lobby == sender.lobby {
			err := c.enc.Encode(message)
			if err != nil {
				log.Println("Error encoding message: ", err)
			}
			log.Printf("%s A message '%s' was sent to: %s", time.Now().Format("01-02 15:04:05"), message.Text, c.conn.RemoteAddr())
		}
	}
}
