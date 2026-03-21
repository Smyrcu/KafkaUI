package auth

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"strings"

	"github.com/go-ldap/ldap/v3"

	"github.com/Smyrcu/KafkaUI/internal/config"
)

// LDAPAuthenticator verifies credentials against an LDAP directory.
type LDAPAuthenticator struct {
	cfg    config.LDAPConfig
	logger *slog.Logger
}

func NewLDAPAuthenticator(cfg config.LDAPConfig, logger *slog.Logger) *LDAPAuthenticator {
	return &LDAPAuthenticator{cfg: cfg, logger: logger}
}

// Authenticate verifies username/password against LDAP.
// Returns a UserIdentity on success with groups in the Orgs field.
func (a *LDAPAuthenticator) Authenticate(username, password string) (*UserIdentity, error) {
	timeout := a.cfg.ConnectionTimeoutDuration()

	conn, err := ldap.DialURL(a.cfg.URL, ldap.DialWithDialer(&net.Dialer{Timeout: timeout}))
	if err != nil {
		a.logger.Error("LDAP dial failed", "url", a.cfg.URL, "error", err)
		return nil, fmt.Errorf("invalid credentials")
	}
	defer conn.Close()

	if a.cfg.StartTLS {
		if err := conn.StartTLS(&tls.Config{InsecureSkipVerify: false}); err != nil {
			a.logger.Error("LDAP StartTLS failed", "error", err)
			return nil, fmt.Errorf("invalid credentials")
		}
	}

	// Service account bind
	if err := conn.Bind(a.cfg.BindDN, a.cfg.BindPassword); err != nil {
		a.logger.Error("LDAP service bind failed", "bind-dn", a.cfg.BindDN, "error", err)
		return nil, fmt.Errorf("invalid credentials")
	}

	// Search for user
	searchFilter := a.cfg.SearchFilter
	if searchFilter == "" {
		searchFilter = "(&(objectClass=person)(uid={username}))"
	}
	searchFilter = strings.ReplaceAll(searchFilter, "{username}", ldap.EscapeFilter(username))

	emailAttr := a.cfg.EmailAttribute
	if emailAttr == "" {
		emailAttr = "mail"
	}
	nameAttr := a.cfg.NameAttribute
	if nameAttr == "" {
		nameAttr = "cn"
	}
	groupAttr := a.cfg.GroupAttribute
	if groupAttr == "" {
		groupAttr = "memberOf"
	}

	result, err := conn.Search(ldap.NewSearchRequest(
		a.cfg.SearchBase,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 1, int(timeout.Seconds()), false,
		searchFilter,
		[]string{"dn", emailAttr, nameAttr, groupAttr},
		nil,
	))
	if err != nil || len(result.Entries) == 0 {
		// Timing normalization: perform a dummy bind to prevent user enumeration
		_ = conn.Bind("cn=timing-normalization,dc=invalid", password)
		return nil, fmt.Errorf("invalid credentials")
	}

	userEntry := result.Entries[0]
	userDN := userEntry.DN

	// Verify password via user bind
	if err := conn.Bind(userDN, password); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	email := userEntry.GetAttributeValue(emailAttr)
	name := userEntry.GetAttributeValue(nameAttr)

	// Re-bind as service account for group search (user bind may lack search permissions)
	if err := conn.Bind(a.cfg.BindDN, a.cfg.BindPassword); err != nil {
		a.logger.Error("LDAP service re-bind failed for group search", "error", err)
	}

	// Extract groups
	var groups []string
	if a.cfg.GroupSearchBase != "" {
		// Active group search mode
		groups = a.searchGroups(conn, userDN)
	} else {
		// memberOf attribute mode
		groups = userEntry.GetAttributeValues(groupAttr)
	}

	return &UserIdentity{
		ProviderName: "ldap",
		ProviderType: "ldap",
		ExternalID:   userDN,
		Email:        email,
		Name:         name,
		Orgs:         groups,
	}, nil
}

func (a *LDAPAuthenticator) searchGroups(conn *ldap.Conn, userDN string) []string {
	filter := a.cfg.GroupSearchFilter
	if filter == "" {
		filter = "(&(objectClass=groupOfNames)(member={dn}))"
	}
	// Escape LDAP filter special characters (RFC 4515: *, (, ), \, NUL)
	// but NOT DN structural characters (= and ,) which are valid in member DN values.
	// ldap.EscapeFilter is too aggressive for DN values, so we do selective escaping.
	escapedDN := userDN
	escapedDN = strings.ReplaceAll(escapedDN, `\`, `\5c`)
	escapedDN = strings.ReplaceAll(escapedDN, `*`, `\2a`)
	escapedDN = strings.ReplaceAll(escapedDN, `(`, `\28`)
	escapedDN = strings.ReplaceAll(escapedDN, `)`, `\29`)
	escapedDN = strings.ReplaceAll(escapedDN, "\x00", `\00`)
	filter = strings.ReplaceAll(filter, "{dn}", escapedDN)

	result, err := conn.Search(ldap.NewSearchRequest(
		a.cfg.GroupSearchBase,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 10, false,
		filter, []string{"dn"}, nil,
	))
	if err != nil {
		a.logger.Warn("LDAP group search failed", "error", err)
		return nil
	}

	groups := make([]string, 0, len(result.Entries))
	for _, entry := range result.Entries {
		groups = append(groups, entry.DN)
	}
	return groups
}
