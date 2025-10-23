package chat

import (
	"encoding/json"
	"sort"
)

// 确保某个用户的 inbox map 存在
func (m *ChatManager) ensureInbox(nick string) {
	if _, ok := m.Inbox[nick]; !ok {
		m.Inbox[nick] = map[string]*ThreadPreview{}
	}
}

func (m *ChatManager) GetInbox(nick string) []*ThreadPreview {
	m.mu.RLock()
	defer m.mu.RUnlock()
	m.ensureInbox(nick)
	list := make([]*ThreadPreview, 0, len(m.Inbox[nick]))
	for _, p := range m.Inbox[nick] {
		cp := *p
		list = append(list, &cp)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].LastTs > list[j].LastTs })
	return list
}

func (m *ChatManager) MarkRead(nick, threadID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureInbox(nick)
	if p, ok := m.Inbox[nick][threadID]; ok {
		p.Unread = 0
	}
}

// 轻量 signal：提示前端刷新 inbox（不包含消息正文）
func (m *ChatManager) pushInboxSignal(nick string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if c, ok := m.clientsByName[nick]; ok && c != nil {
		b, _ := json.Marshal(&InboxSignal{Kind: "inbox_update"})
		select {
		case c.Send <- b:
		default:
		}
	}
}

// 私聊消息到达时更新双方的 inbox
func (m *ChatManager) onPrivateMessage(fromNick, toNick, body string, ts int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureInbox(fromNick)
	m.ensureInbox(toNick)

	m.Inbox[fromNick]["u:"+toNick] = &ThreadPreview{
		ThreadID: "u:" + toNick, Kind: ThreadPrivate, Title: toNick,
		LastBody: body, LastTs: ts, Unread: 0,
	}
	if prev, ok := m.Inbox[toNick]["u:"+fromNick]; ok {
		prev.LastBody, prev.LastTs = body, ts
		prev.Unread += 1
	} else {
		m.Inbox[toNick]["u:"+fromNick] = &ThreadPreview{
			ThreadID: "u:" + fromNick, Kind: ThreadPrivate, Title: fromNick,
			LastBody: body, LastTs: ts, Unread: 1,
		}
	}
	go m.pushInboxSignal(fromNick)
	go m.pushInboxSignal(toNick)
}

// 群聊消息到达时更新订阅者的 inbox
func (m *ChatManager) onGroupMessage(room, fromNick, body string, ts int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r := normalizeRoom(room)
	if r == "" {
		return
	}
	m.Rooms[r] = true

	users := m.Subs.RoomUsers[r]
	if users == nil {
		return
	}

	for nick := range users {
		m.ensureInbox(nick)
		unread := 0
		if nick != fromNick {
			unread = 1
		}
		key := "g:" + r
		if prev, ok := m.Inbox[nick][key]; ok {
			prev.LastBody, prev.LastTs = body, ts
			prev.Unread += unread
		} else {
			m.Inbox[nick][key] = &ThreadPreview{
				ThreadID: key, Kind: ThreadGroup, Title: r,
				LastBody: body, LastTs: ts, Unread: unread,
			}
		}
		go m.pushInboxSignal(nick)
	}
}
