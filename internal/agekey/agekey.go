package agekey

import (
	"errors"
	"regexp"
	"strings"
)

// The second form covers recipients with a longer prefix, such as age1pq1 (post-quantum) or age1yubikey1 (plugins).
var recipientRe = regexp.MustCompile(`^age1(?:[02-9ac-hj-np-z]{58}|[a-z0-9+._-]+1[02-9ac-hj-np-z]{6,})$`)

const charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

var errInvalid = errors.New("invalid age public key: expected an age1... bech32 recipient")

// ValidateRecipient checks the shape and the bech32 checksum; age recipients have no length limit, unlike BIP 173.
func ValidateRecipient(key string) error {
	if !recipientRe.MatchString(key) {
		return errInvalid
	}
	sep := strings.LastIndexByte(key, '1')
	hrp, data := key[:sep], key[sep+1:]
	values := make([]byte, len(data))
	for i := range len(data) {
		values[i] = byte(strings.IndexByte(charset, data[i])) //nolint:gosec // the regex keeps every data char in charset, so 0-31
	}
	if polymod(append(expandHRP(hrp), values...)) != 1 {
		return errors.New("invalid age public key: bad checksum (the key may be truncated or mistyped)")
	}
	return nil
}

func expandHRP(hrp string) []byte {
	out := make([]byte, 0, 2*len(hrp)+1)
	for i := range len(hrp) {
		out = append(out, hrp[i]>>5)
	}
	out = append(out, 0)
	for i := range len(hrp) {
		out = append(out, hrp[i]&31)
	}
	return out
}

func polymod(values []byte) uint32 {
	gen := [5]uint32{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}
	chk := uint32(1)
	for _, v := range values {
		top := chk >> 25
		chk = (chk&0x1ffffff)<<5 ^ uint32(v)
		for i := range 5 {
			if (top>>i)&1 == 1 {
				chk ^= gen[i]
			}
		}
	}
	return chk
}
