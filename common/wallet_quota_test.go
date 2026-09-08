package common

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCanAddWalletQuota(t *testing.T) {
	for _, tc := range []struct {
		name    string
		balance int
		credit  int64
		want    bool
	}{
		{"above single charge limit", MaxQuota, 10_000_000, true},
		{"exact wallet limit", MaxWalletQuota - 100, 100, true},
		{"over wallet limit", MaxWalletQuota - 100, 101, false},
		{"hostile credit", 100, math.MaxInt64, false},
		{"negative credit", 100, -1, false},
		{"negative balance", -100, 200, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, CanAddWalletQuota(tc.balance, tc.credit))
		})
	}
}
