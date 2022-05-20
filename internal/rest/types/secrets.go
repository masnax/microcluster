package types

import (
	"encoding/base64"
	"encoding/json"
)

type Secret struct {
	JoinerCert string `json:"joiner_cert" yaml:"joiner_cert"`
	Token      string `json:"token" yaml:"token"`
}

type SecretResponse struct {
	ClusterCert X509Certificate `json:"cluster_cert" yaml:"cluster_cert"`
	ClusterKey  string          `json:"cluster_key" yaml:"cluster_key"`
	Peers       []Peer          `json:"peers" yaml:"peers"`
}

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
