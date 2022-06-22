package types

import (
	"encoding/base64"
	"encoding/json"
)

// Secret holds information for requesting a join token.
type Secret struct {
	JoinerCert string `json:"joiner_cert" yaml:"joiner_cert"`
	Token      string `json:"token" yaml:"token"`
}

// SecretResponse holds the information for connecting to a cluster by a node with a valid join token.
type SecretResponse struct {
	ClusterCert    X509Certificate `json:"cluster_cert" yaml:"cluster_cert"`
	ClusterKey     string          `json:"cluster_key" yaml:"cluster_key"`
	ClusterMembers []ClusterMember `json:"cluster_members" yaml:"cluster_members"`
}

// Token holds the information that is presented to the joining node when requesting a token.
type Token struct {
	Token       string          `json:"token" yaml:"token"`
	ClusterCert X509Certificate `json:"cluster_cert" yaml:"cluster_cert"`
	JoinAddress AddrPort        `json:"join_address" yaml:"join_address"`
}

func (t Token) String() (string, error) {
	tokenData, err := json.Marshal(t)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(tokenData), nil
}

// DecodeToken decodes a base64-encoded token string.
func DecodeToken(tokenString string) (*Token, error) {
	tokenData, err := base64.StdEncoding.DecodeString(tokenString)
	if err != nil {
		return nil, err
	}

	var token Token
	err = json.Unmarshal(tokenData, &token)
	if err != nil {
		return nil, err
	}

	return &token, nil
}
