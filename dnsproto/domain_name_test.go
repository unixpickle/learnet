package dnsproto

import (
	"testing"
)

func TestValidDomains(t *testing.T) {
	domains := []string{"a.b", "a9.b", "a.b-9", "A.B-9.Gh", "AbcD.E-fF.g"}
	for _, domain := range domains {
		parsed, err := ParseDomainName(domain)
		if err != nil {
			t.Error(err)
		} else if err := parsed.Validate(); err != nil {
			t.Error(err)
		}
	}
}

func TestInvalidDomains(t *testing.T) {
	domains := []string{"a.b-", "a-.9", "9.b-9", "9ab.B-9.Gh", "Ab_D.E-fF.g"}
	for _, domain := range domains {
		if _, err := ParseDomainName(domain); err == nil {
			t.Error("expected error for: " + domain)
		}
	}
}
