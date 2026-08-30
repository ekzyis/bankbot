// Package sn wraps the Stacker News client, exposing the types and helpers
// ccbank needs behind a stable local facade.
package sn

import snappy "github.com/ekzyis/snappy"

// Type aliases so the rest of ccbank depends on this package rather than on
// snappy directly. Aliases (not new types) keep them identical to snappy's, so
// the concrete client still satisfies interfaces declared against these names.
type (
	Notification = snappy.Notification
	Item         = snappy.Item
	User         = snappy.User
	UserPrivates = snappy.UserPrivates
	Client       = snappy.Client
)

// NewClient builds a Stacker News client authenticated with the given nsec.
func NewClient(nsec, baseURL string) *snappy.Client {
	return snappy.NewClient(
		snappy.WithNsec(nsec),
		snappy.WithBaseUrl(baseURL),
	)
}

type meClient interface {
	Me() (*snappy.User, error)
}

// Balancer reports the treasury's current credit balance via the SN client.
type Balancer struct {
	Client meClient
}

func (b Balancer) Credits() (int, error) {
	me, err := b.Client.Me()
	if err != nil {
		return 0, err
	}
	return me.Privates.Credits, nil
}
