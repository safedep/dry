# {{classname}}

All URIs are relative to *http://localhost/*

Method | HTTP request | Description
------------- | ------------- | -------------
[**InvalidateCache**](CacheInvalidationApi.md#InvalidateCache) | **Delete** /tyk/cache/{apiID} | Invalidate cache

# **InvalidateCache**
> ApiStatusMessage InvalidateCache(ctx, apiID)
Invalidate cache

Invalidate cache for the given API

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **apiID** | **string**| The API ID | 

### Return type

[**ApiStatusMessage**](apiStatusMessage.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

