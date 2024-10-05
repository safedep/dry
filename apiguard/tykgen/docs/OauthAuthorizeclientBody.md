# OauthAuthorizeclientBody

## Properties
Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**ResponseType** | **string** | Should be provided by requesting client as part of authorisation request, this should be either &#x60;code&#x60; or &#x60;token&#x60; depending on the methods you have specified for the API. | [optional] [default to null]
**ClientId** | **string** | Should be provided by requesting client as part of authorisation request. The Client ID that is making the request. | [optional] [default to null]
**RedirectUri** | **string** | Should be provided by requesting client as part of authorisation request. Must match with the record stored with Tyk. | [optional] [default to null]
**KeyRules** | **string** | A string representation of a Session Object (form-encoded). This should be provided by your application in order to apply any quotas or rules to the key. | [optional] [default to null]

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

