package chat

import (
	"path"
	"sort"
	"strings"
)

// 规范化房间名：去首尾空格、合并多余斜杠、去前导斜杠
func normalizeRoom(room string) string {
	r := strings.TrimSpace(room)
	if r == "" {
		return ""
	}
	r = path.Clean("/" + r) // 合并多余斜杠/处理 . ..
	r = strings.TrimPrefix(r, "/")
	return r
}

func (m *ChatManager) setRoom(room string) {
	r := normalizeRoom(room)
	if r == "" {
		return
	}
	m.Rooms[r] = true
}

// CreateRoom: 创建房间并设置拥有者；自动订阅创建者
func (m *ChatManager) CreateRoom(owner, room string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	r := normalizeRoom(room)
	if r == "" {
		return false
	}
	// 已存在就直接返回 false
	if m.Rooms[r] {
		return false
	}
	// 目录与拥有者
	m.Rooms[r] = true
	if m.RoomOwner == nil {
		m.RoomOwner = map[string]string{}
	}
	m.RoomOwner[r] = owner

	// 自动订阅创建者
	if _, ok := m.Subs.UserRooms[owner]; !ok {
		m.Subs.UserRooms[owner] = map[string]bool{}
	}
	m.Subs.UserRooms[owner][r] = true

	if _, ok := m.Subs.RoomUsers[r]; !ok {
		m.Subs.RoomUsers[r] = map[string]bool{}
	}
	m.Subs.RoomUsers[r][owner] = true

	return true
}

// DeleteRoom: 只有拥有者能删除；删除后所有人订阅、会话、目录全部清除
func (m *ChatManager) DeleteRoom(owner, room string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	r := normalizeRoom(room)
	if r == "" {
		return false
	}
	if m.RoomOwner == nil || m.RoomOwner[r] != owner {
		return false // 不是拥有者
	}

	// 清理“房间 -> 用户”订阅关系
	if subs, ok := m.Subs.RoomUsers[r]; ok {
		for nick := range subs {
			// 清理“用户 -> 房间”订阅关系
			if ur, ok2 := m.Subs.UserRooms[nick]; ok2 {
				delete(ur, r)
				if len(ur) == 0 {
					delete(m.Subs.UserRooms, nick)
				}
			}
			// 清理每个用户的该群会话预览
			if m.Inbox != nil {
				if _, ok2 := m.Inbox[nick]; ok2 {
					delete(m.Inbox[nick], "g:"+r)
				}
			}
		}
		delete(m.Subs.RoomUsers, r)
	}

	// 清理目录与拥有者
	delete(m.Rooms, r)
	if m.RoomOwner != nil {
		delete(m.RoomOwner, r)
	}
	return true
}

func (m *ChatManager) Subscribe(nick, room string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	r := normalizeRoom(room)
	if r == "" {
		return false
	}
	// 房间不存在则不可订阅
	if !m.Rooms[r] {
		return false
	}

	if _, ok := m.Subs.UserRooms[nick]; !ok {
		m.Subs.UserRooms[nick] = map[string]bool{}
	}
	m.Subs.UserRooms[nick][r] = true

	if _, ok := m.Subs.RoomUsers[r]; !ok {
		m.Subs.RoomUsers[r] = map[string]bool{}
	}
	m.Subs.RoomUsers[r][nick] = true
	return true
}

func (m *ChatManager) Unsubscribe(nick, room string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	r := normalizeRoom(room)
	if r == "" {
		return false
	}
	// 拥有者不能退订（只能删除）
	if m.RoomOwner != nil && m.RoomOwner[r] == nick {
		return false
	}

	// 仅移除“用户->房间”的订阅关系
	if s, ok := m.Subs.UserRooms[nick]; ok {
		delete(s, r)
		if len(s) == 0 {
			delete(m.Subs.UserRooms, nick)
		}
	}
	// 仅移除“房间->用户”的订阅关系
	if s, ok := m.Subs.RoomUsers[r]; ok {
		delete(s, nick)
	}

	// 清理该用户该群的会话预览
	m.ensureInbox(nick)
	delete(m.Inbox[nick], "g:"+r)
	return true
}

// 返回：room 列表，包含订阅状态与 owner 信息
func (m *ChatManager) ListRoomsWithSub(nick string) []map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	seen := map[string]bool{}
	res := make([]map[string]interface{}, 0, len(m.Rooms))

	for room := range m.Rooms {
		r := normalizeRoom(room)
		if r == "" || seen[r] {
			continue
		}
		seen[r] = true
		_, sub := m.Subs.UserRooms[nick][r]
		owner := ""
		if m.RoomOwner != nil {
			owner = m.RoomOwner[r]
		}
		res = append(res, map[string]interface{}{
			"room":       r,
			"subscribed": sub,
			"owner":      owner,
			"is_owner":   owner != "" && owner == nick,
		})
	}
	sort.Slice(res, func(i, j int) bool { return res[i]["room"].(string) < res[j]["room"].(string) })
	return res
}

// DeleteRoomWithReason: 返回是否成功 + 失败原因(方便前端提示/排查)
func (m *ChatManager) DeleteRoomWithReason(owner, room string) (bool, string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r := normalizeRoom(room)
	if r == "" {
		return false, "invalid_room"
	}
	if !m.Rooms[r] {
		return false, "not_found"
	}
	if m.RoomOwner == nil {
		return false, "no_owner"
	}
	// 大小写不敏感比较
	realOwner := m.RoomOwner[r]
	if realOwner == "" || !strings.EqualFold(realOwner, owner) {
		return false, "not_owner"
	}

	// 清理“房间 -> 用户”订阅关系
	if subs, ok := m.Subs.RoomUsers[r]; ok {
		for nick := range subs {
			if ur, ok2 := m.Subs.UserRooms[nick]; ok2 {
				delete(ur, r)
				if len(ur) == 0 {
					delete(m.Subs.UserRooms, nick)
				}
			}
			if m.Inbox != nil {
				if _, ok2 := m.Inbox[nick]; ok2 {
					delete(m.Inbox[nick], "g:"+r)
				}
			}
		}
		delete(m.Subs.RoomUsers, r)
	}
	delete(m.Rooms, r)
	delete(m.RoomOwner, r)
	return true, ""
}
