package stream

// Declaration of public streams
// These are EXPERIMENTAL. SafeDep do not yet offer any strong contract for
// these streams. Use functions for immutable declaration

// OpenSourcePackageMonitorStream defines a stream for monitoring open source package versions
func OpenSourcePackageMonitorStream() Stream {
	return Stream{
		Namespace: "malysis",
		Name:      "monitor-package-versions",

		Meta: StreamMeta{
			Public:      true,
			Description: "Near-realtime stream of open source packages published to public package registries",
		},
	}
}
