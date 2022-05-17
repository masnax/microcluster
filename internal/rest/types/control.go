package types

// Control represents the arguments that can be used to initialize/shutdown the daemon.
type Control struct {
	Bootstrap   bool   `json:"bootstrap" yaml:"bootstrap"`
	JoinAddress string `json:"join_address" yaml:"join_address"`
}
