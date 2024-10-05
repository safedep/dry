# SessionState

## Properties
Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Tags** | **[]string** |  | [optional] [default to null]
**AccessRights** | [**map[string]AccessDefinition**](AccessDefinition.md) |  | [optional] [default to null]
**Alias** | **string** |  | [optional] [default to null]
**Allowance** | **float64** |  | [optional] [default to null]
**ApplyPolicies** | **[]string** |  | [optional] [default to null]
**ApplyPolicyId** | **string** |  | [optional] [default to null]
**BasicAuthData** | [***SessionStateBasicAuthData**](SessionState_basic_auth_data.md) |  | [optional] [default to null]
**Certificate** | **string** |  | [optional] [default to null]
**DataExpires** | **int64** |  | [optional] [default to null]
**EnableDetailRecording** | **bool** |  | [optional] [default to null]
**Expires** | **int64** |  | [optional] [default to null]
**HmacEnabled** | **bool** |  | [optional] [default to null]
**HmacString** | **string** |  | [optional] [default to null]
**IdExtractorDeadline** | **int64** |  | [optional] [default to null]
**IsInactive** | **bool** |  | [optional] [default to null]
**JwtData** | [***SessionStateJwtData**](SessionState_jwt_data.md) |  | [optional] [default to null]
**LastCheck** | **int64** |  | [optional] [default to null]
**LastUpdated** | **string** |  | [optional] [default to null]
**MetaData** | [**map[string]interface{}**](interface{}.md) |  | [optional] [default to null]
**Monitor** | [***SessionStateMonitor**](SessionState_monitor.md) |  | [optional] [default to null]
**OauthClientId** | **string** |  | [optional] [default to null]
**OauthKeys** | **map[string]string** |  | [optional] [default to null]
**OrgId** | **string** |  | [optional] [default to null]
**Per** | **float64** |  | [optional] [default to null]
**QuotaMax** | **int64** |  | [optional] [default to null]
**QuotaRemaining** | **int64** |  | [optional] [default to null]
**QuotaRenewalRate** | **int64** |  | [optional] [default to null]
**QuotaRenews** | **int64** |  | [optional] [default to null]
**Rate** | **float64** |  | [optional] [default to null]
**SessionLifetime** | **int64** |  | [optional] [default to null]
**Smoothing** | [***RateLimitSmoothing**](RateLimitSmoothing.md) |  | [optional] [default to null]
**ThrottleInterval** | **float64** |  | [optional] [default to null]
**ThrottleRetryLimit** | **int64** |  | [optional] [default to null]

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

