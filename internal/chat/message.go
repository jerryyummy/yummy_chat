package chat

type Message struct {
	Broadcast     bool   `json:"broadcast"`                // true: group; false: private
	Room          string `json:"room,omitempty"`           // group room name
	DestinationId string `json:"destination_id,omitempty"` // private: target client id
	Content       string `json:"content"`                  // message body

	// filled by server before sending
	OriginId   string `json:"origin_id,omitempty"`
	OriginName string `json:"origin_name,omitempty"`
}

type InboxSignal struct {
	Kind string `json:"kind"` // "inbox_update"
}

type ThreadKind string

const (
	ThreadPrivate ThreadKind = "private"
	ThreadGroup   ThreadKind = "group"
)

type ThreadPreview struct {
	ThreadID string     `json:"thread_id"` // u:<peerNick> or g:<room>
	Kind     ThreadKind `json:"kind"`
	Title    string     `json:"title"`
	LastBody string     `json:"last_body"`
	LastTs   int64      `json:"last_ts"`
	Unread   int        `json:"unread"`
}

type Subscriptions struct {
	UserRooms map[string]map[string]bool // nick -> set(room)
	RoomUsers map[string]map[string]bool // room -> set(nick)
}

type InboxStore map[string]map[string]*ThreadPreview // nick -> threadId -> preview
