// Code generated by private/model/cli/gen-api/main.go. DO NOT EDIT.

package organizations

import (
	"github.com/aws/aws-sdk-go/private/protocol"
)

const (

	// ErrCodeAWSOrganizationsNotInUseException for service response error code
	// "AWSOrganizationsNotInUseException".
	//
	// Your account isn't a member of an organization. To make this request, you
	// must use the credentials of an account that belongs to an organization.
	ErrCodeAWSOrganizationsNotInUseException = "AWSOrganizationsNotInUseException"

	// ErrCodeAccessDeniedException for service response error code
	// "AccessDeniedException".
	//
	// You don't have permissions to perform the requested operation. The user or
	// role that is making the request must have at least one IAM permissions policy
	// attached that grants the required permissions. For more information, see
	// Access Management (https://docs.aws.amazon.com/IAM/latest/UserGuide/access.html)
	// in the IAM User Guide.
	ErrCodeAccessDeniedException = "AccessDeniedException"

	// ErrCodeAccessDeniedForDependencyException for service response error code
	// "AccessDeniedForDependencyException".
	//
	// The operation that you attempted requires you to have the iam:CreateServiceLinkedRole
	// for organizations.amazonaws.com permission so that AWS Organizations can
	// create the required service-linked role. You don't have that permission.
	ErrCodeAccessDeniedForDependencyException = "AccessDeniedForDependencyException"

	// ErrCodeAccountNotFoundException for service response error code
	// "AccountNotFoundException".
	//
	// We can't find an AWS account with the AccountId that you specified. Or the
	// account whose credentials you used to make this request isn't a member of
	// an organization.
	ErrCodeAccountNotFoundException = "AccountNotFoundException"

	// ErrCodeAccountOwnerNotVerifiedException for service response error code
	// "AccountOwnerNotVerifiedException".
	//
	// You can't invite an existing account to your organization until you verify
	// that you own the email address associated with the master account. For more
	// information, see Email Address Verification (http://docs.aws.amazon.com/organizations/latest/userguide/orgs_manage_create.html#about-email-verification)
	// in the AWS Organizations User Guide.
	ErrCodeAccountOwnerNotVerifiedException = "AccountOwnerNotVerifiedException"

	// ErrCodeAlreadyInOrganizationException for service response error code
	// "AlreadyInOrganizationException".
	//
	// This account is already a member of an organization. An account can belong
	// to only one organization at a time.
	ErrCodeAlreadyInOrganizationException = "AlreadyInOrganizationException"

	// ErrCodeChildNotFoundException for service response error code
	// "ChildNotFoundException".
	//
	// We can't find an organizational unit (OU) or AWS account with the ChildId
	// that you specified.
	ErrCodeChildNotFoundException = "ChildNotFoundException"

	// ErrCodeConcurrentModificationException for service response error code
	// "ConcurrentModificationException".
	//
	// The target of the operation is currently being modified by a different request.
	// Try again later.
	ErrCodeConcurrentModificationException = "ConcurrentModificationException"

	// ErrCodeConstraintViolationException for service response error code
	// "ConstraintViolationException".
	//
	// Performing this operation violates a minimum or maximum value limit. Examples
	// include attempting to remove the last service control policy (SCP) from an
	// OU or root, or attaching too many policies to an account, OU, or root. This
	// exception includes a reason that contains additional information about the
	// violated limit.
	//
	// Some of the reasons in the following list might not be applicable to this
	// specific API or operation:
	//
	//    * ACCOUNT_CANNOT_LEAVE_WITHOUT_EULA: You attempted to remove an account
	//    from the organization that doesn't yet have enough information to exist
	//    as a standalone account. This account requires you to first agree to the
	//    AWS Customer Agreement. Follow the steps at To leave an organization when
	//    all required account information has not yet been provided (http://docs.aws.amazon.com/organizations/latest/userguide/orgs_manage_accounts_remove.html#leave-without-all-info)
	//    in the AWS Organizations User Guide.
	//
	//    * ACCOUNT_CANNOT_LEAVE_WITHOUT_PHONE_VERIFICATION: You attempted to remove
	//    an account from the organization that doesn't yet have enough information
	//    to exist as a standalone account. This account requires you to first complete
	//    phone verification. Follow the steps at To leave an organization when
	//    all required account information has not yet been provided (http://docs.aws.amazon.com/organizations/latest/userguide/orgs_manage_accounts_remove.html#leave-without-all-info)
	//    in the AWS Organizations User Guide.
	//
	//    * ACCOUNT_CREATION_RATE_LIMIT_EXCEEDED: You attempted to exceed the number
	//    of accounts that you can create in one day.
	//
	//    * ACCOUNT_NUMBER_LIMIT_EXCEEDED: You attempted to exceed the limit on
	//    the number of accounts in an organization. If you need more accounts,
	//    contact AWS Support (https://console.aws.amazon.com/support/home#/) to
	//    request an increase in your limit. Or the number of invitations that you
	//    tried to send would cause you to exceed the limit of accounts in your
	//    organization. Send fewer invitations or contact AWS Support to request
	//    an increase in the number of accounts. Deleted and closed accounts still
	//    count toward your limit. If you get receive this exception when running
	//    a command immediately after creating the organization, wait one hour and
	//    try again. If after an hour it continues to fail with this error, contact
	//    AWS Support (https://console.aws.amazon.com/support/home#/).
	//
	//    * HANDSHAKE_RATE_LIMIT_EXCEEDED: You attempted to exceed the number of
	//    handshakes that you can send in one day.
	//
	//    * MASTER_ACCOUNT_ADDRESS_DOES_NOT_MATCH_MARKETPLACE: To create an account
	//    in this organization, you first must migrate the organization's master
	//    account to the marketplace that corresponds to the master account's address.
	//    For example, accounts with India addresses must be associated with the
	//    AISPL marketplace. All accounts in an organization must be associated
	//    with the same marketplace.
	//
	//    * MASTER_ACCOUNT_MISSING_CONTACT_INFO: To complete this operation, you
	//    must first provide contact a valid address and phone number for the master
	//    account. Then try the operation again.
	//
	//    * MASTER_ACCOUNT_NOT_GOVCLOUD_ENABLED: To complete this operation, the
	//    master account must have an associated account in the AWS GovCloud (US-West)
	//    Region. For more information, see AWS Organizations (http://docs.aws.amazon.com/govcloud-us/latest/UserGuide/govcloud-organizations.html)
	//    in the AWS GovCloud User Guide.
	//
	//    * MASTER_ACCOUNT_PAYMENT_INSTRUMENT_REQUIRED: To create an organization
	//    with this master account, you first must associate a valid payment instrument,
	//    such as a credit card, with the account. Follow the steps at To leave
	//    an organization when all required account information has not yet been
	//    provided (http://docs.aws.amazon.com/organizations/latest/userguide/orgs_manage_accounts_remove.html#leave-without-all-info)
	//    in the AWS Organizations User Guide.
	//
	//    * MAX_POLICY_TYPE_ATTACHMENT_LIMIT_EXCEEDED: You attempted to exceed the
	//    number of policies of a certain type that can be attached to an entity
	//    at one time.
	//
	//    * MAX_TAG_LIMIT_EXCEEDED: You have exceeded the number of tags allowed
	//    on this resource.
	//
	//    * MEMBER_ACCOUNT_PAYMENT_INSTRUMENT_REQUIRED: To complete this operation
	//    with this member account, you first must associate a valid payment instrument,
	//    such as a credit card, with the account. Follow the steps at To leave
	//    an organization when all required account information has not yet been
	//    provided (http://docs.aws.amazon.com/organizations/latest/userguide/orgs_manage_accounts_remove.html#leave-without-all-info)
	//    in the AWS Organizations User Guide.
	//
	//    * MIN_POLICY_TYPE_ATTACHMENT_LIMIT_EXCEEDED: You attempted to detach a
	//    policy from an entity, which would cause the entity to have fewer than
	//    the minimum number of policies of the required type.
	//
	//    * OU_DEPTH_LIMIT_EXCEEDED: You attempted to create an OU tree that is
	//    too many levels deep.
	//
	//    * ORGANIZATION_NOT_IN_ALL_FEATURES_MODE: You attempted to perform an operation
	//    that requires the organization to be configured to support all features.
	//    An organization that supports only consolidated billing features can't
	//    perform this operation.
	//
	//    * OU_NUMBER_LIMIT_EXCEEDED: You attempted to exceed the number of OUs
	//    that you can have in an organization.
	//
	//    * POLICY_NUMBER_LIMIT_EXCEEDED. You attempted to exceed the number of
	//    policies that you can have in an organization.
	ErrCodeConstraintViolationException = "ConstraintViolationException"

	// ErrCodeCreateAccountStatusNotFoundException for service response error code
	// "CreateAccountStatusNotFoundException".
	//
	// We can't find a create account request with the CreateAccountRequestId that
	// you specified.
	ErrCodeCreateAccountStatusNotFoundException = "CreateAccountStatusNotFoundException"

	// ErrCodeDestinationParentNotFoundException for service response error code
	// "DestinationParentNotFoundException".
	//
	// We can't find the destination container (a root or OU) with the ParentId
	// that you specified.
	ErrCodeDestinationParentNotFoundException = "DestinationParentNotFoundException"

	// ErrCodeDuplicateAccountException for service response error code
	// "DuplicateAccountException".
	//
	// That account is already present in the specified destination.
	ErrCodeDuplicateAccountException = "DuplicateAccountException"

	// ErrCodeDuplicateHandshakeException for service response error code
	// "DuplicateHandshakeException".
	//
	// A handshake with the same action and target already exists. For example,
	// if you invited an account to join your organization, the invited account
	// might already have a pending invitation from this organization. If you intend
	// to resend an invitation to an account, ensure that existing handshakes that
	// might be considered duplicates are canceled or declined.
	ErrCodeDuplicateHandshakeException = "DuplicateHandshakeException"

	// ErrCodeDuplicateOrganizationalUnitException for service response error code
	// "DuplicateOrganizationalUnitException".
	//
	// An OU with the same name already exists.
	ErrCodeDuplicateOrganizationalUnitException = "DuplicateOrganizationalUnitException"

	// ErrCodeDuplicatePolicyAttachmentException for service response error code
	// "DuplicatePolicyAttachmentException".
	//
	// The selected policy is already attached to the specified target.
	ErrCodeDuplicatePolicyAttachmentException = "DuplicatePolicyAttachmentException"

	// ErrCodeDuplicatePolicyException for service response error code
	// "DuplicatePolicyException".
	//
	// A policy with the same name already exists.
	ErrCodeDuplicatePolicyException = "DuplicatePolicyException"

	// ErrCodeEffectivePolicyNotFoundException for service response error code
	// "EffectivePolicyNotFoundException".
	//
	// If you ran this action on the master account, this policy type is not enabled.
	// If you ran the action on a member account, the account doesn't have an effective
	// policy of this type. Contact the administrator of your organization about
	// attaching a policy of this type to the account.
	ErrCodeEffectivePolicyNotFoundException = "EffectivePolicyNotFoundException"

	// ErrCodeFinalizingOrganizationException for service response error code
	// "FinalizingOrganizationException".
	//
	// AWS Organizations couldn't perform the operation because your organization
	// hasn't finished initializing. This can take up to an hour. Try again later.
	// If after one hour you continue to receive this error, contact AWS Support
	// (https://console.aws.amazon.com/support/home#/).
	ErrCodeFinalizingOrganizationException = "FinalizingOrganizationException"

	// ErrCodeHandshakeAlreadyInStateException for service response error code
	// "HandshakeAlreadyInStateException".
	//
	// The specified handshake is already in the requested state. For example, you
	// can't accept a handshake that was already accepted.
	ErrCodeHandshakeAlreadyInStateException = "HandshakeAlreadyInStateException"

	// ErrCodeHandshakeConstraintViolationException for service response error code
	// "HandshakeConstraintViolationException".
	//
	// The requested operation would violate the constraint identified in the reason
	// code.
	//
	// Some of the reasons in the following list might not be applicable to this
	// specific API or operation:
	//
	//    * ACCOUNT_NUMBER_LIMIT_EXCEEDED: You attempted to exceed the limit on
	//    the number of accounts in an organization. Note that deleted and closed
	//    accounts still count toward your limit. If you get this exception immediately
	//    after creating the organization, wait one hour and try again. If after
	//    an hour it continues to fail with this error, contact AWS Support (https://console.aws.amazon.com/support/home#/).
	//
	//    * ALREADY_IN_AN_ORGANIZATION: The handshake request is invalid because
	//    the invited account is already a member of an organization.
	//
	//    * HANDSHAKE_RATE_LIMIT_EXCEEDED: You attempted to exceed the number of
	//    handshakes that you can send in one day.
	//
	//    * INVITE_DISABLED_DURING_ENABLE_ALL_FEATURES: You can't issue new invitations
	//    to join an organization while it's in the process of enabling all features.
	//    You can resume inviting accounts after you finalize the process when all
	//    accounts have agreed to the change.
	//
	//    * ORGANIZATION_ALREADY_HAS_ALL_FEATURES: The handshake request is invalid
	//    because the organization has already enabled all features.
	//
	//    * ORGANIZATION_FROM_DIFFERENT_SELLER_OF_RECORD: The request failed because
	//    the account is from a different marketplace than the accounts in the organization.
	//    For example, accounts with India addresses must be associated with the
	//    AISPL marketplace. All accounts in an organization must be from the same
	//    marketplace.
	//
	//    * ORGANIZATION_MEMBERSHIP_CHANGE_RATE_LIMIT_EXCEEDED: You attempted to
	//    change the membership of an account too quickly after its previous change.
	//
	//    * PAYMENT_INSTRUMENT_REQUIRED: You can't complete the operation with an
	//    account that doesn't have a payment instrument, such as a credit card,
	//    associated with it.
	ErrCodeHandshakeConstraintViolationException = "HandshakeConstraintViolationException"

	// ErrCodeHandshakeNotFoundException for service response error code
	// "HandshakeNotFoundException".
	//
	// We can't find a handshake with the HandshakeId that you specified.
	ErrCodeHandshakeNotFoundException = "HandshakeNotFoundException"

	// ErrCodeInvalidHandshakeTransitionException for service response error code
	// "InvalidHandshakeTransitionException".
	//
	// You can't perform the operation on the handshake in its current state. For
	// example, you can't cancel a handshake that was already accepted or accept
	// a handshake that was already declined.
	ErrCodeInvalidHandshakeTransitionException = "InvalidHandshakeTransitionException"

	// ErrCodeInvalidInputException for service response error code
	// "InvalidInputException".
	//
	// The requested operation failed because you provided invalid values for one
	// or more of the request parameters. This exception includes a reason that
	// contains additional information about the violated limit:
	//
	// Some of the reasons in the following list might not be applicable to this
	// specific API or operation:
	//
	//    * IMMUTABLE_POLICY: You specified a policy that is managed by AWS and
	//    can't be modified.
	//
	//    * INPUT_REQUIRED: You must include a value for all required parameters.
	//
	//    * INVALID_ENUM: You specified an invalid value.
	//
	//    * INVALID_ENUM_POLICY_TYPE: You specified an invalid policy type.
	//
	//    * INVALID_FULL_NAME_TARGET: You specified a full name that contains invalid
	//    characters.
	//
	//    * INVALID_LIST_MEMBER: You provided a list to a parameter that contains
	//    at least one invalid value.
	//
	//    * INVALID_PAGINATION_TOKEN: Get the value for the NextToken parameter
	//    from the response to a previous call of the operation.
	//
	//    * INVALID_PARTY_TYPE_TARGET: You specified the wrong type of entity (account,
	//    organization, or email) as a party.
	//
	//    * INVALID_PATTERN: You provided a value that doesn't match the required
	//    pattern.
	//
	//    * INVALID_PATTERN_TARGET_ID: You specified a policy target ID that doesn't
	//    match the required pattern.
	//
	//    * INVALID_ROLE_NAME: You provided a role name that isn't valid. A role
	//    name can't begin with the reserved prefix AWSServiceRoleFor.
	//
	//    * INVALID_SYNTAX_ORGANIZATION_ARN: You specified an invalid Amazon Resource
	//    Name (ARN) for the organization.
	//
	//    * INVALID_SYNTAX_POLICY_ID: You specified an invalid policy ID.
	//
	//    * INVALID_SYSTEM_TAGS_PARAMETER: You specified a tag key that is a system
	//    tag. You can’t add, edit, or delete system tag keys because they're
	//    reserved for AWS use. System tags don’t count against your tags per
	//    resource limit.
	//
	//    * MAX_FILTER_LIMIT_EXCEEDED: You can specify only one filter parameter
	//    for the operation.
	//
	//    * MAX_LENGTH_EXCEEDED: You provided a string parameter that is longer
	//    than allowed.
	//
	//    * MAX_VALUE_EXCEEDED: You provided a numeric parameter that has a larger
	//    value than allowed.
	//
	//    * MIN_LENGTH_EXCEEDED: You provided a string parameter that is shorter
	//    than allowed.
	//
	//    * MIN_VALUE_EXCEEDED: You provided a numeric parameter that has a smaller
	//    value than allowed.
	//
	//    * MOVING_ACCOUNT_BETWEEN_DIFFERENT_ROOTS: You can move an account only
	//    between entities in the same root.
	ErrCodeInvalidInputException = "InvalidInputException"

	// ErrCodeMalformedPolicyDocumentException for service response error code
	// "MalformedPolicyDocumentException".
	//
	// The provided policy document doesn't meet the requirements of the specified
	// policy type. For example, the syntax might be incorrect. For details about
	// service control policy syntax, see Service Control Policy Syntax (https://docs.aws.amazon.com/organizations/latest/userguide/orgs_reference_scp-syntax.html)
	// in the AWS Organizations User Guide.
	ErrCodeMalformedPolicyDocumentException = "MalformedPolicyDocumentException"

	// ErrCodeMasterCannotLeaveOrganizationException for service response error code
	// "MasterCannotLeaveOrganizationException".
	//
	// You can't remove a master account from an organization. If you want the master
	// account to become a member account in another organization, you must first
	// delete the current organization of the master account.
	ErrCodeMasterCannotLeaveOrganizationException = "MasterCannotLeaveOrganizationException"

	// ErrCodeOrganizationNotEmptyException for service response error code
	// "OrganizationNotEmptyException".
	//
	// The organization isn't empty. To delete an organization, you must first remove
	// all accounts except the master account, delete all OUs, and delete all policies.
	ErrCodeOrganizationNotEmptyException = "OrganizationNotEmptyException"

	// ErrCodeOrganizationalUnitNotEmptyException for service response error code
	// "OrganizationalUnitNotEmptyException".
	//
	// The specified OU is not empty. Move all accounts to another root or to other
	// OUs, remove all child OUs, and try the operation again.
	ErrCodeOrganizationalUnitNotEmptyException = "OrganizationalUnitNotEmptyException"

	// ErrCodeOrganizationalUnitNotFoundException for service response error code
	// "OrganizationalUnitNotFoundException".
	//
	// We can't find an OU with the OrganizationalUnitId that you specified.
	ErrCodeOrganizationalUnitNotFoundException = "OrganizationalUnitNotFoundException"

	// ErrCodeParentNotFoundException for service response error code
	// "ParentNotFoundException".
	//
	// We can't find a root or OU with the ParentId that you specified.
	ErrCodeParentNotFoundException = "ParentNotFoundException"

	// ErrCodePolicyChangesInProgressException for service response error code
	// "PolicyChangesInProgressException".
	//
	// Changes to the effective policy are in progress, and its contents can't be
	// returned. Try the operation again later.
	ErrCodePolicyChangesInProgressException = "PolicyChangesInProgressException"

	// ErrCodePolicyInUseException for service response error code
	// "PolicyInUseException".
	//
	// The policy is attached to one or more entities. You must detach it from all
	// roots, OUs, and accounts before performing this operation.
	ErrCodePolicyInUseException = "PolicyInUseException"

	// ErrCodePolicyNotAttachedException for service response error code
	// "PolicyNotAttachedException".
	//
	// The policy isn't attached to the specified target in the specified root.
	ErrCodePolicyNotAttachedException = "PolicyNotAttachedException"

	// ErrCodePolicyNotFoundException for service response error code
	// "PolicyNotFoundException".
	//
	// We can't find a policy with the PolicyId that you specified.
	ErrCodePolicyNotFoundException = "PolicyNotFoundException"

	// ErrCodePolicyTypeAlreadyEnabledException for service response error code
	// "PolicyTypeAlreadyEnabledException".
	//
	// The specified policy type is already enabled in the specified root.
	ErrCodePolicyTypeAlreadyEnabledException = "PolicyTypeAlreadyEnabledException"

	// ErrCodePolicyTypeNotAvailableForOrganizationException for service response error code
	// "PolicyTypeNotAvailableForOrganizationException".
	//
	// You can't use the specified policy type with the feature set currently enabled
	// for this organization. For example, you can enable SCPs only after you enable
	// all features in the organization. For more information, see Enabling and
	// Disabling a Policy Type on a Root (https://docs.aws.amazon.com/organizations/latest/userguide/orgs_manage_policies.html#enable_policies_on_root)
	// in the AWS Organizations User Guide.
	ErrCodePolicyTypeNotAvailableForOrganizationException = "PolicyTypeNotAvailableForOrganizationException"

	// ErrCodePolicyTypeNotEnabledException for service response error code
	// "PolicyTypeNotEnabledException".
	//
	// The specified policy type isn't currently enabled in this root. You can't
	// attach policies of the specified type to entities in a root until you enable
	// that type in the root. For more information, see Enabling All Features in
	// Your Organization (https://docs.aws.amazon.com/organizations/latest/userguide/orgs_manage_org_support-all-features.html)
	// in the AWS Organizations User Guide.
	ErrCodePolicyTypeNotEnabledException = "PolicyTypeNotEnabledException"

	// ErrCodeRootNotFoundException for service response error code
	// "RootNotFoundException".
	//
	// We can't find a root with the RootId that you specified.
	ErrCodeRootNotFoundException = "RootNotFoundException"

	// ErrCodeServiceException for service response error code
	// "ServiceException".
	//
	// AWS Organizations can't complete your request because of an internal service
	// error. Try again later.
	ErrCodeServiceException = "ServiceException"

	// ErrCodeSourceParentNotFoundException for service response error code
	// "SourceParentNotFoundException".
	//
	// We can't find a source root or OU with the ParentId that you specified.
	ErrCodeSourceParentNotFoundException = "SourceParentNotFoundException"

	// ErrCodeTargetNotFoundException for service response error code
	// "TargetNotFoundException".
	//
	// We can't find a root, OU, or account with the TargetId that you specified.
	ErrCodeTargetNotFoundException = "TargetNotFoundException"

	// ErrCodeTooManyRequestsException for service response error code
	// "TooManyRequestsException".
	//
	// You have sent too many requests in too short a period of time. The limit
	// helps protect against denial-of-service attacks. Try again later.
	//
	// For information on limits that affect AWS Organizations, see Limits of AWS
	// Organizations (https://docs.aws.amazon.com/organizations/latest/userguide/orgs_reference_limits.html)
	// in the AWS Organizations User Guide.
	ErrCodeTooManyRequestsException = "TooManyRequestsException"

	// ErrCodeUnsupportedAPIEndpointException for service response error code
	// "UnsupportedAPIEndpointException".
	//
	// This action isn't available in the current Region.
	ErrCodeUnsupportedAPIEndpointException = "UnsupportedAPIEndpointException"
)

var exceptionFromCode = map[string]func(protocol.ResponseMetadata) error{
	"AWSOrganizationsNotInUseException":              newErrorAWSOrganizationsNotInUseException,
	"AccessDeniedException":                          newErrorAccessDeniedException,
	"AccessDeniedForDependencyException":             newErrorAccessDeniedForDependencyException,
	"AccountNotFoundException":                       newErrorAccountNotFoundException,
	"AccountOwnerNotVerifiedException":               newErrorAccountOwnerNotVerifiedException,
	"AlreadyInOrganizationException":                 newErrorAlreadyInOrganizationException,
	"ChildNotFoundException":                         newErrorChildNotFoundException,
	"ConcurrentModificationException":                newErrorConcurrentModificationException,
	"ConstraintViolationException":                   newErrorConstraintViolationException,
	"CreateAccountStatusNotFoundException":           newErrorCreateAccountStatusNotFoundException,
	"DestinationParentNotFoundException":             newErrorDestinationParentNotFoundException,
	"DuplicateAccountException":                      newErrorDuplicateAccountException,
	"DuplicateHandshakeException":                    newErrorDuplicateHandshakeException,
	"DuplicateOrganizationalUnitException":           newErrorDuplicateOrganizationalUnitException,
	"DuplicatePolicyAttachmentException":             newErrorDuplicatePolicyAttachmentException,
	"DuplicatePolicyException":                       newErrorDuplicatePolicyException,
	"EffectivePolicyNotFoundException":               newErrorEffectivePolicyNotFoundException,
	"FinalizingOrganizationException":                newErrorFinalizingOrganizationException,
	"HandshakeAlreadyInStateException":               newErrorHandshakeAlreadyInStateException,
	"HandshakeConstraintViolationException":          newErrorHandshakeConstraintViolationException,
	"HandshakeNotFoundException":                     newErrorHandshakeNotFoundException,
	"InvalidHandshakeTransitionException":            newErrorInvalidHandshakeTransitionException,
	"InvalidInputException":                          newErrorInvalidInputException,
	"MalformedPolicyDocumentException":               newErrorMalformedPolicyDocumentException,
	"MasterCannotLeaveOrganizationException":         newErrorMasterCannotLeaveOrganizationException,
	"OrganizationNotEmptyException":                  newErrorOrganizationNotEmptyException,
	"OrganizationalUnitNotEmptyException":            newErrorOrganizationalUnitNotEmptyException,
	"OrganizationalUnitNotFoundException":            newErrorOrganizationalUnitNotFoundException,
	"ParentNotFoundException":                        newErrorParentNotFoundException,
	"PolicyChangesInProgressException":               newErrorPolicyChangesInProgressException,
	"PolicyInUseException":                           newErrorPolicyInUseException,
	"PolicyNotAttachedException":                     newErrorPolicyNotAttachedException,
	"PolicyNotFoundException":                        newErrorPolicyNotFoundException,
	"PolicyTypeAlreadyEnabledException":              newErrorPolicyTypeAlreadyEnabledException,
	"PolicyTypeNotAvailableForOrganizationException": newErrorPolicyTypeNotAvailableForOrganizationException,
	"PolicyTypeNotEnabledException":                  newErrorPolicyTypeNotEnabledException,
	"RootNotFoundException":                          newErrorRootNotFoundException,
	"ServiceException":                               newErrorServiceException,
	"SourceParentNotFoundException":                  newErrorSourceParentNotFoundException,
	"TargetNotFoundException":                        newErrorTargetNotFoundException,
	"TooManyRequestsException":                       newErrorTooManyRequestsException,
	"UnsupportedAPIEndpointException":                newErrorUnsupportedAPIEndpointException,
}
