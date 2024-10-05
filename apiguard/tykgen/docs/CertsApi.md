# {{classname}}

All URIs are relative to *http://localhost/*

Method | HTTP request | Description
------------- | ------------- | -------------
[**AddCert**](CertsApi.md#AddCert) | **Post** /tyk/certs | Add a certificate
[**DeleteCerts**](CertsApi.md#DeleteCerts) | **Delete** /tyk/certs | Delete Certificate
[**ListCerts**](CertsApi.md#ListCerts) | **Get** /tyk/certs | List Certificates

# **AddCert**
> ApiCertificateStatusMessage AddCert(ctx, orgId, optional)
Add a certificate

Add a certificate to the Tyk Gateway

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **orgId** | **string**| Organisation ID to list the certificates | 
 **optional** | ***CertsApiAddCertOpts** | optional parameters | nil if no parameters

### Optional Parameters
Optional parameters are passed through a pointer to a CertsApiAddCertOpts struct
Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **body** | [**optional.Interface of string**](string.md)|  | 

### Return type

[**ApiCertificateStatusMessage**](APICertificateStatusMessage.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: text/plain
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **DeleteCerts**
> ApiStatusMessage DeleteCerts(ctx, certID, orgId)
Delete Certificate

Delete certificate by id

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **certID** | **string**| Certifiicate ID to be deleted | 
  **orgId** | **string**| Organisation ID to list the certificates | 

### Return type

[**ApiStatusMessage**](apiStatusMessage.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ListCerts**
> InlineResponse2001 ListCerts(ctx, orgId, optional)
List Certificates

List All Certificates in the Tyk Gateway

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **orgId** | **string**| Organisation ID to list the certificates | 
 **optional** | ***CertsApiListCertsOpts** | optional parameters | nil if no parameters

### Optional Parameters
Optional parameters are passed through a pointer to a CertsApiListCertsOpts struct
Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **mode** | **optional.String**| Mode to list the certificate details | 
 **certID** | **optional.String**| Comma separated list of certificates to list | 

### Return type

[**InlineResponse2001**](inline_response_200_1.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

