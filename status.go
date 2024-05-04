package radv

type Status struct {
	Interfaces []*InterfaceStatus `yaml:"interfaces" json:"interfaces"`
}

const (
	Running   = "Running"
	Reloading = "Reloading"
	Failing   = "Failing"
	Stopped   = "Stopped"
)

type InterfaceStatus struct {
	Name    string `yaml:"name" json:"name"`
	State   string `yaml:"state" json:"state"`
	Message string `yaml:"message,omitempty" json:"message,omitempty"`
}
