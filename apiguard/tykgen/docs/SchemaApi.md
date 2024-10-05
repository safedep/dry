# {{classname}}

All URIs are relative to *http://localhost/*

Method | HTTP request | Description
------------- | ------------- | -------------
[**GetSchema**](SchemaApi.md#GetSchema) | **Get** /tyk/schema | 

# **GetSchema**
> OasSchemaResponse GetSchema(ctx, optional)


Get OAS schema

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
 **optional** | ***SchemaApiGetSchemaOpts** | optional parameters | nil if no parameters

### Optional Parameters
Optional parameters are passed through a pointer to a SchemaApiGetSchemaOpts struct
Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **oasVersion** | **optional.String**| The OAS version | 

### Return type

[**OasSchemaResponse**](OASSchemaResponse.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

