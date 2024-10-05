# {{classname}}

All URIs are relative to *http://localhost/*

Method | HTTP request | Description
------------- | ------------- | -------------
[**CreateApi**](APIsApi.md#CreateApi) | **Post** /tyk/apis | 
[**DeleteApi**](APIsApi.md#DeleteApi) | **Delete** /tyk/apis/{apiID} | 
[**GetApi**](APIsApi.md#GetApi) | **Get** /tyk/apis/{apiID} | 
[**ListApiVersions**](APIsApi.md#ListApiVersions) | **Get** /tyk/apis/{apiID}/versions | 
[**ListApis**](APIsApi.md#ListApis) | **Get** /tyk/apis | 
[**UpdateApi**](APIsApi.md#UpdateApi) | **Put** /tyk/apis/{apiID} | 

# **CreateApi**
> ApiModifyKeySuccess CreateApi(ctx, optional)


Create API  A single Tyk node can have its API Definitions queried, deleted and updated remotely. This functionality enables you to remotely update your Tyk definitions without having to manage the files manually.

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
 **optional** | ***APIsApiCreateApiOpts** | optional parameters | nil if no parameters

### Optional Parameters
Optional parameters are passed through a pointer to a APIsApiCreateApiOpts struct
Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **body** | [**optional.Interface of ApiDefinition**](ApiDefinition.md)|  | 
 **baseApiId** | **optional.**| The base API which the new version will be linked to. | 
 **baseApiVersionName** | **optional.**| The version name of the base API while creating the first version. This doesn&#x27;t have to be sent for the next versions but if it is set, it will override base API version name. | 
 **newVersionName** | **optional.**| The version name of the created version. | 
 **setDefault** | **optional.**| If true, the new version is set as default version. | 

### Return type

[**ApiModifyKeySuccess**](apiModifyKeySuccess.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **DeleteApi**
> ApiStatusMessage DeleteApi(ctx, apiID)


Deleting an API definition will remove the file from the file store, the API definition will NOT be unloaded, a separate reload request will need to be made to disable the API endpoint.

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

# **GetApi**
> ApiDefinition GetApi(ctx, apiID)


Get API definition Only if used without the Tyk Dashboard

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **apiID** | **string**| The API ID | 

### Return type

[**ApiDefinition**](APIDefinition.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ListApiVersions**
> InlineResponse200 ListApiVersions(ctx, apiID, optional)


Listing versions of an OAS API

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **apiID** | **string**| The API ID | 
 **optional** | ***APIsApiListApiVersionsOpts** | optional parameters | nil if no parameters

### Optional Parameters
Optional parameters are passed through a pointer to a APIsApiListApiVersionsOpts struct
Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **searchText** | **optional.String**| Search for API version name | 
 **accessType** | **optional.String**| Filter for internal or external API versions | 

### Return type

[**InlineResponse200**](inline_response_200.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ListApis**
> []ApiDefinition ListApis(ctx, )


List APIs  Only if used without the Tyk Dashboard

### Required Parameters
This endpoint does not need any parameter.

### Return type

[**[]ApiDefinition**](APIDefinition.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **UpdateApi**
> ApiModifyKeySuccess UpdateApi(ctx, apiID, optional)


Updating an API definition uses the same signature an object as a `POST`, however it will first ensure that the API ID that is being updated is the same as the one in the object being `PUT`.   Updating will completely replace the file descriptor and will not change an API Definition that has already been loaded, the hot-reload endpoint will need to be called to push the new definition to live. 

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **apiID** | **string**| The API ID | 
 **optional** | ***APIsApiUpdateApiOpts** | optional parameters | nil if no parameters

### Optional Parameters
Optional parameters are passed through a pointer to a APIsApiUpdateApiOpts struct
Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **body** | [**optional.Interface of ApiDefinition**](ApiDefinition.md)|  | 

### Return type

[**ApiModifyKeySuccess**](apiModifyKeySuccess.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

