package connections

import "github.com/zalando/go-keyring"

const credentialService = "BajiuDB.connections"

type Secrets interface {
	Get(string) (string, error)
	Set(string, string) error
	Delete(string) error
}

type SystemSecrets struct{}

func (SystemSecrets) Get(id string) (string, error) { return keyring.Get(credentialService, id) }
func (SystemSecrets) Set(id, value string) error    { return keyring.Set(credentialService, id, value) }
func (SystemSecrets) Delete(id string) error {
	err := keyring.Delete(credentialService, id)
	if err == keyring.ErrNotFound {
		return nil
	}
	return err
}
