package cvss

import (
	"fmt"

	v2_metric "github.com/goark/go-cvss/v2/metric"
	v3_metric "github.com/goark/go-cvss/v3/metric"
)

type CvssVersion string

// As per OSV schema
// https://ossf.github.io/osv-schema/#severitytype-field
const (
	CVSS_V2 CvssVersion = "CVSS_V2"
	CVSS_V3 CvssVersion = "CVSS_V3"
	CVSS_V4 CvssVersion = "CVSS_V4"
)

type CvssRisk string

// Qualitative Severity Ratings
// https://nvd.nist.gov/vuln-metrics/cvss
const (
	// Introduced in v3
	CRITICAL CvssRisk = "CRITICAL"

	// Present in both v3 and v2
	HIGH   CvssRisk = "HIGH"
	MEDIUM CvssRisk = "MEDIUM"
	LOW    CvssRisk = "LOW"
	NONE   CvssRisk = "NONE"
)

// This is the API. Everything else should be hidden
// within the package
type CVSS interface {
	Severity() CvssRisk
}

// Implementation for V2
type cvssV2 struct {
	base *v2_metric.Base
}

func newBaseCvssV2(base string) (CVSS, error) {
	bm, err := v2_metric.NewBase().Decode(base)
	if err != nil {
		return nil, err
	}

	return &cvssV2{
		base: bm,
	}, nil
}

func (c *cvssV2) Severity() CvssRisk {
	s := c.base.Severity()
	switch s {
	case v2_metric.SeverityHigh:
		return HIGH
	case v2_metric.SeverityMedium:
		return MEDIUM
	case v2_metric.SeverityLow:
		return LOW
	default:
		return NONE
	}
}

// Implementation for V3
type cvssV3 struct {
	base *v3_metric.Base
}

func newBaseCvssV3(base string) (CVSS, error) {
	bm, err := v3_metric.NewBase().Decode(base)
	if err != nil {
		return nil, err
	}

	return &cvssV3{
		base: bm,
	}, nil
}

func (c *cvssV3) Severity() CvssRisk {
	s := c.base.Severity()
	switch s {
	case v3_metric.SeverityCritical:
		return CRITICAL
	case v3_metric.SeverityHigh:
		return HIGH
	case v3_metric.SeverityMedium:
		return MEDIUM
	case v3_metric.SeverityLow:
		return LOW
	default:
		return NONE
	}
}

// Factory
func NewCvssBaseString(raw string, version CvssVersion) (CVSS, error) {
	switch version {
	case CVSS_V2:
		return newBaseCvssV2(raw)
	case CVSS_V3:
		return newBaseCvssV3(raw)
	default:
		return nil, fmt.Errorf("unsupported CVSS version: %s", version)
	}
}
