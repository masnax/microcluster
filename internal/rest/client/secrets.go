package client

import (
	"context"
	"time"

	"github.com/canonical/microcluster/internal/rest/types"
	"github.com/lxc/lxd/shared/api"
)

// RequestToken requests a join token with the given certificate fingerprint.
func (c *Client) RequestToken(ctx context.Context, fingerprint string) (string, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var token string
	secret := types.Secret{JoinerCert: fingerprint}
	err := c.QueryStruct(queryCtx, "POST", ControlEndpoint, api.NewURL().Path("secrets"), secret, &token)

	return token, err
}

// SubmitToken authenticates a token and returns information necessary to join the cluster.
func (c *Client) SubmitToken(ctx context.Context, fingerprint string, token string) (types.SecretResponse, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var secret types.SecretResponse
	secretPost := types.Secret{Token: token}
	err := c.QueryStruct(queryCtx, "POST", InternalEndpoint, api.NewURL().Path("secrets", fingerprint), secretPost, &secret)

	return secret, err
}

// DeleteSecret deletes the secret record.
func (c *Client) DeleteSecret(ctx context.Context, fingerprint string) error {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := c.QueryStruct(queryCtx, "DELETE", ControlEndpoint, api.NewURL().Path("secrets", fingerprint), nil, nil)

	return err
}

// GetSecrets returns the secret records.
func (c *Client) GetSecrets(ctx context.Context) ([]types.Secret, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	secrets := []types.Secret{}
	err := c.QueryStruct(queryCtx, "GET", ControlEndpoint, api.NewURL().Path("secrets"), nil, &secrets)

	return secrets, err
}
