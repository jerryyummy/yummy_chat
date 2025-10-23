package chat

import (
	"encoding/json"
	"sort"
	"sync"
	"time"
)

type ChatManager struct {
	mu sync.RWMutex

	Clients       map[string]*Client // id -> client
	clientsByName map[string]*Client // name -> client

	// message channels
	RegisterChan              chan *Client
	UnregisterChan            chan *Client
	SendMessageChan           chan *Message
	BroadcastNotificationChan chan *Message // system messages (join/leave)

	// rooms & inbox
	Subs      *Subscriptions
	Inbox     InboxStore
	Rooms     map[string]bool   // directory of rooms
	RoomOwner map[string]string // room -> owner
}

var Manager = &ChatManager{
	Clients:                   map[string]*Client{},
	clientsByName:             map[string]*Client{},
	RegisterChan:              make(chan *Client),
	UnregisterChan:            make(chan *Client),
	SendMessageChan:           make(chan *Message),
	BroadcastNotificationChan: make(chan *Message, 16),
	Subs: &Subscriptions{
		UserRooms: map[string]map[string]bool{},
		RoomUsers: map[string]map[string]bool{},
	},
	Inbox:     map[string]map[string]*ThreadPreview{},
	Rooms:     map[string]bool{"general": true},
	RoomOwner: map[string]string{"general": ""}, // 默认房无 owner，可自行移除
}

// 在线用户列表（可排除自己：按 id 或 name）
type ClientJson struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

func (m *ChatManager) ListClients(exclude string) []ClientJson {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ClientJson, 0, len(m.Clients))
	for id, c := range m.Clients {
		if exclude != "" && (exclude == id || exclude == c.Name) {
			continue
		}
		out = append(out, ClientJson{Id: id, Name: c.Name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (manager *ChatManager) Start() {
	for {
		select {
		case client := <-manager.RegisterChan:
			manager.mu.Lock()
			manager.Clients[client.Id] = client
			manager.clientsByName[client.Name] = client
			manager.mu.Unlock()
			manager.BroadcastNotificationChan <- &Message{
				Broadcast:  true,
				Content:    client.Name + " joined",
				OriginName: "Manager",
			}

		case client := <-manager.UnregisterChan:
			manager.mu.Lock()
			delete(manager.Clients, client.Id)
			delete(manager.clientsByName, client.Name)
			manager.mu.Unlock()
			manager.BroadcastNotificationChan <- &Message{
				Broadcast:  true,
				Content:    client.Name + " left",
				OriginName: "Manager",
			}

		case msg := <-manager.SendMessageChan:
			now := time.Now().Unix()

			if msg.Broadcast {
				// 群聊：只推给订阅者
				room := normalizeRoom(msg.Room)
				if room == "" {
					room = "general"
				}
				manager.setRoom(room)

				manager.mu.RLock()
				subs := manager.Subs.RoomUsers[room]
				snapshot := make([]*Client, 0, len(subs))
				for nick := range subs {
					if c := manager.clientsByName[nick]; c != nil {
						snapshot = append(snapshot, c)
					}
				}
				from := manager.Clients[msg.OriginId]
				manager.mu.RUnlock()

				for _, c := range snapshot {
					out := *msg
					out.Room = room // 回传规范化后的房间名
					if from != nil {
						out.OriginId, out.OriginName = from.Id, from.Name
					}
					data, _ := json.Marshal(&out)
					select {
					case c.Send <- data:
					default:
					}
				}
				if from != nil {
					manager.onGroupMessage(room, from.Name, msg.Content, now)
				}

			} else {
				// 私聊：点对点 + 回显给发送者
				manager.mu.RLock()
				toCli := manager.Clients[msg.DestinationId]
				fromCli := manager.Clients[msg.OriginId]
				manager.mu.RUnlock()

				if toCli != nil {
					out := *msg
					if fromCli != nil {
						out.OriginId, out.OriginName = fromCli.Id, fromCli.Name
					}
					data, _ := json.Marshal(&out)
					select {
					case toCli.Send <- data:
					default:
					}
					if fromCli != nil {
						select {
						case fromCli.Send <- data:
						default:
						}
						manager.onPrivateMessage(fromCli.Name, toCli.Name, msg.Content, now)
					}
				}
			}

		case sys := <-manager.BroadcastNotificationChan:
			manager.mu.RLock()
			for _, c := range manager.Clients {
				data, _ := json.Marshal(sys)
				select {
				case c.Send <- data:
				default:
				}
			}
			manager.mu.RUnlock()
		}
	}
}
