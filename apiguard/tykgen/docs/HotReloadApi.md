# {{classname}}

All URIs are relative to *http://localhost/*

Method | HTTP request | Description
------------- | ------------- | -------------
[**HotReload**](HotReloadApi.md#HotReload) | **Get** /tyk/reload/ | Hot-reload a single node
[**HotReloadGroup**](HotReloadApi.md#HotReloadGroup) | **Get** /tyk/reload/group | Hot-reload a Tyk group

# **HotReload**
> ApiStatusMessage HotReload(ctx, optional)
Hot-reload a single node

Tyk is capable of reloading configurations without having to stop serving requests. This means that API configurations can be added at runtime, or even modified at runtime and those rules applied immediately without any downtime.

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
 **optional** | ***HotReloadApiHotReloadOpts** | optional parameters | nil if no parameters

### Optional Parameters
Optional parameters are passed through a pointer to a HotReloadApiHotReloadOpts struct
Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **block** | **optional.Bool**| Block a response until the reload is performed. This can be useful in scripting environments like CI/CD workflows. | 

### Return type

[**ApiStatusMessage**](apiStatusMessage.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **HotReloadGroup**
> ApiStatusMessage HotReloadGroup(ctx, )
Hot-reload a Tyk group

To reload a whole group of Tyk nodes (without using the Dashboard or host manager). You can send an API request to a single node, this node will then send a notification through the pub/sub infrastructure to all other listening nodes (including the host manager if it is being used to manage NginX) which will then trigger a global reload.

### Required Parameters
This endpoint does not need any parameter.

### Return type

[**ApiStatusMessage**](apiStatusMessage.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

