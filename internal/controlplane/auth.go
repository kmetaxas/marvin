package controlplane

import "fmt"

type Authenticator interface {
	Method() string
	RegistrationKey() (string, error)
}

type SecretKeyAuthenticator struct {
	Key string
}

func (a SecretKeyAuthenticator) Method() string {
	return "secret_key"
}

func (a SecretKeyAuthenticator) RegistrationKey() (string, error) {
	if a.Key == "" {
		return "", fmt.Errorf("secret key is empty")
	}

	return a.Key, nil
}
