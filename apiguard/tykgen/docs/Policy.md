# Policy

## Properties
Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Id** | **string** |  | [optional] [default to null]
**Id** | **string** |  | [optional] [default to null]
**Name** | **string** |  | [optional] [default to null]
**OrgId** | **string** |  | [optional] [default to null]
**Rate** | **float64** |  | [optional] [default to null]
**Per** | **float64** |  | [optional] [default to null]
**QuotaMax** | **int64** |  | [optional] [default to null]
**QuotaRenewalRate** | **int64** |  | [optional] [default to null]
**ThrottleInterval** | **float64** |  | [optional] [default to null]
**ThrottleRetryLimit** | **float64** |  | [optional] [default to null]
**MaxQueryDepth** | **float64** |  | [optional] [default to null]
**AccessRights** | [**map[string]AccessDefinition**](AccessDefinition.md) |  | [optional] [default to null]
**HmacEnabled** | **bool** |  | [optional] [default to null]
**EnableHttpSignatureValidation** | **bool** |  | [optional] [default to null]
**Active** | **bool** |  | [optional] [default to null]
**IsInactive** | **bool** |  | [optional] [default to null]
**Tags** | **[]string** |  | [optional] [default to null]
**KeyExpiresIn** | **float64** |  | [optional] [default to null]
**Partitions** | [***PolicyPartitions**](PolicyPartitions.md) |  | [optional] [default to null]
**LastUpdated** | **string** |  | [optional] [default to null]
**Smoothing** | [***RateLimitSmoothing**](RateLimitSmoothing.md) |  | [optional] [default to null]
**MetaData** | [***interface{}**](interface{}.md) |  | [optional] [default to null]
**GraphqlAccessRights** | [***GraphAccessDefinition**](GraphAccessDefinition.md) |  | [optional] [default to null]

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

