package accountsync

import (
	"bytes"
	"testing"
)

func TestSetMasterDigestKeepsCredential(t *testing.T) {
	raw := []byte(`{"UserID":1000001,"Revision":2,"Metadata":"YQ==","MetadataDigest":"abc","MasterDigest":"","Account":{},"Credential":{"Username":"player_one","Salt":"AQEBAQEBAQEBAQEBAQEBAA==","Hash":"AgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgI=","Iterations":600000}}`)
	encoded, revision, err := setMasterDigest(raw, "masters")
	if err != nil {
		t.Fatal(err)
	}
	if revision != 2 || !bytes.Contains(encoded, []byte(`"Username":"player_one"`)) {
		t.Fatalf("credential dropped from transfer: %s", encoded)
	}
}
