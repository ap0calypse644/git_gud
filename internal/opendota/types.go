package opendota

type RecentMatch struct {
	MatchID    int64  `json:"match_id"`
	PlayerSlot int    `json:"player_slot"`
	HeroID     int    `json:"hero_id"`
	StartTime  int64  `json:"start_time"`
	Duration   int    `json:"duration"`
	GameMode   int    `json:"game_mode"`
	LobbyType  int    `json:"lobby_type"`
	RadiantWin bool   `json:"radiant_win"`
	Kills      int    `json:"kills"`
	Deaths     int    `json:"deaths"`
	Assists    int    `json:"assists"`
}

type Match struct {
	MatchID    int64  `json:"match_id"`
	StartTime  int64  `json:"start_time"`
	Duration   int    `json:"duration"`
	Cluster    int    `json:"cluster"`
	ReplaySalt uint64 `json:"replay_salt"`
	ReplayURL  string `json:"replay_url"`
	Version    int    `json:"version"`
	ODData     struct {
		HasAPI     bool `json:"has_api"`
		HasGCData  bool `json:"has_gcdata"`
		HasParsed  bool `json:"has_parsed"`
		HasArchive bool `json:"has_archive"`
	} `json:"od_data"`
}
