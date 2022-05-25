package types

type Cluster struct {
	Name        string          `json:"name" yaml:"name"`
	Addresses   AddrPorts       `json:"addresses" yaml:"addresses"`
	Certificate X509Certificate `json:"certificate" yaml:"certificate"`
}
