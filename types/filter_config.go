package types

// FilterConfig structure that stores the information provided by the exclude/include flag
type FilterConfig struct {
	ExcludeAccess   *FilterAccessConfig   `yaml:"excludeAccess"`
	IncludeAccess   *FilterAccessConfig   `yaml:"includeAccess"`
	ExcludeInstance *FilterInstanceConfig `yaml:"excludeInstance"`
	IncludeInstance *FilterInstanceConfig `yaml:"includeInstance"`
}

// FilterAccessConfig filter properties for access items
type FilterAccessConfig struct {
	Aws struct {
		Names  []string `yaml:"names"`
		Owners []string `yaml:"owners"`
	} `yaml:"aws"`
	Azure struct {
		Names  []string `yaml:"names"`
		Owners []string `yaml:"owners"`
	} `yaml:"azure"`
	Gcp struct {
		Names  []string `yaml:"names"`
		Owners []string `yaml:"owners"`
	} `yaml:"gcp"`
}

// FilterInstanceConfig filter properties for instances
type FilterInstanceConfig struct {
	Aws struct {
		Labels []string `yaml:"labels"`
		Names  []string `yaml:"names"`
		Owners []string `yaml:"owners"`
	} `yaml:"aws"`
	Azure struct {
		Labels []string `yaml:"labels"`
		Names  []string `yaml:"names"`
		Owners []string `yaml:"owners"`
	} `yaml:"azure"`
	Gcp struct {
		Labels []string `yaml:"labels"`
		Names  []string `yaml:"names"`
		Owners []string `yaml:"owners"`
	} `yaml:"gcp"`
}
