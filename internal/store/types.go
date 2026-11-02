package store

import "encoding/json"

type Account struct {
	ID          string
	Source      string
	RemoteID    string
	Nickname    string
	AvatarURL   string
	SessionName string
	Status      string
	Position    int
}

type Author struct {
	RemoteID  string
	Nickname  string
	AvatarURL string
}

type Work struct {
	ID          string
	Source      string
	RemoteID    string
	Kind        string
	Title       string
	Author      *Author
	PublishedAt int64
	DurationMS  int
	Width       int
	Height      int
	ImageCount  int
	IsLive      bool
	CoverURL    string
	CoverHash   string
	Available   *bool // nil preserves the stored availability; new works default to available
	Extra       json.RawMessage
	Seq         int64
	AddedAt     int64 // Source-provided list addition time; zero keeps pagination order.
}

type WorkPage struct {
	AccountID string
	Kind      string
	Phase     string
	Cursor    string
	Complete  bool
	Works     []Work
}

type WorkCursor struct {
	Seq    int64
	WorkID string
}

type SyncState struct {
	Phase          string
	Status         string
	HeadCursor     string
	BackfillCursor string
	HeadSeq        int64
	Pages          int
	Items          int
}
