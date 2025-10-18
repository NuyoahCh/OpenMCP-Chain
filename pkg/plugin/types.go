package plugin

// Type represents the functional category of a plugin.
type Type string

const (
	// TypeDataSource plugins are responsible for providing raw data to the system.
	TypeDataSource Type = "datasource"
	// TypeProcessor plugins transform, enrich or validate data.
	TypeProcessor Type = "processor"
)

// Capability expresses optional features a plugin may request access to.
type Capability string

const (
	CapabilityFilesystem Capability = "filesystem"
	CapabilityNetwork    Capability = "network"
	CapabilityExecution  Capability = "execution"
)

// Info contains descriptive metadata for a plugin implementation.
type Info struct {
	ID           string
	Name         string
	Description  string
	Author       string
	Version      string
	Category     Type
	Capabilities []Capability
}

// State represents the lifecycle position of a plugin instance.
type State string

const (
	StateRegistered  State = "registered"
	StateInitialised State = "initialised"
	StateStarted     State = "started"
	StateStopped     State = "stopped"
)
