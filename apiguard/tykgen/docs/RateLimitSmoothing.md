# RateLimitSmoothing

## Properties
Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Delay** | **int64** | Delay is a hold-off between smoothing events and controls how frequently the current allowance will step up or down (in seconds). | [optional] [default to null]
**Enabled** | **bool** | Enabled indicates if rate limit smoothing is active. | [optional] [default to null]
**Step** | **int64** | Step is the increment by which the current allowance will be increased or decreased each time a smoothing event is emitted. | [optional] [default to null]
**Threshold** | **int64** | Threshold is the initial rate limit beyond which smoothing will be applied. It is a count of requests during the &#x60;per&#x60; interval and should be less than the maximum configured &#x60;rate&#x60;. | [optional] [default to null]
**Trigger** | **float64** | Trigger is a fraction (typically in the range 0.1-1.0) of the step at which point a smoothing event will be emitted as the request rate approaches the current allowance. | [optional] [default to null]

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

