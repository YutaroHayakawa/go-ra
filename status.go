package ra

// Status is the status of the Daemon
type Status struct {
	// Interfaces-specific status
	Interfaces []*InterfaceStatus `yaml:"interfaces" json:"interfaces"`
}

// Possible interface status
const (
	// Running means the router advertisement is running
	Running = "Running"
	// Reloading means the router advertisement is reloading the configuration
	Reloading = "Reloading"
	// Failing means the router advertisement is failing with an error
	Failing = "Failing"
	// Stopped means the router advertisement is stopped
	Stopped = "Stopped"
)

// InterfaceStatus represents the interface-specific status of the Daemon
type InterfaceStatus struct {
	// Interface name
	Name string `yaml:"name" json:"name"`

	// Status of the router advertisement on the interface
	State string `yaml:"state" json:"state"`

	// Error message maybe set when the state is Failing or Stopped
	Message string `yaml:"message,omitempty" json:"message,omitempty"`

	// Last configuration update time in Unix time
	LastUpdate int64 `yaml:"lastUpdate" json:"lastUpdate"`

	// Number of sent solicited router advertisements
	TxSolicitedRA int `yaml:"txSolicitedRA" json:"txSolicitedRA"`

	// Number of sent unsolicited router advertisements
	TxUnsolicitedRA int `yaml:"txUnsolicitedRA" json:"txUnsolicitedRA"`
}
