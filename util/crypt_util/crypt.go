package crypt_util

import (
	"collector-agent/util"
	"encoding/pem"
	"log"
	"os"
	"sync"

	"github.com/farmerx/gorsa"
)

var once sync.Once

var internalCryptUtil *CryptUtil
var rootDir string

func init() {
	rootDir = util.GetRootDir()
}

type CryptUtil struct {
}

func New() *CryptUtil {
	once.Do(func() {
		internalCryptUtil = &CryptUtil{}

		publicKeyPEM, err := os.ReadFile(rootDir + "/storage/keys/publicKey.pem")
		if err != nil {
			log.Fatal("Private key not found")
		}

		block, _ := pem.Decode(publicKeyPEM)
		if block == nil || block.Type != "PUBLIC KEY" {
			log.Fatal("Failed to decode public key")
		}

		if err := gorsa.RSA.SetPublicKey(string(publicKeyPEM)); err != nil {
			log.Fatalln(`set public key :`, err)
		}
	})

	return internalCryptUtil
}

func (cu *CryptUtil) EncryptViaPub(input []byte) ([]byte, error) {
	return gorsa.RSA.PubKeyENCTYPT(input)
}

func (cu *CryptUtil) DecryptViaPub(input []byte) ([]byte, error) {
	return gorsa.RSA.PubKeyDECRYPT(input)
}

func (cu *CryptUtil) EncryptViaPrivate(input []byte) ([]byte, error) {
	return gorsa.RSA.PriKeyENCTYPT(input)
}

func (cu *CryptUtil) DecryptViaPrivate(input []byte) ([]byte, error) {
	return gorsa.RSA.PriKeyDECRYPT(input)
}
