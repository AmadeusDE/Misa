package auth

// layeredBackend reads credentials from primary, filling gaps with api keys from
// fallback; it only ever writes primary. OAuth credentials are never borrowed:
// refresh tokens rotate, and two stores holding copies would invalidate each other.
type layeredBackend struct {
	primary  Backend
	fallback Backend
}

func (l layeredBackend) Name() string { return l.primary.Name() + "+" + l.fallback.Name() }

func (l layeredBackend) Load() (*Credentials, error) {
	c, err := l.primary.Load()
	if err != nil {
		return nil, err
	}
	fb, err := l.fallback.Load()
	if err != nil || fb == nil {
		return c, nil
	}
	for name, cred := range fb.Providers {
		if _, own := c.Providers[name]; own || cred.Type != "api_key" {
			continue
		}
		if c.Providers == nil {
			c.Providers = map[string]Credential{}
		}
		c.Providers[name] = cred
	}
	return c, nil
}

func (l layeredBackend) Save(c *Credentials) error { return l.primary.Save(c) }
