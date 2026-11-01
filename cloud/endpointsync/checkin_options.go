package endpointsync

import (
	controltowerv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/controltower/v1"
	servicev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/services/controltower/v1"
)

// CheckInOption adds data to one check-in request.
type CheckInOption func(*checkInConfig)

type checkInConfig struct {
	tools  []*servicev1.ToolPolicyState
	labels []*controltowerv1.Label
}

// WithToolPolicy adds the cloud policy state of one tool to the check-in.
// Each call adds one state, so a daemon can report each tool on the
// endpoint. A nil state adds nothing.
func WithToolPolicy(state *servicev1.ToolPolicyState) CheckInOption {
	return func(c *checkInConfig) {
		if state != nil {
			c.tools = append(c.tools, state)
		}
	}
}

// WithLabels adds the "managed/" and "reported/" labels of the endpoint to
// the check-in, as ReportedLabels gives them.
func WithLabels(labels []*controltowerv1.Label) CheckInOption {
	return func(c *checkInConfig) {
		c.labels = append(c.labels, labels...)
	}
}
