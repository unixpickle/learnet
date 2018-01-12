package dnsproto

import (
	"errors"
	"strings"

	"github.com/unixpickle/essentials"
)

// A DomainName is a sequence of labels comprising a DNS
// domain name.
type DomainName []string

// ParseDomainName parses a DomainName from a
// period-delimited list of labels.
func ParseDomainName(name string) (domain DomainName, err error) {
	defer essentials.AddCtxTo("parse domain '"+name+"'", &err)

	// Trailing dot is valid, but we ignore it.
	if strings.HasSuffix(name, ".") {
		name = name[:len(name)-1]
	}

	parts := DomainName(strings.Split(name, "."))
	return parts, parts.Validate()
}

// String returns the domain's string representation.
func (d DomainName) String() string {
	return strings.Join(d, ".")
}

// Validate checks if the domain name is valid.
// If not, it returns an error.
func (d DomainName) Validate() error {
	for _, label := range d {
		if len(label) > 63 || len(label) == 0 {
			return errors.New("invalid label size in domain name")
		}
	}
	// TODO: check global length.
	// TODO: check character encoding.
	return nil
}
