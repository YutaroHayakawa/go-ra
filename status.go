package radv

type Status struct {
	Interfaces []*InterfaceStatus `yaml:"interfaces" json:"interfaces"`
}

type State string

const (
	Unknown   State = "Unknown"
	Running   State = "Running"
	Reloading State = "Reloading"
	Failing   State = "Failing"
	Stopped   State = "Stopped"
)

type InterfaceStatus struct {
	Name    string `yaml:"name" json:"name"`
	State   State  `yaml:"state" json:"state"`
	Message string `yaml:"message,omitempty" json:"message,omitempty"`
}

func (s *InterfaceStatus) running() {
	s.State = Running
	s.Message = ""
}

func (s *InterfaceStatus) reloading() {
	s.State = Reloading
	s.Message = ""
}

func (s *InterfaceStatus) failing(err error) {
	s.State = Failing
	if err == nil {
		s.Message = ""
	} else {
		s.Message = err.Error()
	}
}

func (s *InterfaceStatus) stopped(err error) {
	s.State = Stopped
	if err == nil {
		s.Message = ""
	} else {
		s.Message = err.Error()
	}
}
