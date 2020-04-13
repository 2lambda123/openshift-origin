// Code generated by private/model/cli/gen-api/main.go. DO NOT EDIT.

package apigatewaymanagementapi

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awsutil"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/private/protocol"
	"github.com/aws/aws-sdk-go/private/protocol/restjson"
)

const opDeleteConnection = "DeleteConnection"

// DeleteConnectionRequest generates a "aws/request.Request" representing the
// client's request for the DeleteConnection operation. The "output" return
// value will be populated with the request's response once the request completes
// successfully.
//
// Use "Send" method on the returned Request to send the API call to the service.
// the "output" return value is not valid until after Send returns without error.
//
// See DeleteConnection for more information on using the DeleteConnection
// API call, and error handling.
//
// This method is useful when you want to inject custom logic or configuration
// into the SDK's request lifecycle. Such as custom headers, or retry logic.
//
//
//    // Example sending a request using the DeleteConnectionRequest method.
//    req, resp := client.DeleteConnectionRequest(params)
//
//    err := req.Send()
//    if err == nil { // resp is now filled
//        fmt.Println(resp)
//    }
//
// See also, https://docs.aws.amazon.com/goto/WebAPI/apigatewaymanagementapi-2018-11-29/DeleteConnection
func (c *ApiGatewayManagementApi) DeleteConnectionRequest(input *DeleteConnectionInput) (req *request.Request, output *DeleteConnectionOutput) {
	op := &request.Operation{
		Name:       opDeleteConnection,
		HTTPMethod: "DELETE",
		HTTPPath:   "/@connections/{connectionId}",
	}

	if input == nil {
		input = &DeleteConnectionInput{}
	}

	output = &DeleteConnectionOutput{}
	req = c.newRequest(op, input, output)
	req.Handlers.Unmarshal.Swap(restjson.UnmarshalHandler.Name, protocol.UnmarshalDiscardBodyHandler)
	return
}

// DeleteConnection API operation for AmazonApiGatewayManagementApi.
//
// Delete the connection with the provided id.
//
// Returns awserr.Error for service API and SDK errors. Use runtime type assertions
// with awserr.Error's Code and Message methods to get detailed information about
// the error.
//
// See the AWS API reference guide for AmazonApiGatewayManagementApi's
// API operation DeleteConnection for usage and error information.
//
// Returned Error Types:
//   * GoneException
//   The connection with the provided id no longer exists.
//
//   * LimitExceededException
//   The client is sending more than the allowed number of requests per unit of
//   time or the WebSocket client side buffer is full.
//
//   * ForbiddenException
//   The caller is not authorized to invoke this operation.
//
// See also, https://docs.aws.amazon.com/goto/WebAPI/apigatewaymanagementapi-2018-11-29/DeleteConnection
func (c *ApiGatewayManagementApi) DeleteConnection(input *DeleteConnectionInput) (*DeleteConnectionOutput, error) {
	req, out := c.DeleteConnectionRequest(input)
	return out, req.Send()
}

// DeleteConnectionWithContext is the same as DeleteConnection with the addition of
// the ability to pass a context and additional request options.
//
// See DeleteConnection for details on how to use this API operation.
//
// The context must be non-nil and will be used for request cancellation. If
// the context is nil a panic will occur. In the future the SDK may create
// sub-contexts for http.Requests. See https://golang.org/pkg/context/
// for more information on using Contexts.
func (c *ApiGatewayManagementApi) DeleteConnectionWithContext(ctx aws.Context, input *DeleteConnectionInput, opts ...request.Option) (*DeleteConnectionOutput, error) {
	req, out := c.DeleteConnectionRequest(input)
	req.SetContext(ctx)
	req.ApplyOptions(opts...)
	return out, req.Send()
}

const opGetConnection = "GetConnection"

// GetConnectionRequest generates a "aws/request.Request" representing the
// client's request for the GetConnection operation. The "output" return
// value will be populated with the request's response once the request completes
// successfully.
//
// Use "Send" method on the returned Request to send the API call to the service.
// the "output" return value is not valid until after Send returns without error.
//
// See GetConnection for more information on using the GetConnection
// API call, and error handling.
//
// This method is useful when you want to inject custom logic or configuration
// into the SDK's request lifecycle. Such as custom headers, or retry logic.
//
//
//    // Example sending a request using the GetConnectionRequest method.
//    req, resp := client.GetConnectionRequest(params)
//
//    err := req.Send()
//    if err == nil { // resp is now filled
//        fmt.Println(resp)
//    }
//
// See also, https://docs.aws.amazon.com/goto/WebAPI/apigatewaymanagementapi-2018-11-29/GetConnection
func (c *ApiGatewayManagementApi) GetConnectionRequest(input *GetConnectionInput) (req *request.Request, output *GetConnectionOutput) {
	op := &request.Operation{
		Name:       opGetConnection,
		HTTPMethod: "GET",
		HTTPPath:   "/@connections/{connectionId}",
	}

	if input == nil {
		input = &GetConnectionInput{}
	}

	output = &GetConnectionOutput{}
	req = c.newRequest(op, input, output)
	return
}

// GetConnection API operation for AmazonApiGatewayManagementApi.
//
// Get information about the connection with the provided id.
//
// Returns awserr.Error for service API and SDK errors. Use runtime type assertions
// with awserr.Error's Code and Message methods to get detailed information about
// the error.
//
// See the AWS API reference guide for AmazonApiGatewayManagementApi's
// API operation GetConnection for usage and error information.
//
// Returned Error Types:
//   * GoneException
//   The connection with the provided id no longer exists.
//
//   * LimitExceededException
//   The client is sending more than the allowed number of requests per unit of
//   time or the WebSocket client side buffer is full.
//
//   * ForbiddenException
//   The caller is not authorized to invoke this operation.
//
// See also, https://docs.aws.amazon.com/goto/WebAPI/apigatewaymanagementapi-2018-11-29/GetConnection
func (c *ApiGatewayManagementApi) GetConnection(input *GetConnectionInput) (*GetConnectionOutput, error) {
	req, out := c.GetConnectionRequest(input)
	return out, req.Send()
}

// GetConnectionWithContext is the same as GetConnection with the addition of
// the ability to pass a context and additional request options.
//
// See GetConnection for details on how to use this API operation.
//
// The context must be non-nil and will be used for request cancellation. If
// the context is nil a panic will occur. In the future the SDK may create
// sub-contexts for http.Requests. See https://golang.org/pkg/context/
// for more information on using Contexts.
func (c *ApiGatewayManagementApi) GetConnectionWithContext(ctx aws.Context, input *GetConnectionInput, opts ...request.Option) (*GetConnectionOutput, error) {
	req, out := c.GetConnectionRequest(input)
	req.SetContext(ctx)
	req.ApplyOptions(opts...)
	return out, req.Send()
}

const opPostToConnection = "PostToConnection"

// PostToConnectionRequest generates a "aws/request.Request" representing the
// client's request for the PostToConnection operation. The "output" return
// value will be populated with the request's response once the request completes
// successfully.
//
// Use "Send" method on the returned Request to send the API call to the service.
// the "output" return value is not valid until after Send returns without error.
//
// See PostToConnection for more information on using the PostToConnection
// API call, and error handling.
//
// This method is useful when you want to inject custom logic or configuration
// into the SDK's request lifecycle. Such as custom headers, or retry logic.
//
//
//    // Example sending a request using the PostToConnectionRequest method.
//    req, resp := client.PostToConnectionRequest(params)
//
//    err := req.Send()
//    if err == nil { // resp is now filled
//        fmt.Println(resp)
//    }
//
// See also, https://docs.aws.amazon.com/goto/WebAPI/apigatewaymanagementapi-2018-11-29/PostToConnection
func (c *ApiGatewayManagementApi) PostToConnectionRequest(input *PostToConnectionInput) (req *request.Request, output *PostToConnectionOutput) {
	op := &request.Operation{
		Name:       opPostToConnection,
		HTTPMethod: "POST",
		HTTPPath:   "/@connections/{connectionId}",
	}

	if input == nil {
		input = &PostToConnectionInput{}
	}

	output = &PostToConnectionOutput{}
	req = c.newRequest(op, input, output)
	req.Handlers.Unmarshal.Swap(restjson.UnmarshalHandler.Name, protocol.UnmarshalDiscardBodyHandler)
	return
}

// PostToConnection API operation for AmazonApiGatewayManagementApi.
//
// Sends the provided data to the specified connection.
//
// Returns awserr.Error for service API and SDK errors. Use runtime type assertions
// with awserr.Error's Code and Message methods to get detailed information about
// the error.
//
// See the AWS API reference guide for AmazonApiGatewayManagementApi's
// API operation PostToConnection for usage and error information.
//
// Returned Error Types:
//   * GoneException
//   The connection with the provided id no longer exists.
//
//   * LimitExceededException
//   The client is sending more than the allowed number of requests per unit of
//   time or the WebSocket client side buffer is full.
//
//   * PayloadTooLargeException
//   The data has exceeded the maximum size allowed.
//
//   * ForbiddenException
//   The caller is not authorized to invoke this operation.
//
// See also, https://docs.aws.amazon.com/goto/WebAPI/apigatewaymanagementapi-2018-11-29/PostToConnection
func (c *ApiGatewayManagementApi) PostToConnection(input *PostToConnectionInput) (*PostToConnectionOutput, error) {
	req, out := c.PostToConnectionRequest(input)
	return out, req.Send()
}

// PostToConnectionWithContext is the same as PostToConnection with the addition of
// the ability to pass a context and additional request options.
//
// See PostToConnection for details on how to use this API operation.
//
// The context must be non-nil and will be used for request cancellation. If
// the context is nil a panic will occur. In the future the SDK may create
// sub-contexts for http.Requests. See https://golang.org/pkg/context/
// for more information on using Contexts.
func (c *ApiGatewayManagementApi) PostToConnectionWithContext(ctx aws.Context, input *PostToConnectionInput, opts ...request.Option) (*PostToConnectionOutput, error) {
	req, out := c.PostToConnectionRequest(input)
	req.SetContext(ctx)
	req.ApplyOptions(opts...)
	return out, req.Send()
}

type DeleteConnectionInput struct {
	_ struct{} `type:"structure"`

	// ConnectionId is a required field
	ConnectionId *string `location:"uri" locationName:"connectionId" type:"string" required:"true"`
}

// String returns the string representation
func (s DeleteConnectionInput) String() string {
	return awsutil.Prettify(s)
}

// GoString returns the string representation
func (s DeleteConnectionInput) GoString() string {
	return s.String()
}

// Validate inspects the fields of the type to determine if they are valid.
func (s *DeleteConnectionInput) Validate() error {
	invalidParams := request.ErrInvalidParams{Context: "DeleteConnectionInput"}
	if s.ConnectionId == nil {
		invalidParams.Add(request.NewErrParamRequired("ConnectionId"))
	}
	if s.ConnectionId != nil && len(*s.ConnectionId) < 1 {
		invalidParams.Add(request.NewErrParamMinLen("ConnectionId", 1))
	}

	if invalidParams.Len() > 0 {
		return invalidParams
	}
	return nil
}

// SetConnectionId sets the ConnectionId field's value.
func (s *DeleteConnectionInput) SetConnectionId(v string) *DeleteConnectionInput {
	s.ConnectionId = &v
	return s
}

type DeleteConnectionOutput struct {
	_ struct{} `type:"structure"`
}

// String returns the string representation
func (s DeleteConnectionOutput) String() string {
	return awsutil.Prettify(s)
}

// GoString returns the string representation
func (s DeleteConnectionOutput) GoString() string {
	return s.String()
}

// The caller is not authorized to invoke this operation.
type ForbiddenException struct {
	_            struct{} `type:"structure"`
	respMetadata protocol.ResponseMetadata
}

// String returns the string representation
func (s ForbiddenException) String() string {
	return awsutil.Prettify(s)
}

// GoString returns the string representation
func (s ForbiddenException) GoString() string {
	return s.String()
}

func newErrorForbiddenException(v protocol.ResponseMetadata) error {
	return &ForbiddenException{
		respMetadata: v,
	}
}

// Code returns the exception type name.
func (s ForbiddenException) Code() string {
	return "ForbiddenException"
}

// Message returns the exception's message.
func (s ForbiddenException) Message() string {
	return ""
}

// OrigErr always returns nil, satisfies awserr.Error interface.
func (s ForbiddenException) OrigErr() error {
	return nil
}

func (s ForbiddenException) Error() string {
	return fmt.Sprintf("%s: %s", s.Code(), s.Message())
}

// Status code returns the HTTP status code for the request's response error.
func (s ForbiddenException) StatusCode() int {
	return s.respMetadata.StatusCode
}

// RequestID returns the service's response RequestID for request.
func (s ForbiddenException) RequestID() string {
	return s.respMetadata.RequestID
}

type GetConnectionInput struct {
	_ struct{} `type:"structure"`

	// ConnectionId is a required field
	ConnectionId *string `location:"uri" locationName:"connectionId" type:"string" required:"true"`
}

// String returns the string representation
func (s GetConnectionInput) String() string {
	return awsutil.Prettify(s)
}

// GoString returns the string representation
func (s GetConnectionInput) GoString() string {
	return s.String()
}

// Validate inspects the fields of the type to determine if they are valid.
func (s *GetConnectionInput) Validate() error {
	invalidParams := request.ErrInvalidParams{Context: "GetConnectionInput"}
	if s.ConnectionId == nil {
		invalidParams.Add(request.NewErrParamRequired("ConnectionId"))
	}
	if s.ConnectionId != nil && len(*s.ConnectionId) < 1 {
		invalidParams.Add(request.NewErrParamMinLen("ConnectionId", 1))
	}

	if invalidParams.Len() > 0 {
		return invalidParams
	}
	return nil
}

// SetConnectionId sets the ConnectionId field's value.
func (s *GetConnectionInput) SetConnectionId(v string) *GetConnectionInput {
	s.ConnectionId = &v
	return s
}

type GetConnectionOutput struct {
	_ struct{} `type:"structure"`

	ConnectedAt *time.Time `locationName:"connectedAt" type:"timestamp" timestampFormat:"iso8601"`

	Identity *Identity `locationName:"identity" type:"structure"`

	LastActiveAt *time.Time `locationName:"lastActiveAt" type:"timestamp" timestampFormat:"iso8601"`
}

// String returns the string representation
func (s GetConnectionOutput) String() string {
	return awsutil.Prettify(s)
}

// GoString returns the string representation
func (s GetConnectionOutput) GoString() string {
	return s.String()
}

// SetConnectedAt sets the ConnectedAt field's value.
func (s *GetConnectionOutput) SetConnectedAt(v time.Time) *GetConnectionOutput {
	s.ConnectedAt = &v
	return s
}

// SetIdentity sets the Identity field's value.
func (s *GetConnectionOutput) SetIdentity(v *Identity) *GetConnectionOutput {
	s.Identity = v
	return s
}

// SetLastActiveAt sets the LastActiveAt field's value.
func (s *GetConnectionOutput) SetLastActiveAt(v time.Time) *GetConnectionOutput {
	s.LastActiveAt = &v
	return s
}

// The connection with the provided id no longer exists.
type GoneException struct {
	_            struct{} `type:"structure"`
	respMetadata protocol.ResponseMetadata
}

// String returns the string representation
func (s GoneException) String() string {
	return awsutil.Prettify(s)
}

// GoString returns the string representation
func (s GoneException) GoString() string {
	return s.String()
}

func newErrorGoneException(v protocol.ResponseMetadata) error {
	return &GoneException{
		respMetadata: v,
	}
}

// Code returns the exception type name.
func (s GoneException) Code() string {
	return "GoneException"
}

// Message returns the exception's message.
func (s GoneException) Message() string {
	return ""
}

// OrigErr always returns nil, satisfies awserr.Error interface.
func (s GoneException) OrigErr() error {
	return nil
}

func (s GoneException) Error() string {
	return fmt.Sprintf("%s: %s", s.Code(), s.Message())
}

// Status code returns the HTTP status code for the request's response error.
func (s GoneException) StatusCode() int {
	return s.respMetadata.StatusCode
}

// RequestID returns the service's response RequestID for request.
func (s GoneException) RequestID() string {
	return s.respMetadata.RequestID
}

type Identity struct {
	_ struct{} `type:"structure"`

	// The source IP address of the TCP connection making the request to API Gateway.
	//
	// SourceIp is a required field
	SourceIp *string `locationName:"sourceIp" type:"string" required:"true"`

	// The User Agent of the API caller.
	//
	// UserAgent is a required field
	UserAgent *string `locationName:"userAgent" type:"string" required:"true"`
}

// String returns the string representation
func (s Identity) String() string {
	return awsutil.Prettify(s)
}

// GoString returns the string representation
func (s Identity) GoString() string {
	return s.String()
}

// SetSourceIp sets the SourceIp field's value.
func (s *Identity) SetSourceIp(v string) *Identity {
	s.SourceIp = &v
	return s
}

// SetUserAgent sets the UserAgent field's value.
func (s *Identity) SetUserAgent(v string) *Identity {
	s.UserAgent = &v
	return s
}

// The client is sending more than the allowed number of requests per unit of
// time or the WebSocket client side buffer is full.
type LimitExceededException struct {
	_            struct{} `type:"structure"`
	respMetadata protocol.ResponseMetadata
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

// The data has exceeded the maximum size allowed.
type PayloadTooLargeException struct {
	_            struct{} `type:"structure"`
	respMetadata protocol.ResponseMetadata

	Message_ *string `locationName:"message" type:"string"`
}

// String returns the string representation
func (s PayloadTooLargeException) String() string {
	return awsutil.Prettify(s)
}

// GoString returns the string representation
func (s PayloadTooLargeException) GoString() string {
	return s.String()
}

func newErrorPayloadTooLargeException(v protocol.ResponseMetadata) error {
	return &PayloadTooLargeException{
		respMetadata: v,
	}
}

// Code returns the exception type name.
func (s PayloadTooLargeException) Code() string {
	return "PayloadTooLargeException"
}

// Message returns the exception's message.
func (s PayloadTooLargeException) Message() string {
	if s.Message_ != nil {
		return *s.Message_
	}
	return ""
}

// OrigErr always returns nil, satisfies awserr.Error interface.
func (s PayloadTooLargeException) OrigErr() error {
	return nil
}

func (s PayloadTooLargeException) Error() string {
	return fmt.Sprintf("%s: %s", s.Code(), s.Message())
}

// Status code returns the HTTP status code for the request's response error.
func (s PayloadTooLargeException) StatusCode() int {
	return s.respMetadata.StatusCode
}

// RequestID returns the service's response RequestID for request.
func (s PayloadTooLargeException) RequestID() string {
	return s.respMetadata.RequestID
}

type PostToConnectionInput struct {
	_ struct{} `type:"structure" payload:"Data"`

	// ConnectionId is a required field
	ConnectionId *string `location:"uri" locationName:"connectionId" type:"string" required:"true"`

	// The data to be sent to the client specified by its connection id.
	//
	// Data is a required field
	Data []byte `type:"blob" required:"true"`
}

// String returns the string representation
func (s PostToConnectionInput) String() string {
	return awsutil.Prettify(s)
}

// GoString returns the string representation
func (s PostToConnectionInput) GoString() string {
	return s.String()
}

// Validate inspects the fields of the type to determine if they are valid.
func (s *PostToConnectionInput) Validate() error {
	invalidParams := request.ErrInvalidParams{Context: "PostToConnectionInput"}
	if s.ConnectionId == nil {
		invalidParams.Add(request.NewErrParamRequired("ConnectionId"))
	}
	if s.ConnectionId != nil && len(*s.ConnectionId) < 1 {
		invalidParams.Add(request.NewErrParamMinLen("ConnectionId", 1))
	}
	if s.Data == nil {
		invalidParams.Add(request.NewErrParamRequired("Data"))
	}

	if invalidParams.Len() > 0 {
		return invalidParams
	}
	return nil
}

// SetConnectionId sets the ConnectionId field's value.
func (s *PostToConnectionInput) SetConnectionId(v string) *PostToConnectionInput {
	s.ConnectionId = &v
	return s
}

// SetData sets the Data field's value.
func (s *PostToConnectionInput) SetData(v []byte) *PostToConnectionInput {
	s.Data = v
	return s
}

type PostToConnectionOutput struct {
	_ struct{} `type:"structure"`
}

// String returns the string representation
func (s PostToConnectionOutput) String() string {
	return awsutil.Prettify(s)
}

// GoString returns the string representation
func (s PostToConnectionOutput) GoString() string {
	return s.String()
}
