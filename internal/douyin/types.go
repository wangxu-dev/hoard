package douyin

type Status struct {
	Ready   bool   `json:"ready"`
	Open    bool   `json:"open"`
	URL     string `json:"url"`
	Loading bool   `json:"loading"`
}

type LoginState struct {
	LoggedIn bool   `json:"loggedIn"`
	Nickname string `json:"nickname"`
	UID      string `json:"uid"`
	Detail   string `json:"detail"`
}

type Identity struct {
	LoginState
	RemoteID     string `json:"remoteId"`
	Avatar       string `json:"avatar"`
	AuthRequired bool   `json:"authRequired"`
}

type LikeItem struct {
	ID        string   `json:"id"`
	Desc      string   `json:"desc"`
	Author    string   `json:"author"`
	Cover     string   `json:"cover"`
	Duration  int      `json:"duration"`
	CreatedAt int64    `json:"createdAt"`
	AwemeType int      `json:"awemeType"`
	PlayURL   string   `json:"playUrl"`
	SourceURL string   `json:"sourceUrl"`
	Images    []string `json:"images"`
}

type LikesResult struct {
	OK      bool       `json:"ok"`
	Items   []LikeItem `json:"items"`
	Cursor  string     `json:"cursor"`
	HasMore bool       `json:"hasMore"`
	Error   string     `json:"error"`
}
