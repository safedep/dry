# {{classname}}

All URIs are relative to *http://localhost/*

Method | HTTP request | Description
------------- | ------------- | -------------
[**Hello**](HealthCheckingApi.md#Hello) | **Get** /tyk/hello | Check the Health of the Tyk Gateway

# **Hello**
> string Hello(ctx, )
Check the Health of the Tyk Gateway

From v2.7.5 you can now rename the `/hello`  endpoint by using the `health_check_endpoint_name` option  Returns 200 response in case of success 

### Required Parameters
This endpoint does not need any parameter.

### Return type

**string**

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: text/html

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

