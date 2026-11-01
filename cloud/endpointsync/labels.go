package endpointsync

import (
	"runtime"

	controltowerv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/controltower/v1"
)

const (
	// LabelReportedOS is the label key of the operating system of the
	// endpoint, with a runtime.GOOS value such as "darwin".
	LabelReportedOS = "reported/os"

	// LabelReportedArch is the label key of the CPU architecture of the
	// endpoint, with a runtime.GOARCH value such as "arm64".
	LabelReportedArch = "reported/arch"
)

// ReportedLabels returns the "reported/" labels that a tool sends in the
// policy fetch and the check-in. A policy can select endpoints by these
// labels, so every tool must send one spelling: the values of runtime.GOOS
// and runtime.GOARCH.
func ReportedLabels() []*controltowerv1.Label {
	return reportedLabels(runtime.GOOS, runtime.GOARCH)
}

func reportedLabels(goos, goarch string) []*controltowerv1.Label {
	return []*controltowerv1.Label{
		controltowerv1.Label_builder{Key: LabelReportedOS, Value: goos}.Build(),
		controltowerv1.Label_builder{Key: LabelReportedArch, Value: goarch}.Build(),
	}
}
