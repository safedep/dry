# {{classname}}

All URIs are relative to *http://localhost/*

Method | HTTP request | Description
------------- | ------------- | -------------
[**AddPolicy**](PoliciesApi.md#AddPolicy) | **Post** /tyk/policies | Create a Policy
[**DeletePolicy**](PoliciesApi.md#DeletePolicy) | **Delete** /tyk/policies/{polID} | Delete a Policy
[**GetPolicy**](PoliciesApi.md#GetPolicy) | **Get** /tyk/policies/{polID} | Get a Policy
[**ListPolicies**](PoliciesApi.md#ListPolicies) | **Get** /tyk/policies | List Policies
[**UpdatePolicy**](PoliciesApi.md#UpdatePolicy) | **Put** /tyk/policies/{polID} | Update a Policy

# **AddPolicy**
> ApiModifyKeySuccess AddPolicy(ctx, optional)
Create a Policy

You can create a Policy in your Tyk Instance

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
 **optional** | ***PoliciesApiAddPolicyOpts** | optional parameters | nil if no parameters

### Optional Parameters
Optional parameters are passed through a pointer to a PoliciesApiAddPolicyOpts struct
Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **body** | [**optional.Interface of Policy**](Policy.md)|  | 

### Return type

[**ApiModifyKeySuccess**](apiModifyKeySuccess.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **DeletePolicy**
> ApiModifyKeySuccess DeletePolicy(ctx, polID)
Delete a Policy

Delete a policy by ID in your Tyk instance.

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **polID** | **string**| The policy ID | 

### Return type

[**ApiModifyKeySuccess**](apiModifyKeySuccess.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **GetPolicy**
> Policy GetPolicy(ctx, polID)
Get a Policy

You can retrieve details of a single policy by ID in your Tyk instance. Returns an array policies.

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **polID** | **string**| The policy ID | 

### Return type

[**Policy**](Policy.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ListPolicies**
> []Policy ListPolicies(ctx, )
List Policies

You can retrieve all the policies in your Tyk instance. Returns an array policies.

### Required Parameters
This endpoint does not need any parameter.

### Return type

[**[]Policy**](Policy.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **UpdatePolicy**
> ApiModifyKeySuccess UpdatePolicy(ctx, polID, optional)
Update a Policy

You can update a Policy in your Tyk Instance by ID

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **polID** | **string**| The policy ID | 
 **optional** | ***PoliciesApiUpdatePolicyOpts** | optional parameters | nil if no parameters

### Optional Parameters
Optional parameters are passed through a pointer to a PoliciesApiUpdatePolicyOpts struct
Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **body** | [**optional.Interface of Policy**](Policy.md)|  | 

### Return type

[**ApiModifyKeySuccess**](apiModifyKeySuccess.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

