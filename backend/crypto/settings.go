package crypto

// EncryptSetting encrypts a string-valued secret setting (e.g. payment API key)
// with AES-256-GCM keyed by secretKey. Output is base64(nonce || ciphertext),
// the same shape used for signing_keys.encrypted_private_key.
func EncryptSetting(secretKey [32]byte, plaintext string) (string, error) {
	return EncryptPrivateKey(secretKey, []byte(plaintext))
}

// DecryptSetting reverses EncryptSetting and returns the plaintext secret.
func DecryptSetting(secretKey [32]byte, ciphertext string) (string, error) {
	b, err := DecryptPrivateKey(secretKey, ciphertext)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
