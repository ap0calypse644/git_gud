package timeline

// CreepClusterTimeline is a compact, deterministic spatial summary of living
// lane creeps. It deliberately stops short of assigning top/mid/bottom lanes or
// coaching meaning. The explicit method/radius fields make the approximation
// auditable and patch-safe for later validation.
type CreepClusterTimeline struct {
	Available             bool                `json:"available"`
	Method                string              `json:"method"`
	SampleIntervalSeconds float64             `json:"sample_interval_seconds"`
	ClusterRadiusWorld    float64             `json:"cluster_radius_world"`
	ClusterRadiusTimeline float64             `json:"cluster_radius_timeline"`
	Frames                []CreepClusterFrame `json:"frames"`
}

// CreepClusterFrame is the last replay-observed creep state at or before an
// integer match-second boundary. Clusters from different teams are never
// connected to each other.
type CreepClusterFrame struct {
	T        float64        `json:"t"`
	Clusters []CreepCluster `json:"clusters"`
}

// CreepCluster is one same-team connected spatial component. CenterX/CenterY
// use the same Source 2 cell-coordinate scale as HeroSample. LaneCreepCount
// includes ordinary CDOTA_BaseNPC_Creep_Lane entities; SiegeCreepCount counts
// CDOTA_BaseNPC_Creep_Siege entities separately.
type CreepCluster struct {
	Team                      int     `json:"team"`
	CenterX                   float64 `json:"center_x"`
	CenterY                   float64 `json:"center_y"`
	CreepCount                int     `json:"creep_count"`
	LaneCreepCount            int     `json:"lane_creep_count"`
	SiegeCreepCount           int     `json:"siege_creep_count"`
	MaxMemberDistanceTimeline float64 `json:"max_member_distance_timeline"`
	MaxMemberDistanceWorld    float64 `json:"max_member_distance_world"`
}
