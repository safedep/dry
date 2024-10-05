# {{classname}}

All URIs are relative to *http://localhost/*

Method | HTTP request | Description
------------- | ------------- | -------------
[**AuthorizeClient**](OAuthApi.md#AuthorizeClient) | **Post** /tyk/oauth/authorize-client/ | Authorize client
[**CreateOAuthClient**](OAuthApi.md#CreateOAuthClient) | **Post** /tyk/oauth/clients/create | Create new OAuth client
[**DeleteOAuthClient**](OAuthApi.md#DeleteOAuthClient) | **Delete** /tyk/oauth/clients/{apiID}/{keyName} | Delete OAuth client
[**GetOAuthClient**](OAuthApi.md#GetOAuthClient) | **Get** /tyk/oauth/clients/{apiID}/{keyName} | Get OAuth client
[**GetOAuthClientTokens**](OAuthApi.md#GetOAuthClientTokens) | **Get** /tyk/oauth/clients/{apiID}/{keyName}/tokens | List tokens
[**InvalidateOAuthRefresh**](OAuthApi.md#InvalidateOAuthRefresh) | **Delete** /tyk/oauth/refresh/{keyName} | Invalidate OAuth refresh token
[**ListOAuthClients**](OAuthApi.md#ListOAuthClients) | **Get** /tyk/oauth/clients/{apiID} | List oAuth clients
[**RevokeAllTokens**](OAuthApi.md#RevokeAllTokens) | **Post** /tyk/oauth/revoke_all | revoke all client&#x27;s tokens
[**RevokeSingleToken**](OAuthApi.md#RevokeSingleToken) | **Post** /tyk/oauth/revoke | revoke token
[**UpdateoAuthClient**](OAuthApi.md#UpdateoAuthClient) | **Put** /tyk/oauth/clients/{apiID} | Update OAuth metadata and Policy ID

# **AuthorizeClient**
> interface{} AuthorizeClient(ctx, responseType, clientId, redirectUri, keyRules)
Authorize client

With the OAuth flow you will need to create authorisation or access tokens for your clients, in order to do this, Tyk provides a private API endpoint for your application to generate these codes and redirect the end-user back to the API Client.

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **responseType** | **string**|  | 
  **clientId** | **string**|  | 
  **redirectUri** | **string**|  | 
  **keyRules** | **string**|  | 

### Return type

[**interface{}**](interface{}.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: application/x-www-form-urlencoded
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **CreateOAuthClient**
> NewClientRequest CreateOAuthClient(ctx, optional)
Create new OAuth client

Any OAuth keys must be generated with the help of a client ID. These need to be pre-registered with Tyk before they can be used (in a similar vein to how you would register your app with Twitter before attempting to ask user permissions using their API). <br/><br/> <h3>Creating OAuth clients with Access to Multiple APIs</h3> New from Tyk Gateway 2.6.0 is the ability to create OAuth clients with access to more than one API. If you provide the api_id it works the same as in previous releases. If you don't provide the api_id the request uses policy access rights and enumerates APIs from their setting in the newly created OAuth-client.

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
 **optional** | ***OAuthApiCreateOAuthClientOpts** | optional parameters | nil if no parameters

### Optional Parameters
Optional parameters are passed through a pointer to a OAuthApiCreateOAuthClientOpts struct
Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **body** | [**optional.Interface of NewClientRequest**](NewClientRequest.md)|  | 

### Return type

[**NewClientRequest**](NewClientRequest.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **DeleteOAuthClient**
> ApiModifyKeySuccess DeleteOAuthClient(ctx, apiID, keyName)
Delete OAuth client

Please note that tokens issued with the client ID will still be valid until they expire.

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **apiID** | **string**| The API ID | 
  **keyName** | **string**| The Client ID | 

### Return type

[**ApiModifyKeySuccess**](apiModifyKeySuccess.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **GetOAuthClient**
> NewClientRequest GetOAuthClient(ctx, apiID, keyName)
Get OAuth client

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **apiID** | **string**| The API ID | 
  **keyName** | **string**| The Client ID | 

### Return type

[**NewClientRequest**](NewClientRequest.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **GetOAuthClientTokens**
> []string GetOAuthClientTokens(ctx, apiID, keyName)
List tokens

This endpoint allows you to retrieve a list of all current tokens and their expiry date for a provided API ID and OAuth-client ID in the following format. This endpoint will work only for newly created tokens. <br/> <br/> You can control how long you want to store expired tokens in this list using `oauth_token_expired_retain_period` gateway option, which specifies retain period for expired tokens stored in Redis. By default expired token not get removed. See <a href=\"https://tyk.io/docs/configure/tyk-gateway-configuration-options/#a-name-oauth-token-expired-retain-period-a-oauth-token-expired-retain-period\" target=\"_blank\">here</a> for more details.

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **apiID** | **string**| The API ID | 
  **keyName** | **string**| The Client ID | 

### Return type

**[]string**

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **InvalidateOAuthRefresh**
> ApiModifyKeySuccess InvalidateOAuthRefresh(ctx, apiId, keyName)
Invalidate OAuth refresh token

It is possible to invalidate refresh tokens in order to manage OAuth client access more robustly.

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **apiId** | **string**| The API id | 
  **keyName** | **string**| Refresh token | 

### Return type

[**ApiModifyKeySuccess**](apiModifyKeySuccess.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ListOAuthClients**
> []NewClientRequest ListOAuthClients(ctx, apiID)
List oAuth clients

OAuth Clients are organised by API ID, and therefore are queried as such.

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **apiID** | **string**| The API ID | 

### Return type

[**[]NewClientRequest**](NewClientRequest.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **RevokeAllTokens**
> RevokeAllTokens(ctx, clientId, clientSecret)
revoke all client's tokens

revoke all the tokens for a given oauth client

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **clientId** | **string**|  | 
  **clientSecret** | **string**|  | 

### Return type

 (empty response body)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: application/x-www-form-urlencoded
 - **Accept**: Not defined

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **RevokeSingleToken**
> RevokeSingleToken(ctx, token, clientId, tokenTypeHint)
revoke token

revoke a single token

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **token** | **string**|  | 
  **clientId** | **string**|  | 
  **tokenTypeHint** | **string**|  | 

### Return type

 (empty response body)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: application/x-www-form-urlencoded
 - **Accept**: Not defined

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **UpdateoAuthClient**
> []NewClientRequest UpdateoAuthClient(ctx, apiID)
Update OAuth metadata and Policy ID

Allows you to update the metadata and Policy ID for an OAuth client.

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **apiID** | **string**| The API ID | 

### Return type

[**[]NewClientRequest**](NewClientRequest.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

