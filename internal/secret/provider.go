package secret

// Provider abstracts secret loading from an encrypted store.
type Provider interface {
	Load(path string) (map[string]string, error)
}
