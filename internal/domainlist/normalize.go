package domainlist

import (
	"strings"

	"github.com/rs/zerolog/log"
	"golang.org/x/net/idna"
)

var idnaProfile = idna.New(
	idna.MapForLookup(),
	idna.StrictDomainName(false), // Allow some non-strict domains
	idna.Transitional(false),     // IDNA2008 rules
)

// normalizeDomain converts a domain to canonical form:
//   - Lowercase
//   - Trim trailing "."
//   - Convert IDN (internationalized domain names) to punycode
//
// If IDNA conversion fails, logs a warning and returns the input as-is
// (lowercased and trimmed).
func normalizeDomain(domain string) string {
	// Lowercase.
	domain = strings.ToLower(domain)

	// Trim trailing dot (absolute domain notation).
	domain = strings.TrimSuffix(domain, ".")

	// Convert IDN to punycode using IDNA Lookup profile.
	// This handles domains with non-ASCII characters like "пример.рф" → "xn--e1afmkfd.xn--p1ai"
	punycode, err := idnaProfile.ToASCII(domain)
	if err != nil {
		// IDNA conversion failed - this can happen with invalid domains.
		// Log and return the input domain (already lowercased + trimmed).
		log.Debug().
			Err(err).
			Str("domain", domain).
			Msg("domainlist: IDNA conversion failed, using original")
		return domain
	}

	return punycode
}
