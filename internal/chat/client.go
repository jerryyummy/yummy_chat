package chat

import (
	"encoding/json"

	"github.com/gofiber/contrib/websocket"
)

type Client struct {
	Id   string
	Name string
	Conn ConnLike
	Send chan []byte
}

type ConnLike interface {
	ReadMessage() (int, []byte, error)
	WriteMessage(int, []byte) error
	Close() error
}

func (c *Client) ReadPump() {
	for {
		_, data, err := c.Conn.ReadMessage()
		if err != nil {
			Manager.UnregisterChan <- c
			return
		}
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		msg.OriginId = c.Id
		msg.OriginName = c.Name
		Manager.SendMessageChan <- &msg
	}
}

func (c *Client) WritePump() {
	for data := range c.Send {
		_ = c.Conn.WriteMessage(websocket.TextMessage, data)
	}
}
