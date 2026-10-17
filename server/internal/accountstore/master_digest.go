package accountstore

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

func MasterDigest(cardTemplates, growthTables []byte) string {
	digest := sha256.New()
	for _, content := range [][]byte{cardTemplates, growthTables} {
		fmt.Fprintf(digest, "%d:", len(content))
		digest.Write(content)
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func (accounts *Accounts) ImportMatchingMaster(transfer AccountTransfer, masterDigest string) error {
	if transfer.MasterDigest != masterDigest {
		return errors.New("CN account master digest mismatch")
	}
	return accounts.ImportAccount(transfer)
}
