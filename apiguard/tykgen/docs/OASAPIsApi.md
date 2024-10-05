# {{classname}}

All URIs are relative to *http://localhost/*

Method | HTTP request | Description
------------- | ------------- | -------------
[**CreateApiOAS**](OASAPIsApi.md#CreateApiOAS) | **Post** /tyk/apis/oas | 
[**DeleteOASApi**](OASAPIsApi.md#DeleteOASApi) | **Delete** /tyk/apis/oas/{apiID} | 
[**DownloadApiOASPublic**](OASAPIsApi.md#DownloadApiOASPublic) | **Get** /tyk/apis/oas/{apiID}/export | 
[**DownloadApisOASPublic**](OASAPIsApi.md#DownloadApisOASPublic) | **Get** /tyk/apis/oas/export | 
[**ImportOAS**](OASAPIsApi.md#ImportOAS) | **Post** /tyk/apis/oas/import | 
[**ListApiOAS**](OASAPIsApi.md#ListApiOAS) | **Get** /tyk/apis/oas/{apiID} | 
[**ListApisOAS**](OASAPIsApi.md#ListApisOAS) | **Get** /tyk/apis/oas | 
[**ListOASApiVersions**](OASAPIsApi.md#ListOASApiVersions) | **Get** /tyk/apis/oas/{apiID}/versions | 
[**PatchApiOAS**](OASAPIsApi.md#PatchApiOAS) | **Patch** /tyk/apis/oas/{apiID} | Patch a single OAS API by ID
[**UpdateApiOAS**](OASAPIsApi.md#UpdateApiOAS) | **Put** /tyk/apis/oas/{apiID} | 

# **CreateApiOAS**
> ApiModifyKeySuccess CreateApiOAS(ctx, optional)


Create API with OAS format  A single Tyk node can have its API Definitions queried, deleted and updated remotely. This functionality enables you to remotely update your Tyk definitions without having to manage the files manually.

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
 **optional** | ***OASAPIsApiCreateApiOASOpts** | optional parameters | nil if no parameters

### Optional Parameters
Optional parameters are passed through a pointer to a OASAPIsApiCreateApiOASOpts struct
Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **body** | [**optional.Interface of Schema**](Schema.md)|  | 
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

# **DeleteOASApi**
> ApiStatusMessage DeleteOASApi(ctx, apiID)


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

# **DownloadApiOASPublic**
> OasSchemaResponse DownloadApiOASPublic(ctx, apiID, optional)


Download all OAS format APIs, when used without the Tyk Dashboard.

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **apiID** | **string**| The API ID | 
 **optional** | ***OASAPIsApiDownloadApiOASPublicOpts** | optional parameters | nil if no parameters

### Optional Parameters
Optional parameters are passed through a pointer to a OASAPIsApiDownloadApiOASPublicOpts struct
Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **mode** | **optional.String**| Mode of OAS export, by default mode could be empty which means to export OAS spec including OAS Tyk extension.  When mode&#x3D;public, OAS spec excluding Tyk extension is exported | 

### Return type

[**OasSchemaResponse**](OASSchemaResponse.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **DownloadApisOASPublic**
> []OasSchemaResponse DownloadApisOASPublic(ctx, optional)


Download all OAS format APIs, when used without the Tyk Dashboard.

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
 **optional** | ***OASAPIsApiDownloadApisOASPublicOpts** | optional parameters | nil if no parameters

### Optional Parameters
Optional parameters are passed through a pointer to a OASAPIsApiDownloadApisOASPublicOpts struct
Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **mode** | **optional.String**| The mode of OAS export. By default the mode is not set which means the OAS spec is exported including the OAS Tyk extension.   If the mode is set to public, the OAS spec excluding the Tyk extension is exported. | 

### Return type

[**[]OasSchemaResponse**](OASSchemaResponse.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ImportOAS**
> ApiModifyKeySuccess ImportOAS(ctx, optional)


Create a new OAS format API, without x-tyk-gateway. For use with an existing OAS API that you want to expose via your Tyk Gateway. (New)

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
 **optional** | ***OASAPIsApiImportOASOpts** | optional parameters | nil if no parameters

### Optional Parameters
Optional parameters are passed through a pointer to a OASAPIsApiImportOASOpts struct
Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **body** | [**optional.Interface of Schema**](Schema.md)|  | 
 **upstreamURL** | **optional.**| Upstream URL for the API | 
 **listenPath** | **optional.**| Listen path for the API | 
 **customDomain** | **optional.**| Custom domain for the API | 
 **apiID** | **optional.**| ID of the API | 
 **allowList** | [**optional.Interface of BooleanQueryParam**](.md)| Enable allowList middleware for all endpoints | 
 **mockResponse** | [**optional.Interface of BooleanQueryParam**](.md)| Enable mockResponse middleware for all endpoints having responses configured. | 
 **validateRequest** | [**optional.Interface of BooleanQueryParam**](.md)| Enable validateRequest middleware for all endpoints having a request body with media type application/json | 
 **authentication** | [**optional.Interface of BooleanQueryParam**](.md)| Enable or disable authentication in your Tyk Gateway as per your OAS document. | 

### Return type

[**ApiModifyKeySuccess**](apiModifyKeySuccess.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ListApiOAS**
> OasSchemaResponse ListApiOAS(ctx, apiID, optional)


Get API definition in OAS format Only if used without the Tyk Dashboard

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **apiID** | **string**| The API ID | 
 **optional** | ***OASAPIsApiListApiOASOpts** | optional parameters | nil if no parameters

### Optional Parameters
Optional parameters are passed through a pointer to a OASAPIsApiListApiOASOpts struct
Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **mode** | **optional.String**| Mode of OAS get, by default mode could be empty which means to get OAS spec including OAS Tyk extension.  When mode&#x3D;public, OAS spec excluding Tyk extension will be returned in the response | 

### Return type

[**OasSchemaResponse**](OASSchemaResponse.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ListApisOAS**
> []OasSchemaResponse ListApisOAS(ctx, optional)


List all OAS format APIs, when used without the Tyk Dashboard.

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
 **optional** | ***OASAPIsApiListApisOASOpts** | optional parameters | nil if no parameters

### Optional Parameters
Optional parameters are passed through a pointer to a OASAPIsApiListApisOASOpts struct
Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **mode** | **optional.String**| Mode of OAS get, by default mode could be empty which means to get OAS spec including OAS Tyk extension.  When mode&#x3D;public, OAS spec excluding Tyk extension will be returned in the response | 

### Return type

[**[]OasSchemaResponse**](OASSchemaResponse.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ListOASApiVersions**
> InlineResponse200 ListOASApiVersions(ctx, apiID, optional)


Listing versions of an OAS API

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **apiID** | **string**| The API ID | 
 **optional** | ***OASAPIsApiListOASApiVersionsOpts** | optional parameters | nil if no parameters

### Optional Parameters
Optional parameters are passed through a pointer to a OASAPIsApiListOASApiVersionsOpts struct
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

# **PatchApiOAS**
> ApiModifyKeySuccess PatchApiOAS(ctx, apiID, optional)
Patch a single OAS API by ID

Update API with OAS format. You can use this endpoint to update OAS part of the tyk API definition. This endpoint allows you to configure tyk OAS extension based on query params provided(similar to import)

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **apiID** | **string**| The API ID | 
 **optional** | ***OASAPIsApiPatchApiOASOpts** | optional parameters | nil if no parameters

### Optional Parameters
Optional parameters are passed through a pointer to a OASAPIsApiPatchApiOASOpts struct
Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **body** | [**optional.Interface of Schema**](Schema.md)|  | 
 **upstreamURL** | **optional.**| Upstream URL for the API | 
 **listenPath** | **optional.**| Listen path for the API | 
 **customDomain** | **optional.**| Custom domain for the API | 
 **validateRequest** | [**optional.Interface of BooleanQueryParam**](.md)| Enable validateRequest middleware for all endpoints having a request body with media type application/json | 
 **allowList** | [**optional.Interface of BooleanQueryParam**](.md)| Enable allowList middleware for all endpoints | 
 **mockResponse** | [**optional.Interface of BooleanQueryParam**](.md)| Enable mockResponse middleware for all endpoints having responses configured. | 
 **authentication** | [**optional.Interface of BooleanQueryParam**](.md)| Enable or disable authentication in your Tyk Gateway as per your OAS document. | 

### Return type

[**ApiModifyKeySuccess**](apiModifyKeySuccess.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **UpdateApiOAS**
> ApiModifyKeySuccess UpdateApiOAS(ctx, apiID, optional)


Updating an API definition uses the same signature an object as a `POST`, however it will first ensure that the API ID that is being updated is the same as the one in the object being `PUT`.   Updating will completely replace the file descriptor and will not change an API Definition that has already been loaded, the hot-reload endpoint will need to be called to push the new definition to live. 

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **apiID** | **string**| The API ID | 
 **optional** | ***OASAPIsApiUpdateApiOASOpts** | optional parameters | nil if no parameters

### Optional Parameters
Optional parameters are passed through a pointer to a OASAPIsApiUpdateApiOASOpts struct
Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **body** | [**optional.Interface of Schema**](Schema.md)|  | 

### Return type

[**ApiModifyKeySuccess**](apiModifyKeySuccess.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

