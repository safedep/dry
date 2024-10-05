# {{classname}}

All URIs are relative to *http://localhost/*

Method | HTTP request | Description
------------- | ------------- | -------------
[**Batch**](BatchRequestsApi.md#Batch) | **Post** /{listen_path}/tyk/batch | Run batch request

# **Batch**
> ApiStatusMessage Batch(ctx, listenPath)
Run batch request

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **listenPath** | **string**| API listen path | 

### Return type

[**ApiStatusMessage**](apiStatusMessage.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

