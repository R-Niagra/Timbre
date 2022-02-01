package pos

// Config contains the configuations for our PoS scheme
type Config struct {
	s int // the number of elements in each data block
	N int // each element in the data block is N bytes
}

// MakeDefaultConfig returns the default configuration where each block is 16KB
func MakeDefaultConfig() *Config {
	return &Config{
		s: 16,
		N: 1024,
	}
}

// MakeTestConfig returns configuration used for testing, where each block is 32 bytes
func MakeTestConfig() *Config {
	return &Config{
		s: 4,
		N: 8,
	}
}
