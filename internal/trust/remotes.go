package trust

import (
	"crypto/x509"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"github.com/lxc/lxd/shared"
	"github.com/lxc/lxd/shared/api"
	"gopkg.in/yaml.v2"

	"github.com/canonical/microcluster/internal/rest/types"
)

// Remotes is a convenient alias as we will often deal with groups of yaml files.
type Remotes map[string]Remote

// Remote represents a yaml file with credentials to be read by the daemon.
type Remote struct {
	Name        string                `yaml:"name"`
	Addresses   types.AddrPorts       `yaml:"addresses"`
	Certificate types.X509Certificate `yaml:"certificate"`
}

// Load reads any yaml files in the given directory and parses them into a set of Remotes.
func Load(dir string) (Remotes, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("Unable to read trust directory: %q: %w", dir, err)
	}

	remotes := Remotes{}
	for _, file := range files {
		fileName := file.Name()
		if file.IsDir() || !strings.HasSuffix(fileName, ".yaml") {
			continue
		}

		content, err := os.ReadFile(filepath.Join(dir, fileName))
		if err != nil {
			return nil, fmt.Errorf("Unable to read file %q: %w", fileName, err)
		}

		remote := &Remote{}
		err = yaml.Unmarshal(content, remote)
		if err != nil {
			return nil, fmt.Errorf("Unable to parse yaml for %q: %w", fileName, err)
		}

		remotes[remote.Name] = *remote
	}

	return remotes, nil
}

// SelectRandom returns a random remote.
func (r Remotes) SelectRandom() *Remote {
	allRemotes := make([]Remote, 0, len(r))
	for _, r := range r {
		allRemotes = append(allRemotes, r)
	}

	return &allRemotes[rand.Intn(len(allRemotes))]
}

// Addresses returns just the host:port addresses of the remotes.
func (r Remotes) Addresses() map[string]types.AddrPorts {
	addrs := map[string]types.AddrPorts{}
	for _, remote := range r {
		remoteAddrs := make(types.AddrPorts, 0, len(remote.Addresses))
		for _, addr := range remote.Addresses {
			remoteAddrs = append(remoteAddrs, addr)
		}

		addrs[remote.Name] = remoteAddrs
	}

	return addrs
}

// RemoteByAddress returns a Remote matching the given host address (or nil if none are found).
func (r Remotes) RemoteByAddress(addrPort types.AddrPort) *Remote {
	for _, remote := range r {
		for _, remoteAddr := range remote.Addresses {
			if remoteAddr.String() == addrPort.String() {
				return &remote
			}
		}
	}

	return nil
}

// RemoteByCertificateFingerprint returns a remote whose certificate fingerprint matches the provided fingerprint.
func (r Remotes) RemoteByCertificateFingerprint(fingerprint string) *Remote {
	for _, remote := range r {
		if fingerprint == shared.CertFingerprint(remote.Certificate.Certificate) {
			return &remote
		}
	}

	return nil
}

// Certificates returns a map of remotes certificates by fingerprint.
func (r Remotes) Certificates() map[string]types.X509Certificate {
	certMap := map[string]types.X509Certificate{}
	for _, remote := range r {
		certMap[shared.CertFingerprint(remote.Certificate.Certificate)] = remote.Certificate
	}

	return certMap
}

// CertificatesNative returns the Certificates map with values as native x509.Certificate type.
func (r Remotes) CertificatesNative() map[string]x509.Certificate {
	certMap := map[string]x509.Certificate{}
	for k, v := range r.Certificates() {
		certMap[k] = *v.Certificate
	}

	return certMap
}

// URLs returns the parsed URLs of the Remote.
func (r Remote) URLs() []api.URL {
	hosts := make([]api.URL, 0, len(r.Addresses))

	for _, addr := range r.Addresses {
		url := api.NewURL().Scheme("https").Host(addr.String())
		hosts = append(hosts, *url)
	}

	return hosts
}

// RandomURL returns a randomly selected URL from the addresses of the remote.
func (r Remote) RandomURL() api.URL {
	return *api.NewURL().Scheme("https").Host(r.Addresses.SelectRandom().String())
}
