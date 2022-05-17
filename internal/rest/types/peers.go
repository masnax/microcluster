package types

// Peer represents information about a dqlite cluster member.
type Peer struct {
	Name        string          `json:"name" yaml:"name"`
	Addresses   AddrPorts       `json:"addresses" yaml:"addresses"`
	Role        string          `json:"role" yaml:"role"`
	Certificate X509Certificate `json:"certificate" yaml:"certificate"`
	Status      PeerStatus      `json:"status" yaml:"status"`
}

// PeerStatus represents the online status of a peer.
type PeerStatus string

const (
	// PeerOnline should be the PeerStatus when the node is online and reachable.
	PeerOnline PeerStatus = "ONLINE"

	// PeerUnreachable should be the PeerStatus when we were not able to connect to the node.
	PeerUnreachable PeerStatus = "UNREACHABLE"

	// PeerNotTrusted should be the PeerStatus when there is no remote yaml entry for this node.
	PeerNotTrusted PeerStatus = "NOT TRUSTED"

	// PeerNotFound should be the PeerStatus when the node was not found in dqlite.
	PeerNotFound PeerStatus = "NOT FOUND"
)
