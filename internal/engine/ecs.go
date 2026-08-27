package engine

// Entity is defined in core package to avoid cyclic dependency

// Components are handled in Store

// SystemDomain classifies the state domains a system reads and writes.
type SystemDomain uint8

const (
	SystemShared SystemDomain = iota // Reads and writes shared state only.
	SystemPlayer                     // Reads shared state and writes player state only.
	SystemDual                       // Resolves both domains under an explicit boundary rule.
)

// String returns the stable configuration name for a system domain.
func (d SystemDomain) String() string {
	switch d {
	case SystemShared:
		return "shared"
	case SystemPlayer:
		return "player"
	case SystemDual:
		return "dual"
	default:
		return "unknown"
	}
}

// SystemDependencies separates hard requirements from graceful integrations.
type SystemDependencies struct {
	Required []string
	Optional []string
}

// System is implemented by every game system.
type System interface {
	Init()
	Priority() int // Lower values run first
	Name() string
	Domain() SystemDomain
	Dependencies() SystemDependencies
	Update()
}
