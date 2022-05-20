package cluster

// Code generation directives.
//
//go:generate -command mapper lxd-generate db mapper -t secrets.mapper.go
//go:generate mapper reset
//
//go:generate mapper stmt -e secret objects
//go:generate mapper stmt -e secret objects-by-JoinerCert
//go:generate mapper stmt -e secret id
//go:generate mapper stmt -e secret create
//go:generate mapper stmt -e secret delete-by-JoinerCert
//
//go:generate mapper method -e secret ID version=2
//go:generate mapper method -e secret Exists version=2
//go:generate mapper method -e secret GetOne version=2
//go:generate mapper method -e secret GetMany version=2
//go:generate mapper method -e secret Create version=2
//go:generate mapper method -e secret DeleteOne-by-JoinerCert version=2

type Secret struct {
	ID         int
	Token      string
	JoinerCert string `db:"primary=yes"`
}

type SecretFilter struct {
	ID         *int
	Token      *string
	JoinerCert *string
}
