# {{classname}}

All URIs are relative to *http://localhost/*

Method | HTTP request | Description
------------- | ------------- | -------------
[**AddOrgKey**](OrganisationQuotasApi.md#AddOrgKey) | **Post** /tyk/org/keys/{keyID} | Create an organisation key
[**DeleteOrgKey**](OrganisationQuotasApi.md#DeleteOrgKey) | **Delete** /tyk/org/keys/{keyID} | Delete Organisation Key
[**GetOrgKey**](OrganisationQuotasApi.md#GetOrgKey) | **Get** /tyk/org/keys/{keyID} | Get an Organisation Key
[**ListOrgKeys**](OrganisationQuotasApi.md#ListOrgKeys) | **Get** /tyk/org/keys | List Organisation Keys
[**UpdateOrgKey**](OrganisationQuotasApi.md#UpdateOrgKey) | **Put** /tyk/org/keys/{keyID} | Update Organisation Key

# **AddOrgKey**
> ApiModifyKeySuccess AddOrgKey(ctx, keyID, optional)
Create an organisation key

This work similar to Keys API except that Key ID is always equals Organisation ID

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **keyID** | **string**| The Key ID | 
 **optional** | ***OrganisationQuotasApiAddOrgKeyOpts** | optional parameters | nil if no parameters

### Optional Parameters
Optional parameters are passed through a pointer to a OrganisationQuotasApiAddOrgKeyOpts struct
Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **body** | [**optional.Interface of SessionState**](SessionState.md)|  | 

### Return type

[**ApiModifyKeySuccess**](apiModifyKeySuccess.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **DeleteOrgKey**
> ApiStatusMessage DeleteOrgKey(ctx, keyID)
Delete Organisation Key

Deleting a key will remove all limits from organisation. It does not affects regualar keys created within organisation.

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **keyID** | **string**| The Key ID | 

### Return type

[**ApiStatusMessage**](apiStatusMessage.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **GetOrgKey**
> SessionState GetOrgKey(ctx, keyID)
Get an Organisation Key

Get session info about specified orgnanisation key. Should return up to date rate limit and quota usage numbers.

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **keyID** | **string**| The Key ID | 

### Return type

[**SessionState**](SessionState.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ListOrgKeys**
> InlineResponse2002 ListOrgKeys(ctx, )
List Organisation Keys

You can now set rate limits at the organisation level by using the following fields - allowance and rate. These are the number of allowed requests for the specified per value, and need to be set to the same value. If you don't want to have organisation level rate limiting, set 'rate' or 'per' to zero, or don't add them to your request.

### Required Parameters
This endpoint does not need any parameter.

### Return type

[**InlineResponse2002**](inline_response_200_2.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **UpdateOrgKey**
> ApiModifyKeySuccess UpdateOrgKey(ctx, keyID, optional)
Update Organisation Key

This work similar to Keys API except that Key ID is always equals Organisation ID  For Gateway v2.6.0 onwards, you can now set rate limits at the organisation level by using the following fields - allowance and rate. These are the number of allowed requests for the specified per value, and need to be set to the same value. If you don't want to have organisation level rate limiting, set `rate` or `per` to zero, or don't add them to your request.

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **keyID** | **string**| The Key ID | 
 **optional** | ***OrganisationQuotasApiUpdateOrgKeyOpts** | optional parameters | nil if no parameters

### Optional Parameters
Optional parameters are passed through a pointer to a OrganisationQuotasApiUpdateOrgKeyOpts struct
Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **body** | [**optional.Interface of SessionState**](SessionState.md)|  | 
 **resetQuota** | **optional.**| Adding the &#x60;reset_quota&#x60; parameter and setting it to 1, will cause Tyk reset the organisations quota in the live quota manager, it is recommended to use this mechanism to reset organisation-level access if a monthly subscription is in place. | 

### Return type

[**ApiModifyKeySuccess**](apiModifyKeySuccess.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

