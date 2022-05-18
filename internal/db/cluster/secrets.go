package cluster

// Code generation directives.
//
//go:generate -command mapper lxd-generate db mapper -t secrets.mapper.go
//go:generate mapper reset
//
//go:generate mapper stmt -e secret objects
//go:generate mapper stmt -e secret objects-by-Name
//go:generate mapper stmt -e secret objects-by-Certificate
//go:generate mapper stmt -e secret objects-by-Certificate-and-Name
//go:generate mapper stmt -e secret id
//go:generate mapper stmt -e secret create
//go:generate mapper stmt -e secret delete-by-Name
//
//go:generate mapper method -e secret GetMany version=2
//go:generate mapper method -e secret ID version=2
//go:generate mapper method -e secret Exists version=2
//go:generate mapper method -e secret Create version=2
//go:generate mapper method -e secret DeleteOne-by-Name version=2

type Secret struct {
	ID          int
	Certificate string `db:"join=certificates.fingerprint"`
	Token       string
	Name        string
}

type SecretFilter struct {
	ID          *int
	Certificate *string
	Token       *string
	Name        *string
}
