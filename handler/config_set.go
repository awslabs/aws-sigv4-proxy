package handler

// ConfigSet contains overrides for individual hosts
type ConfigSet struct {
	Name string    `yaml:"name"`
	Region string  `yaml:"region"`
	Host string    `yaml:"host"`
	RoleArn string `yaml:"role-arn"`
}

