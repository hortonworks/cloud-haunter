package types

// Ignores structure that stores the information provided by the ignores flag
type Ignores struct {
	Access struct {
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
	} `yaml:"access"`
	Instance struct {
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
	} `yaml:"instance"`
}
