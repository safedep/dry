package apiguard

type PolicyAccess struct {
	ApiID   string
	ApiName string
}

type Policy struct {
	ID   string
	Name string

	Rate         float64
	RateInterval float64

	QuotaMax         int64
	QuotaRemaining   int64
	QuotaRenewalRate int64

	ThrottleInterval   float64
	ThrottleRetryLimit float64

	AccessRights []PolicyAccess

	Active bool
	Tags   []string

	Metadata map[string]interface{}
}
