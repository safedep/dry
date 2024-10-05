/*
 * Tyk Gateway API
 *
 * The Tyk Gateway API is the primary means for integrating your application with the Tyk API Gateway system. This API is very small, and has no granular permissions system. It is intended to be used purely for internal automation and integration.  **Warning: Under no circumstances should outside parties be granted access to this API.**  The Tyk Gateway API is capable of:  * Managing session objects (key generation) * Managing and listing policies * Managing and listing API Definitions (only when not using the Dashboard) * Hot reloads / reloading a cluster configuration * OAuth client creation (only when not using the Dashboard)   In order to use the Gateway API, you'll need to set the `secret` parameter in your tyk.conf file.  The shared secret you set should then be sent along as a header with each Gateway API Request in order for it to be successful:  ``` x-tyk-authorization: <your-secret> ``` <br/> <b>The Tyk Gateway API is subsumed by the Tyk Dashboard API in Pro installations.</b>
 *
 * API version: 5.5.0
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */
package swagger

type Policy struct {
	InternalId                    string                      `json:"_id,omitempty"`
	Id                            string                      `json:"id,omitempty"`
	Name                          string                      `json:"name,omitempty"`
	OrgId                         string                      `json:"org_id,omitempty"`
	Rate                          float64                     `json:"rate,omitempty"`
	Per                           float64                     `json:"per,omitempty"`
	QuotaMax                      int64                       `json:"quota_max,omitempty"`
	QuotaRenewalRate              int64                       `json:"quota_renewal_rate,omitempty"`
	ThrottleInterval              float64                     `json:"throttle_interval,omitempty"`
	ThrottleRetryLimit            float64                     `json:"throttle_retry_limit,omitempty"`
	MaxQueryDepth                 float64                     `json:"max_query_depth,omitempty"`
	AccessRights                  map[string]AccessDefinition `json:"access_rights,omitempty"`
	HmacEnabled                   bool                        `json:"hmac_enabled,omitempty"`
	EnableHttpSignatureValidation bool                        `json:"enable_http_signature_validation,omitempty"`
	Active                        bool                        `json:"active,omitempty"`
	IsInactive                    bool                        `json:"is_inactive,omitempty"`
	Tags                          []string                    `json:"tags,omitempty"`
	KeyExpiresIn                  float64                     `json:"key_expires_in,omitempty"`
	Partitions                    *PolicyPartitions           `json:"partitions,omitempty"`
	LastUpdated                   string                      `json:"last_updated,omitempty"`
	Smoothing                     *RateLimitSmoothing         `json:"smoothing,omitempty"`
	MetaData                      *interface{}                `json:"meta_data,omitempty"`
	GraphqlAccessRights           *GraphAccessDefinition      `json:"graphql_access_rights,omitempty"`
}
