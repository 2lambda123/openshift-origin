// Code generated by private/model/cli/gen-api/main.go. DO NOT EDIT.

package forecastqueryservice

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awsutil"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/private/protocol"
)

const opQueryForecast = "QueryForecast"

// QueryForecastRequest generates a "aws/request.Request" representing the
// client's request for the QueryForecast operation. The "output" return
// value will be populated with the request's response once the request completes
// successfully.
//
// Use "Send" method on the returned Request to send the API call to the service.
// the "output" return value is not valid until after Send returns without error.
//
// See QueryForecast for more information on using the QueryForecast
// API call, and error handling.
//
// This method is useful when you want to inject custom logic or configuration
// into the SDK's request lifecycle. Such as custom headers, or retry logic.
//
//
//    // Example sending a request using the QueryForecastRequest method.
//    req, resp := client.QueryForecastRequest(params)
//
//    err := req.Send()
//    if err == nil { // resp is now filled
//        fmt.Println(resp)
//    }
//
// See also, https://docs.aws.amazon.com/goto/WebAPI/forecastquery-2018-06-26/QueryForecast
func (c *ForecastQueryService) QueryForecastRequest(input *QueryForecastInput) (req *request.Request, output *QueryForecastOutput) {
	op := &request.Operation{
		Name:       opQueryForecast,
		HTTPMethod: "POST",
		HTTPPath:   "/",
	}

	if input == nil {
		input = &QueryForecastInput{}
	}

	output = &QueryForecastOutput{}
	req = c.newRequest(op, input, output)
	return
}

// QueryForecast API operation for Amazon Forecast Query Service.
//
// Retrieves a forecast filtered by the supplied criteria.
//
// The criteria is a key-value pair. The key is either item_id (or the equivalent
// non-timestamp, non-target field) from the TARGET_TIME_SERIES dataset, or
// one of the forecast dimensions specified as part of the FeaturizationConfig
// object.
//
// By default, the complete date range of the filtered forecast is returned.
// Optionally, you can request a specific date range within the forecast.
//
// The forecasts generated by Amazon Forecast are in the same timezone as the
// dataset that was used to create the predictor.
//
// Returns awserr.Error for service API and SDK errors. Use runtime type assertions
// with awserr.Error's Code and Message methods to get detailed information about
// the error.
//
// See the AWS API reference guide for Amazon Forecast Query Service's
// API operation QueryForecast for usage and error information.
//
// Returned Error Types:
//   * ResourceNotFoundException
//   We can't find that resource. Check the information that you've provided and
//   try again.
//
//   * ResourceInUseException
//   The specified resource is in use.
//
//   * InvalidInputException
//   The value that you provided was invalid or too long.
//
//   * LimitExceededException
//   The limit on the number of requests per second has been exceeded.
//
//   * InvalidNextTokenException
//   The token is not valid. Tokens expire after 24 hours.
//
// See also, https://docs.aws.amazon.com/goto/WebAPI/forecastquery-2018-06-26/QueryForecast
func (c *ForecastQueryService) QueryForecast(input *QueryForecastInput) (*QueryForecastOutput, error) {
	req, out := c.QueryForecastRequest(input)
	return out, req.Send()
}

// QueryForecastWithContext is the same as QueryForecast with the addition of
// the ability to pass a context and additional request options.
//
// See QueryForecast for details on how to use this API operation.
//
// The context must be non-nil and will be used for request cancellation. If
// the context is nil a panic will occur. In the future the SDK may create
// sub-contexts for http.Requests. See https://golang.org/pkg/context/
// for more information on using Contexts.
func (c *ForecastQueryService) QueryForecastWithContext(ctx aws.Context, input *QueryForecastInput, opts ...request.Option) (*QueryForecastOutput, error) {
	req, out := c.QueryForecastRequest(input)
	req.SetContext(ctx)
	req.ApplyOptions(opts...)
	return out, req.Send()
}

// The forecast value for a specific date. Part of the Forecast object.
type DataPoint struct {
	_ struct{} `type:"structure"`

	// The timestamp of the specific forecast.
	Timestamp *string `type:"string"`

	// The forecast value.
	Value *float64 `type:"double"`
}

// String returns the string representation
func (s DataPoint) String() string {
	return awsutil.Prettify(s)
}

// GoString returns the string representation
func (s DataPoint) GoString() string {
	return s.String()
}

// SetTimestamp sets the Timestamp field's value.
func (s *DataPoint) SetTimestamp(v string) *DataPoint {
	s.Timestamp = &v
	return s
}

// SetValue sets the Value field's value.
func (s *DataPoint) SetValue(v float64) *DataPoint {
	s.Value = &v
	return s
}

// Provides information about a forecast. Returned as part of the QueryForecast
// response.
type Forecast struct {
	_ struct{} `type:"structure"`

	// The forecast.
	//
	// The string of the string to array map is one of the following values:
	//
	//    * mean
	//
	//    * p10
	//
	//    * p50
	//
	//    * p90
	Predictions map[string][]*DataPoint `type:"map"`
}

// String returns the string representation
func (s Forecast) String() string {
	return awsutil.Prettify(s)
}

// GoString returns the string representation
func (s Forecast) GoString() string {
	return s.String()
}

// SetPredictions sets the Predictions field's value.
func (s *Forecast) SetPredictions(v map[string][]*DataPoint) *Forecast {
	s.Predictions = v
	return s
}

// The value that you provided was invalid or too long.
type InvalidInputException struct {
	_            struct{} `type:"structure"`
	respMetadata protocol.ResponseMetadata

	Message_ *string `locationName:"Message" type:"string"`
}

// String returns the string representation
func (s InvalidInputException) String() string {
	return awsutil.Prettify(s)
}

// GoString returns the string representation
func (s InvalidInputException) GoString() string {
	return s.String()
}

func newErrorInvalidInputException(v protocol.ResponseMetadata) error {
	return &InvalidInputException{
		respMetadata: v,
	}
}

// Code returns the exception type name.
func (s InvalidInputException) Code() string {
	return "InvalidInputException"
}

// Message returns the exception's message.
func (s InvalidInputException) Message() string {
	if s.Message_ != nil {
		return *s.Message_
	}
	return ""
}

// OrigErr always returns nil, satisfies awserr.Error interface.
func (s InvalidInputException) OrigErr() error {
	return nil
}

func (s InvalidInputException) Error() string {
	return fmt.Sprintf("%s: %s", s.Code(), s.Message())
}

// Status code returns the HTTP status code for the request's response error.
func (s InvalidInputException) StatusCode() int {
	return s.respMetadata.StatusCode
}

// RequestID returns the service's response RequestID for request.
func (s InvalidInputException) RequestID() string {
	return s.respMetadata.RequestID
}

// The token is not valid. Tokens expire after 24 hours.
type InvalidNextTokenException struct {
	_            struct{} `type:"structure"`
	respMetadata protocol.ResponseMetadata

	Message_ *string `locationName:"Message" type:"string"`
}

// String returns the string representation
func (s InvalidNextTokenException) String() string {
	return awsutil.Prettify(s)
}

// GoString returns the string representation
func (s InvalidNextTokenException) GoString() string {
	return s.String()
}

func newErrorInvalidNextTokenException(v protocol.ResponseMetadata) error {
	return &InvalidNextTokenException{
		respMetadata: v,
	}
}

// Code returns the exception type name.
func (s InvalidNextTokenException) Code() string {
	return "InvalidNextTokenException"
}

// Message returns the exception's message.
func (s InvalidNextTokenException) Message() string {
	if s.Message_ != nil {
		return *s.Message_
	}
	return ""
}

// OrigErr always returns nil, satisfies awserr.Error interface.
func (s InvalidNextTokenException) OrigErr() error {
	return nil
}

func (s InvalidNextTokenException) Error() string {
	return fmt.Sprintf("%s: %s", s.Code(), s.Message())
}

// Status code returns the HTTP status code for the request's response error.
func (s InvalidNextTokenException) StatusCode() int {
	return s.respMetadata.StatusCode
}

// RequestID returns the service's response RequestID for request.
func (s InvalidNextTokenException) RequestID() string {
	return s.respMetadata.RequestID
}

// The limit on the number of requests per second has been exceeded.
type LimitExceededException struct {
	_            struct{} `type:"structure"`
	respMetadata protocol.ResponseMetadata

	Message_ *string `locationName:"Message" type:"string"`
}

// String returns the string representation
func (s LimitExceededException) String() string {
	return awsutil.Prettify(s)
}

// GoString returns the string representation
func (s LimitExceededException) GoString() string {
	return s.String()
}

func newErrorLimitExceededException(v protocol.ResponseMetadata) error {
	return &LimitExceededException{
		respMetadata: v,
	}
}

// Code returns the exception type name.
func (s LimitExceededException) Code() string {
	return "LimitExceededException"
}

// Message returns the exception's message.
func (s LimitExceededException) Message() string {
	if s.Message_ != nil {
		return *s.Message_
	}
	return ""
}

// OrigErr always returns nil, satisfies awserr.Error interface.
func (s LimitExceededException) OrigErr() error {
	return nil
}

func (s LimitExceededException) Error() string {
	return fmt.Sprintf("%s: %s", s.Code(), s.Message())
}

// Status code returns the HTTP status code for the request's response error.
func (s LimitExceededException) StatusCode() int {
	return s.respMetadata.StatusCode
}

// RequestID returns the service's response RequestID for request.
func (s LimitExceededException) RequestID() string {
	return s.respMetadata.RequestID
}

type QueryForecastInput struct {
	_ struct{} `type:"structure"`

	// The end date for the forecast. Specify the date using this format: yyyy-MM-dd'T'HH:mm:ss'Z'
	// (ISO 8601 format). For example, "1970-01-01T00:00:00Z."
	EndDate *string `type:"string"`

	// The filtering criteria to apply when retrieving the forecast. For example:
	//
	//    * To get a forecast for a specific item specify the following: {"item_id"
	//    : "client_1"}
	//
	//    * To get a forecast for a specific item sold in a specific location, specify
	//    the following: {"item_id" : "client_1", "location" : "ny"}
	//
	//    * To get a forecast for all blue items sold in a specific location, specify
	//    the following: { "location" : "ny", "color":"blue"}
	//
	// To get the full forecast, use the operation.
	//
	// Filters is a required field
	Filters map[string]*string `min:"1" type:"map" required:"true"`

	// The Amazon Resource Name (ARN) of the forecast to query.
	//
	// ForecastArn is a required field
	ForecastArn *string `type:"string" required:"true"`

	// If the result of the previous request was truncated, the response includes
	// a NextToken. To retrieve the next set of results, use the token in the next
	// request. Tokens expire after 24 hours.
	NextToken *string `min:"1" type:"string"`

	// The start date for the forecast. Specify the date using this format: yyyy-MM-dd'T'HH:mm:ss'Z'
	// (ISO 8601 format) For example, "1970-01-01T00:00:00Z."
	StartDate *string `type:"string"`
}

// String returns the string representation
func (s QueryForecastInput) String() string {
	return awsutil.Prettify(s)
}

// GoString returns the string representation
func (s QueryForecastInput) GoString() string {
	return s.String()
}

// Validate inspects the fields of the type to determine if they are valid.
func (s *QueryForecastInput) Validate() error {
	invalidParams := request.ErrInvalidParams{Context: "QueryForecastInput"}
	if s.Filters == nil {
		invalidParams.Add(request.NewErrParamRequired("Filters"))
	}
	if s.Filters != nil && len(s.Filters) < 1 {
		invalidParams.Add(request.NewErrParamMinLen("Filters", 1))
	}
	if s.ForecastArn == nil {
		invalidParams.Add(request.NewErrParamRequired("ForecastArn"))
	}
	if s.NextToken != nil && len(*s.NextToken) < 1 {
		invalidParams.Add(request.NewErrParamMinLen("NextToken", 1))
	}

	if invalidParams.Len() > 0 {
		return invalidParams
	}
	return nil
}

// SetEndDate sets the EndDate field's value.
func (s *QueryForecastInput) SetEndDate(v string) *QueryForecastInput {
	s.EndDate = &v
	return s
}

// SetFilters sets the Filters field's value.
func (s *QueryForecastInput) SetFilters(v map[string]*string) *QueryForecastInput {
	s.Filters = v
	return s
}

// SetForecastArn sets the ForecastArn field's value.
func (s *QueryForecastInput) SetForecastArn(v string) *QueryForecastInput {
	s.ForecastArn = &v
	return s
}

// SetNextToken sets the NextToken field's value.
func (s *QueryForecastInput) SetNextToken(v string) *QueryForecastInput {
	s.NextToken = &v
	return s
}

// SetStartDate sets the StartDate field's value.
func (s *QueryForecastInput) SetStartDate(v string) *QueryForecastInput {
	s.StartDate = &v
	return s
}

type QueryForecastOutput struct {
	_ struct{} `type:"structure"`

	// The forecast.
	Forecast *Forecast `type:"structure"`
}

// String returns the string representation
func (s QueryForecastOutput) String() string {
	return awsutil.Prettify(s)
}

// GoString returns the string representation
func (s QueryForecastOutput) GoString() string {
	return s.String()
}

// SetForecast sets the Forecast field's value.
func (s *QueryForecastOutput) SetForecast(v *Forecast) *QueryForecastOutput {
	s.Forecast = v
	return s
}

// The specified resource is in use.
type ResourceInUseException struct {
	_            struct{} `type:"structure"`
	respMetadata protocol.ResponseMetadata

	Message_ *string `locationName:"Message" type:"string"`
}

// String returns the string representation
func (s ResourceInUseException) String() string {
	return awsutil.Prettify(s)
}

// GoString returns the string representation
func (s ResourceInUseException) GoString() string {
	return s.String()
}

func newErrorResourceInUseException(v protocol.ResponseMetadata) error {
	return &ResourceInUseException{
		respMetadata: v,
	}
}

// Code returns the exception type name.
func (s ResourceInUseException) Code() string {
	return "ResourceInUseException"
}

// Message returns the exception's message.
func (s ResourceInUseException) Message() string {
	if s.Message_ != nil {
		return *s.Message_
	}
	return ""
}

// OrigErr always returns nil, satisfies awserr.Error interface.
func (s ResourceInUseException) OrigErr() error {
	return nil
}

func (s ResourceInUseException) Error() string {
	return fmt.Sprintf("%s: %s", s.Code(), s.Message())
}

// Status code returns the HTTP status code for the request's response error.
func (s ResourceInUseException) StatusCode() int {
	return s.respMetadata.StatusCode
}

// RequestID returns the service's response RequestID for request.
func (s ResourceInUseException) RequestID() string {
	return s.respMetadata.RequestID
}

// We can't find that resource. Check the information that you've provided and
// try again.
type ResourceNotFoundException struct {
	_            struct{} `type:"structure"`
	respMetadata protocol.ResponseMetadata

	Message_ *string `locationName:"Message" type:"string"`
}

// String returns the string representation
func (s ResourceNotFoundException) String() string {
	return awsutil.Prettify(s)
}

// GoString returns the string representation
func (s ResourceNotFoundException) GoString() string {
	return s.String()
}

func newErrorResourceNotFoundException(v protocol.ResponseMetadata) error {
	return &ResourceNotFoundException{
		respMetadata: v,
	}
}

// Code returns the exception type name.
func (s ResourceNotFoundException) Code() string {
	return "ResourceNotFoundException"
}

// Message returns the exception's message.
func (s ResourceNotFoundException) Message() string {
	if s.Message_ != nil {
		return *s.Message_
	}
	return ""
}

// OrigErr always returns nil, satisfies awserr.Error interface.
func (s ResourceNotFoundException) OrigErr() error {
	return nil
}

func (s ResourceNotFoundException) Error() string {
	return fmt.Sprintf("%s: %s", s.Code(), s.Message())
}

// Status code returns the HTTP status code for the request's response error.
func (s ResourceNotFoundException) StatusCode() int {
	return s.respMetadata.StatusCode
}

// RequestID returns the service's response RequestID for request.
func (s ResourceNotFoundException) RequestID() string {
	return s.respMetadata.RequestID
}
