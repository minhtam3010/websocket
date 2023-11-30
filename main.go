package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Client struct represents a connected client with its associated roomID.
type Client struct {
	conn   *websocket.Conn
	roomID string
	userID int
}

// Mutex to safely access the clients map.
var clientsMu sync.Mutex
var clients = make(map[*websocket.Conn]*Client)

type Message struct {
	Content            string `json:"content"`
	ActionType         string `json:"actionType"`
	FirstUserID        User   `json:"firstUserId"`
	SecondUserID       User   `json:"secondUserId"`
	TimeToSpeak        int    `json:"timeToSpeak"`
	ChangeRole         bool   `json:"changeRole"`
	StartGameCountDown int    `json:"startGameCountDown"`
	IntroduceRole      string `json:"introduceRole"`
}

type User struct {
	ID   int    `json:"id"`
	Role string `json:"role"`
}

func handleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()

	// Assume that the roomID is passed as a query parameter in the URL.
	roomID := r.URL.Query().Get("roomId")
	userIDSTr := r.URL.Query().Get("userId")

	var (
		userID int
	)

	userID, err = strconv.Atoi(userIDSTr)
	if err != nil {
		return
	}

	client := &Client{
		conn:   conn,
		roomID: roomID,
		userID: userID,
	}

	// Register the client.
	clientsMu.Lock()
	clients[conn] = client
	clientsMu.Unlock()

	defer func() {
		// Unregister the client when the connection is closed.
		clientsMu.Lock()
		delete(clients, conn)
		clientsMu.Unlock()
	}()

	for {
		var (
			message Message
		)
		err := conn.ReadJSON(&message)
		if err != nil {
			fmt.Println(err)
			return
		}

		switch message.ActionType {
		case "startGame":
			respondedMessage := handleStartGame(roomID)
			// Broadcast the message to all clients in the same room.
			respondedMessage.StartGameCountDown = 3
			respondedMessage.ActionType = "startGame"
			respondedMessage.TimeToSpeak = 10
			err = broadcastMessage(roomID, respondedMessage)
			if err != nil {
				fmt.Println(err)
				return
			}
		case "countdown":
			// Broadcast the message to all clients in the same room.
			err = broadcastMessage(roomID, message)
			if err != nil {
				fmt.Println(err)
				return
			}
		case "changeRole":
			respondedMessage := handleChangeRole(message)
			respondedMessage.ChangeRole = true
			// Broadcast the message to all clients in the same room.
			err = broadcastMessage(roomID, respondedMessage)
			if err != nil {
				fmt.Println(err)
				return
			}
		}

	}
}

func handleStartGame(roomID string) Message {
	var firstRole = ""
	var (
		res Message
	)
	for _, client := range clients {
		if client.roomID == roomID {
			if firstRole != "" && client.userID == res.FirstUserID.ID {
				continue
			}
			if firstRole == "" {
				whoSpeakFirst := rand.Intn(2)
				if whoSpeakFirst == 0 {
					firstRole = "Speaker"
				} else {
					firstRole = "Listener"
				}
				res.FirstUserID.ID = client.userID
				res.FirstUserID.Role = firstRole
			} else {
				if firstRole == "Speaker" {
					firstRole = "Listener"
				} else {
					firstRole = "Speaker"
				}
				res.SecondUserID.ID = client.userID
				res.SecondUserID.Role = firstRole
			}

		}
	}
	// assgin 2 minutes to speak
	// res.TimeToSpeak = 120
	return res
}

func handleChangeRole(message Message) Message {
	firstRole := message.FirstUserID.Role
	message.FirstUserID.Role = message.SecondUserID.Role
	message.SecondUserID.Role = firstRole
	message.TimeToSpeak = 10
	return message
}

func broadcastMessage(roomID string, message Message) error {
	// Iterate over all connected clients and send the message to clients in the same room.
	clientsMu.Lock()
	defer clientsMu.Unlock()

	for _, client := range clients {
		if client.roomID == roomID {
			err := client.conn.WriteJSON(message)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func main() {
	http.HandleFunc("/ws", handleConnection)

	fmt.Println("WebSocket server listening on :8083")
	err := http.ListenAndServe(":8083", nil)
	if err != nil {
		fmt.Println(err)
	}
}
