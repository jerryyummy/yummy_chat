package handlers

import (
	"strings"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/pelusa-v/pelusa-chat.git/internal/chat"
)

// RegisterHandler GET /api/ws/register/:nick
func RegisterHandler(c *websocket.Conn) {
	nick := c.Params("nick")
	id := uuid.NewString()
	client := &chat.Client{Id: id, Name: nick, Conn: c, Send: make(chan []byte, 16)}
	chat.Manager.RegisterChan <- client
	defer func() { chat.Manager.UnregisterChan <- client }()
	go client.WritePump()
	client.ReadPump()
}

// ShowClientsHandler GET /api/clients?exclude=nickOrId
func ShowClientsHandler(c *fiber.Ctx) error {
	ex := c.Query("exclude")
	return c.JSON(chat.Manager.ListClients(ex))
}

// InboxHandler GET /api/inbox/:nick
func InboxHandler(c *fiber.Ctx) error {
	nick := c.Params("nick")
	return c.JSON(chat.Manager.GetInbox(nick))
}

// MarkReadHandler POST /api/inbox/read?nick=&thread_id=
func MarkReadHandler(c *fiber.Ctx) error {
	nick := c.Query("nick")
	thread := c.Query("thread_id")
	if nick == "" || thread == "" {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	chat.Manager.MarkRead(nick, thread)
	return c.SendStatus(fiber.StatusNoContent)
}

// RoomsHandler GET /api/rooms?nick=
func RoomsHandler(c *fiber.Ctx) error {
	nick := c.Query("nick")
	return c.JSON(chat.Manager.ListRoomsWithSub(nick))
}

// CreateRoomHandler POST /api/room/create?nick=&room=
func CreateRoomHandler(c *fiber.Ctx) error {
	nick := strings.TrimSpace(c.Query("nick"))
	room := strings.TrimSpace(c.Query("room"))
	if nick == "" || room == "" {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	ok := chat.Manager.CreateRoom(nick, room)
	if !ok {
		// 已存在或非法名
		return c.SendStatus(fiber.StatusConflict)
	}
	return c.SendStatus(fiber.StatusCreated)
}

// DeleteRoomHandler POST /api/room/delete?nick=&room=
func DeleteRoomHandler(c *fiber.Ctx) error {
	nick := strings.TrimSpace(c.Query("nick"))
	room := strings.TrimSpace(c.Query("room"))
	if nick == "" || room == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing nick or room"})
	}
	ok, reason := chat.Manager.DeleteRoomWithReason(nick, room)
	if !ok {
		code := fiber.StatusForbidden
		if reason == "not_found" {
			code = fiber.StatusNotFound
		} else if reason == "not_owner" {
			code = fiber.StatusForbidden
		}
		return c.Status(code).JSON(fiber.Map{"error": reason})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// SubscribeRoomHandler POST /api/room/subscribe?nick=&room=
func SubscribeRoomHandler(c *fiber.Ctx) error {
	nick := strings.TrimSpace(c.Query("nick"))
	room := strings.TrimSpace(c.Query("room"))
	if nick == "" || room == "" {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	ok := chat.Manager.Subscribe(nick, room)
	if !ok {
		return c.SendStatus(fiber.StatusNotFound) // 房间不存在或非法名
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// UnsubscribeRoomHandler POST /api/room/unsubscribe?nick=&room=
func UnsubscribeRoomHandler(c *fiber.Ctx) error {
	nick := strings.TrimSpace(c.Query("nick"))
	room := strings.TrimSpace(c.Query("room"))
	if nick == "" || room == "" {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	ok := chat.Manager.Unsubscribe(nick, room)
	if !ok {
		return c.SendStatus(fiber.StatusForbidden) // 拥有者不能退订 或 非法名
	}
	return c.SendStatus(fiber.StatusNoContent)
}
