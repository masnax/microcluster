package cluster

//go:generate -command mapper lxd-generate db mapper -t certificates.mapper.go
//go:generate mapper reset
//
//go:generate mapper stmt -e certificate objects
//go:generate mapper stmt -e certificate objects-by-Fingerprint
//go:generate mapper stmt -e certificate id
//go:generate mapper stmt -e certificate create
//go:generate mapper stmt -e certificate delete-by-Fingerprint
//go:generate mapper stmt -e certificate delete-by-Name-and-Type
//go:generate mapper stmt -e certificate update
//
//go:generate mapper method -i -e certificate GetMany version=2
//go:generate mapper method -i -e certificate GetOne version=2
//go:generate mapper method -i -e certificate ID version=2
//go:generate mapper method -i -e certificate Exists version=2
//go:generate mapper method -i -e certificate Create version=2
//go:generate mapper method -i -e certificate DeleteOne-by-Fingerprint version=2
//go:generate mapper method -i -e certificate DeleteMany-by-Name-and-Type version=2
//go:generate mapper method -i -e certificate Update version=2

// Certificate is here to pass the certificates content from the database around.
type Certificate struct {
	ID          int
	Fingerprint string `db:"primary=yes"`
	Type        CertificateType
	Name        string
	Certificate string
}

// CertificateFilter specifies potential query parameter fields.
type CertificateFilter struct {
	Fingerprint *string
	Name        *string
	Type        *CertificateType
}

// CertificateType indicates the type of the certificate.
type CertificateType int

// CertificateTypeClient indicates a client certificate type.
const CertificateTypeClient = CertificateType(1)

// CertificateTypeServer indicates a server certificate type.
const CertificateTypeServer = CertificateType(2)
