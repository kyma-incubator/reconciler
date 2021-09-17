package jwks

import (
	"fmt"
	"testing"

	jose "github.com/square/go-jose/v3"
	"github.com/stretchr/testify/require"
)

func TestGenerateJwks(t *testing.T) {
	for _, alg := range signingKeysAvailableAlgorithms() {
		alg := alg
		t.Run(fmt.Sprintf("alg=%s", alg), func(t *testing.T) {
			j := new(alg, 0)
			data, err := j.generateJwksSecret()
			require.NoError(t, err)
			t.Logf("%+v", data)
		})
	}
}

func signingKeysAvailableAlgorithms() []string {
	return []string{
		string(jose.RS256), string(jose.RS384), string(jose.RS512), string(jose.PS256), string(jose.PS384), string(jose.PS512),
	}
}
