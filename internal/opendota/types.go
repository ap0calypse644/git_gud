package opendota

type RecentMatch struct {
	MatchID    int64 `json:"match_id"`
	PlayerSlot int   `json:"player_slot"`
	HeroID     int   `json:"hero_id"`
	StartTime  int64 `json:"start_time"`
	Duration   int   `json:"duration"`
	GameMode   int   `json:"game_mode"`
	LobbyType  int   `json:"lobby_type"`
	RadiantWin bool  `json:"radiant_win"`
	Kills      int   `json:"kills"`
	Deaths     int   `json:"deaths"`
	Assists    int   `json:"assists"`
}

type Player struct {
	AccountID   uint32 `json:"account_id"`
	PlayerSlot  int    `json:"player_slot"`
	HeroID      int    `json:"hero_id"`
	Kills       int    `json:"kills"`
	Deaths      int    `json:"deaths"`
	Assists     int    `json:"assists"`
	LastHits    int    `json:"last_hits"`
	GoldPerMin  int    `json:"gold_per_min"`
	XPPerMin    int    `json:"xp_per_min"`
	HeroDamage  int    `json:"hero_damage"`
	TowerDamage int    `json:"tower_damage"`
	NetWorth    int    `json:"net_worth"`
}

type Match struct {
	MatchID    int64    `json:"match_id"`
	StartTime  int64    `json:"start_time"`
	Duration   int      `json:"duration"`
	Cluster    int      `json:"cluster"`
	ReplaySalt uint64   `json:"replay_salt"`
	ReplayURL  string   `json:"replay_url"`
	Version    int      `json:"version"`
	Players    []Player `json:"players"`
	ODData     struct {
		HasAPI     bool `json:"has_api"`
		HasGCData  bool `json:"has_gcdata"`
		HasParsed  bool `json:"has_parsed"`
		HasArchive bool `json:"has_archive"`
	} `json:"od_data"`
}

func (m Match) PlayerByAccountID(accountID uint32) (Player, bool) {
	for _, p := range m.Players {
		if p.AccountID == accountID {
			return p, true
		}
	}
	return Player{}, false
}
