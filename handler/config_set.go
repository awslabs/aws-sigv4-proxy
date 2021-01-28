package handler

// ConfigSet contains overrides for individual hosts
type ConfigSet struct {
	Name string
	Region string
	Host string
	RoleArn string `yaml:"role-arn"`
}

