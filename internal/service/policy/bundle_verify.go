package policy

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
)

func (s *Service) verifyBundle(ctx context.Context, files []ZipFile) error {
	policyBundleFile := files[0]
	signatureFile := files[1]

	if policyBundleFile.Name != BundleFilename {
		return fmt.Errorf("verify bundle: invalid bundle filename: %q", files[0].Name)
	}

	if signatureFile.Name != BundleSignatureFilename {
		return fmt.Errorf("verify bundle: invalid signature filename: %q", files[1].Name)
	}

	bundleFiles, err := s.unzip(policyBundleFile.Content)
	if err != nil {
		return err
	}

	if len(bundleFiles) == 0 || bundleFiles[0].Name != "metadata.json" {
		return fmt.Errorf("invalid bundle")
	}

	var metadata Metadata
	if err := json.Unmarshal(bundleFiles[0].Content, &metadata); err != nil {
		return fmt.Errorf("failed to unmarshal metadata: %v", err)
	}

	// whitelist is insecure to allow fetching keys from arbitrary external locations
	// TODO: this can be fine-tuned with configuration variable so that organizations
	// can specify trusted import locations.
	keyset, err := jwk.Fetch(ctx,
		metadata.PublicKeyURL,
		jwk.WithHTTPClient(s.httpClient),
		jwk.WithFetchWhitelist(jwk.InsecureWhitelist{}),
	)
	if err != nil {
		return fmt.Errorf("verify bundle: %v", err)
	}

	// we expect to receive a single verification key
	verKey, ok := keyset.Key(0)
	if !ok {
		return fmt.Errorf("cannot get bundle verification key")
	}

	// the payload that is signed on policy export is the sha256 hash of the
	// policy bundle zip file itself, so this is the payload that should be verified
	payload := sha256.Sum256(policyBundleFile.Content)

	switch kt := verKey.KeyType(); kt {
	case jwa.EC:
		err = s.verifyECDSA(payload[:], signatureFile.Content, verKey)
	case jwa.OKP:
		err = s.verifyED25519(payload[:], signatureFile.Content, verKey)
	case jwa.RSA:
		err = s.verifyRSA(payload[:], signatureFile.Content, verKey)
	default:
		return fmt.Errorf("unsupported public key type: %v", kt)
	}

	return err
}

func (s *Service) verifyECDSA(payload []byte, signature []byte, key jwk.Key) error {
	// convert key from JWK to ecdsa.PublicKey
	var ecdsaKey ecdsa.PublicKey
	if err := key.Raw(&ecdsaKey); err != nil {
		return err
	}

	// hash function is always sha-256 by default when Hashicorp Vault signs
	// data with ECDSA keys and no specific/different hash function is selected
	// by the client.
	hash := sha256.Sum256(payload)

	// ECDSA signatures returned by Hashicorp Vault are ASN encoded
	valid := ecdsa.VerifyASN1(&ecdsaKey, hash[:], signature)
	if !valid {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

func (s *Service) verifyED25519(payload []byte, signature []byte, key jwk.Key) error {
	// convert key from JWK to ed25519.PublicKey
	var ed25519Key ed25519.PublicKey
	if err := key.Raw(&ed25519Key); err != nil {
		return err
	}

	// for ed25519 signatures we must not specifically hash the payload
	// as this signature algorithm is using its own hash function internally
	valid := ed25519.Verify(ed25519Key, payload, signature)
	if !valid {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

func (s *Service) verifyRSA(payload []byte, signature []byte, key jwk.Key) error {
	// convert key from JWK to rsa.PublicKey
	var rsaKey rsa.PublicKey
	if err := key.Raw(&rsaKey); err != nil {
		return err
	}

	// hash function is always sha-256 by default when Hashicorp Vault signs
	// data with RSA keys and no specific/different hash function is selected
	// by the client.
	hash := sha256.Sum256(payload)

	err := rsa.VerifyPSS(&rsaKey, crypto.SHA256, hash[:], signature, nil)
	if err != nil {
		return fmt.Errorf("invalid signature: %v", err)

	}

	return nil
}
