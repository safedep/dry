# {{classname}}

All URIs are relative to *http://localhost/*

Method | HTTP request | Description
------------- | ------------- | -------------
[**AddKey**](KeysApi.md#AddKey) | **Post** /tyk/keys | Create a key
[**CreateCustomKey**](KeysApi.md#CreateCustomKey) | **Post** /tyk/keys/{keyID} | Create Custom Key / Import Key
[**DeleteKey**](KeysApi.md#DeleteKey) | **Delete** /tyk/keys/{keyID} | Delete Key
[**GetKey**](KeysApi.md#GetKey) | **Get** /tyk/keys/{keyID} | Get a Key
[**ListKeys**](KeysApi.md#ListKeys) | **Get** /tyk/keys | List Keys
[**UpdateKey**](KeysApi.md#UpdateKey) | **Put** /tyk/keys/{keyID} | Update Key

# **AddKey**
> ApiModifyKeySuccess AddKey(ctx, optional)
Create a key

Tyk will generate the access token based on the OrgID specified in the API Definition and a random UUID. This ensures that keys can be \"owned\" by different API Owners should segmentation be needed at an organisational level. <br/><br/> API keys without access_rights data will be written to all APIs on the system (this also means that they will be created across all SessionHandlers and StorageHandlers, it is recommended to always embed access_rights data in a key to ensure that only targeted APIs and their back-ends are written to.

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
 **optional** | ***KeysApiAddKeyOpts** | optional parameters | nil if no parameters

### Optional Parameters
Optional parameters are passed through a pointer to a KeysApiAddKeyOpts struct
Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **body** | [**optional.Interface of SessionState**](SessionState.md)|  | 

### Return type

[**ApiModifyKeySuccess**](apiModifyKeySuccess.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **CreateCustomKey**
> ApiModifyKeySuccess CreateCustomKey(ctx, keyID, optional)
Create Custom Key / Import Key

You can use the `POST /tyk/keys/{KEY_ID}` endpoint as defined below to import existing keys into Tyk.  This example uses standard `authorization` header authentication, and assumes that the Gateway is located at `127.0.0.1:8080` and the Tyk secret is `352d20ee67be67f6340b4c0605b044b7` - update these as necessary to match your environment.  To import a key called `mycustomkey`, save the JSON contents as `token.json` (see example below), then run the following Curl command.  ``` curl http://127.0.0.1:8080/tyk/keys/mycustomkey -H 'x-tyk-authorization: 352d20ee67be67f6340b4c0605b044b7' -H 'Content-Type: application/json'  -d @token.json ```  The following request will fail as the key doesn't exist.  ``` curl http://127.0.0.1:8080/quickstart/headers -H 'Authorization. invalid123' ```  But this request will now work, using the imported key.  ``` curl http://127.0.0.1:8080/quickstart/headers -H 'Authorization: mycustomkey' ```  <h4>Example token.json file<h4>  ``` {   \"allowance\": 1000,   \"rate\": 1000,   \"per\": 60,   \"expires\": -1,   \"quota_max\": -1,   \"quota_renews\": 1406121006,   \"quota_remaining\": 0,   \"quota_renewal_rate\": 60,   \"access_rights\": {     \"3\": {       \"api_name\": \"Tyk Test API\",       \"api_id\": \"3\"     }   },   \"org_id\": \"53ac07777cbb8c2d53000002\",   \"basic_auth_data\": {     \"password\": \"\",     \"hash_type\": \"\"   },   \"hmac_enabled\": false,   \"hmac_string\": \"\",   \"is_inactive\": false,   \"apply_policy_id\": \"\",   \"apply_policies\": [     \"59672779fa4387000129507d\",     \"53222349fa4387004324324e\",     \"543534s9fa4387004324324d\"     ],   \"monitor\": {     \"trigger_limits\": []   } } ```

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **keyID** | **string**| The Key ID | 
 **optional** | ***KeysApiCreateCustomKeyOpts** | optional parameters | nil if no parameters

### Optional Parameters
Optional parameters are passed through a pointer to a KeysApiCreateCustomKeyOpts struct
Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **body** | [**optional.Interface of SessionState**](SessionState.md)|  | 
 **hashed** | **optional.**| Use the hash of the key as input instead of the full key | 
 **suppressReset** | **optional.**| Adding the suppress_reset parameter and setting it to 1, will cause Tyk not to reset the quota limit that is in the current live quota manager. By default Tyk will reset the quota in the live quota manager (initialising it) when adding a key. Adding the &#x60;suppress_reset&#x60; flag to the URL parameters will avoid this behaviour. | 

### Return type

[**ApiModifyKeySuccess**](apiModifyKeySuccess.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **DeleteKey**
> ApiStatusMessage DeleteKey(ctx, keyID, optional)
Delete Key

Deleting a key will remove it permanently from the system, however analytics relating to that key will still be available.

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **keyID** | **string**| The Key ID | 
 **optional** | ***KeysApiDeleteKeyOpts** | optional parameters | nil if no parameters

### Optional Parameters
Optional parameters are passed through a pointer to a KeysApiDeleteKeyOpts struct
Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **hashed** | **optional.Bool**| Use the hash of the key as input instead of the full key | 

### Return type

[**ApiStatusMessage**](apiStatusMessage.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **GetKey**
> SessionState GetKey(ctx, keyID, optional)
Get a Key

Get session info about the specified key. Should return up to date rate limit and quota usage numbers.

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **keyID** | **string**| The Key ID | 
 **optional** | ***KeysApiGetKeyOpts** | optional parameters | nil if no parameters

### Optional Parameters
Optional parameters are passed through a pointer to a KeysApiGetKeyOpts struct
Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **hashed** | **optional.Bool**| Use the hash of the key as input instead of the full key | 

### Return type

[**SessionState**](SessionState.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ListKeys**
> ApiAllKeys ListKeys(ctx, )
List Keys

You can retrieve all the keys in your Tyk instance. Returns an array of Key IDs.

### Required Parameters
This endpoint does not need any parameter.

### Return type

[**ApiAllKeys**](apiAllKeys.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **UpdateKey**
> ApiModifyKeySuccess UpdateKey(ctx, keyID, optional)
Update Key

You can also manually add keys to Tyk using your own key-generation algorithm. It is recommended if using this approach to ensure that the OrgID being used in the API Definition and the key data is blank so that Tyk does not try to prepend or manage the key in any way.

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **keyID** | **string**| The Key ID | 
 **optional** | ***KeysApiUpdateKeyOpts** | optional parameters | nil if no parameters

### Optional Parameters
Optional parameters are passed through a pointer to a KeysApiUpdateKeyOpts struct
Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **body** | [**optional.Interface of SessionState**](SessionState.md)|  | 
 **hashed** | **optional.**| Use the hash of the key as input instead of the full key | 
 **suppressReset** | **optional.**| Adding the suppress_reset parameter and setting it to 1, will cause Tyk not to reset the quota limit that is in the current live quota manager. By default Tyk will reset the quota in the live quota manager (initialising it) when adding a key. Adding the &#x60;suppress_reset&#x60; flag to the URL parameters will avoid this behaviour. | 

### Return type

[**ApiModifyKeySuccess**](apiModifyKeySuccess.md)

### Authorization

[api_key](../README.md#api_key)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

