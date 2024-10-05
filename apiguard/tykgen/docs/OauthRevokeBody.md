# OauthRevokeBody

## Properties
Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Token** | **string** | token to be revoked | [optional] [default to null]
**ClientId** | **string** | id of oauth client | [optional] [default to null]
**TokenTypeHint** | **string** | type of token to be revoked, if sent then the accepted values are access_token and refresh_token. String value and optional, of not provided then it will attempt to remove access and refresh tokens that matchs | [optional] [default to null]

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

