package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow all connections for testing purposes; replace this with your actual logic
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Client represents a connected WebSocket client.
type Client struct {
	IDRoom string
	UserID int
	conn   *websocket.Conn
	send   chan []byte
}

var (
	clients    = make(map[int]*Client)
	clientsMux sync.Mutex
)

func newClient(conn *websocket.Conn, idRoom string, userId int) *Client {
	return &Client{
		IDRoom: idRoom,
		UserID: userId,
		conn:   conn,
		send:   make(chan []byte),
	}
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print(err)
		return
	}
	log.Println("connected to client", clients)

	// Read query
	idRoom := r.URL.Query().Get("roomId")
	userIDSTr := r.URL.Query().Get("userId")

	userIDInt, err := strconv.Atoi(userIDSTr)
	if err != nil {
		return
	}

	client := newClient(conn, idRoom, userIDInt)
	// Register the client
	clientsMux.Lock()
	clients[client.UserID] = client
	clientsMux.Unlock()

	defer func() {
		// Unregister the client on close
		clientsMux.Lock()
		delete(clients, client.UserID)
		clientsMux.Unlock()
		client.conn.Close()
	}()

	// Listen for messages from the client
	go client.readMessages()

	// Read messages from the client using channel

	// Send messages to the client
	client.writeMessages()
}

func (c *Client) readMessages() {
	for {
		// // Read message from client
		// _, msgByte, err := c.conn.ReadMessage()
		// if err != nil {
		// 	break
		// }

		message := Message{}
		if err := c.conn.ReadJSON(&message); err != nil {
			log.Println(err)
			continue
		}

		broadcastMessage([]byte(message.Content))
	}
}

type Message struct {
	Content string `json:"content"`
}

func (c *Client) writeMessages() {
	for {
		select {
		case _, ok := <-c.send:
			if !ok {
				return
			}

			log.Println("Sending message to client", c.UserID)

			for i := 0; i < 10; i++ {
				message := Message{Content: fmt.Sprintf("Message %d", i)}
				err := c.conn.WriteJSON(message)
				if err != nil {
					log.Println(err)
					return
				}
			}
		}
	}
}

func broadcastMessage(message []byte) {
	clientsMux.Lock()
	defer clientsMux.Unlock()

	roomID := string(message)
	for client := range clients {
		log.Println("Sending message to client 2", client, clients[client], roomID)
		log.Println("room Id: ", clients[client].IDRoom)
		if clients[client].IDRoom == roomID {
			select {
			case clients[client].send <- message:
			default:
				// If unable to send to a client, assume it's disconnected and remove it
				close(clients[client].send)
				delete(clients, client)
			}
		}
	}
}

func main() {
	http.HandleFunc("/ws", handleWebSocket)
	log.Fatal(http.ListenAndServe(":8083", nil))
}
