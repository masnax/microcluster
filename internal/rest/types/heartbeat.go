package types

type HeartbeatInfo struct {
	BeginRound     bool
	MaxSchema      int
	ClusterMembers map[string]ClusterMember
	Count          int
}
