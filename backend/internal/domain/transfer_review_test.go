package domain

import "testing"

func TestTransferDecision_AppliedCategory(t *testing.T) {
	cases := []struct {
		d      TransferDecision
		chosen string
		want   string
	}{
		{TransferOwn, "", ""},
		{TransferPerson, "", "PEOPLE"},
		{TransferPayment, "HOUSING", "HOUSING"},
	}
	for _, c := range cases {
		if got := c.d.AppliedCategory(c.chosen); got != c.want {
			t.Errorf("%s(%q) = %q, esperado %q", c.d, c.chosen, got, c.want)
		}
	}
}
