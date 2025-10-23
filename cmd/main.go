package main

import (
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/pelusa-v/pelusa-chat.git/internal/chat"
	"github.com/pelusa-v/pelusa-chat.git/internal/handlers"
)

func main() {
	app := fiber.New()

	// 启动聊天管理器
	go chat.Manager.Start()

	// 只暴露 public 静态资源目录（安全、包含所有页面）
	app.Static("/", "./public")

	// WS & APIs
	app.Get("/api/ws/register/:nick", websocket.New(handlers.RegisterHandler))
	app.Get("/api/clients", handlers.ShowClientsHandler) // ?exclude=nickOrId

	app.Get("/api/inbox/:nick", handlers.InboxHandler)
	app.Post("/api/inbox/read", handlers.MarkReadHandler) // ?nick=&thread_id=

	app.Get("/api/rooms", handlers.RoomsHandler)                       // ?nick=
	app.Post("/api/room/create", handlers.CreateRoomHandler)           // ?nick=&room=
	app.Post("/api/room/delete", handlers.DeleteRoomHandler)           // ?nick=&room=
	app.Post("/api/room/subscribe", handlers.SubscribeRoomHandler)     // ?nick=&room=
	app.Post("/api/room/unsubscribe", handlers.UnsubscribeRoomHandler) // ?nick=&room=

	// 页面（相对路径！指向 public/）
	app.Get("/", func(c *fiber.Ctx) error { return c.SendFile("public/views/register.html") })
	app.Get("/inbox", func(c *fiber.Ctx) error { return c.SendFile("public/views/inbox.html") })
	app.Get("/private", func(c *fiber.Ctx) error { return c.SendFile("public/views/private.html") })
	app.Get("/group", func(c *fiber.Ctx) error { return c.SendFile("public/views/group.html") })

	app.Listen("127.0.0.1:3000")
}
