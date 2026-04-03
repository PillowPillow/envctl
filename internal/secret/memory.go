package secret

// MemoryProvider is an in-memory secret provider for testing.
type MemoryProvider struct {
	Secrets map[string]string
}

func (p *MemoryProvider) Load(path string) (map[string]string, error) {
	result := make(map[string]string, len(p.Secrets))
	for k, v := range p.Secrets {
		result[k] = v
	}
	return result, nil
}
