// +build go1.9

// Copyright 2019 Microsoft Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This code was auto-generated by:
// github.com/Azure/azure-sdk-for-go/tools/profileBuilder

package customerinsights

import (
	"context"

	original "github.com/Azure/azure-sdk-for-go/services/customerinsights/mgmt/2017-04-26/customerinsights"
)

const (
	DefaultBaseURI = original.DefaultBaseURI
)

type CalculationWindowTypes = original.CalculationWindowTypes

const (
	Day      CalculationWindowTypes = original.Day
	Hour     CalculationWindowTypes = original.Hour
	Lifetime CalculationWindowTypes = original.Lifetime
	Month    CalculationWindowTypes = original.Month
	Week     CalculationWindowTypes = original.Week
)

type CanonicalPropertyValueType = original.CanonicalPropertyValueType

const (
	Categorical        CanonicalPropertyValueType = original.Categorical
	DerivedCategorical CanonicalPropertyValueType = original.DerivedCategorical
	DerivedNumeric     CanonicalPropertyValueType = original.DerivedNumeric
	Numeric            CanonicalPropertyValueType = original.Numeric
)

type CardinalityTypes = original.CardinalityTypes

const (
	ManyToMany CardinalityTypes = original.ManyToMany
	OneToMany  CardinalityTypes = original.OneToMany
	OneToOne   CardinalityTypes = original.OneToOne
)

type CompletionOperationTypes = original.CompletionOperationTypes

const (
	DeleteFile CompletionOperationTypes = original.DeleteFile
	DoNothing  CompletionOperationTypes = original.DoNothing
	MoveFile   CompletionOperationTypes = original.MoveFile
)

type ConnectorMappingStates = original.ConnectorMappingStates

const (
	Created  ConnectorMappingStates = original.Created
	Creating ConnectorMappingStates = original.Creating
	Expiring ConnectorMappingStates = original.Expiring
	Failed   ConnectorMappingStates = original.Failed
	Ready    ConnectorMappingStates = original.Ready
	Running  ConnectorMappingStates = original.Running
	Stopped  ConnectorMappingStates = original.Stopped
)

type ConnectorStates = original.ConnectorStates

const (
	ConnectorStatesCreated  ConnectorStates = original.ConnectorStatesCreated
	ConnectorStatesCreating ConnectorStates = original.ConnectorStatesCreating
	ConnectorStatesDeleting ConnectorStates = original.ConnectorStatesDeleting
	ConnectorStatesExpiring ConnectorStates = original.ConnectorStatesExpiring
	ConnectorStatesFailed   ConnectorStates = original.ConnectorStatesFailed
	ConnectorStatesReady    ConnectorStates = original.ConnectorStatesReady
)

type ConnectorTypes = original.ConnectorTypes

const (
	AzureBlob      ConnectorTypes = original.AzureBlob
	CRM            ConnectorTypes = original.CRM
	ExchangeOnline ConnectorTypes = original.ExchangeOnline
	None           ConnectorTypes = original.None
	Outbound       ConnectorTypes = original.Outbound
	Salesforce     ConnectorTypes = original.Salesforce
)

type DataSourceType = original.DataSourceType

const (
	DataSourceTypeConnector       DataSourceType = original.DataSourceTypeConnector
	DataSourceTypeLinkInteraction DataSourceType = original.DataSourceTypeLinkInteraction
	DataSourceTypeSystemDefault   DataSourceType = original.DataSourceTypeSystemDefault
)

type EntityType = original.EntityType

const (
	EntityTypeInteraction  EntityType = original.EntityTypeInteraction
	EntityTypeNone         EntityType = original.EntityTypeNone
	EntityTypeProfile      EntityType = original.EntityTypeProfile
	EntityTypeRelationship EntityType = original.EntityTypeRelationship
)

type EntityTypes = original.EntityTypes

const (
	EntityTypesInteraction  EntityTypes = original.EntityTypesInteraction
	EntityTypesNone         EntityTypes = original.EntityTypesNone
	EntityTypesProfile      EntityTypes = original.EntityTypesProfile
	EntityTypesRelationship EntityTypes = original.EntityTypesRelationship
)

type ErrorManagementTypes = original.ErrorManagementTypes

const (
	RejectAndContinue ErrorManagementTypes = original.RejectAndContinue
	RejectUntilLimit  ErrorManagementTypes = original.RejectUntilLimit
	StopImport        ErrorManagementTypes = original.StopImport
)

type FrequencyTypes = original.FrequencyTypes

const (
	FrequencyTypesDay    FrequencyTypes = original.FrequencyTypesDay
	FrequencyTypesHour   FrequencyTypes = original.FrequencyTypesHour
	FrequencyTypesMinute FrequencyTypes = original.FrequencyTypesMinute
	FrequencyTypesMonth  FrequencyTypes = original.FrequencyTypesMonth
	FrequencyTypesWeek   FrequencyTypes = original.FrequencyTypesWeek
)

type InstanceOperationType = original.InstanceOperationType

const (
	Delete InstanceOperationType = original.Delete
	Upsert InstanceOperationType = original.Upsert
)

type KpiFunctions = original.KpiFunctions

const (
	KpiFunctionsAvg           KpiFunctions = original.KpiFunctionsAvg
	KpiFunctionsCount         KpiFunctions = original.KpiFunctionsCount
	KpiFunctionsCountDistinct KpiFunctions = original.KpiFunctionsCountDistinct
	KpiFunctionsLast          KpiFunctions = original.KpiFunctionsLast
	KpiFunctionsMax           KpiFunctions = original.KpiFunctionsMax
	KpiFunctionsMin           KpiFunctions = original.KpiFunctionsMin
	KpiFunctionsNone          KpiFunctions = original.KpiFunctionsNone
	KpiFunctionsSum           KpiFunctions = original.KpiFunctionsSum
)

type LinkTypes = original.LinkTypes

const (
	CopyIfNull   LinkTypes = original.CopyIfNull
	UpdateAlways LinkTypes = original.UpdateAlways
)

type PermissionTypes = original.PermissionTypes

const (
	Manage PermissionTypes = original.Manage
	Read   PermissionTypes = original.Read
	Write  PermissionTypes = original.Write
)

type PredictionModelLifeCycle = original.PredictionModelLifeCycle

const (
	PredictionModelLifeCycleActive                   PredictionModelLifeCycle = original.PredictionModelLifeCycleActive
	PredictionModelLifeCycleDeleted                  PredictionModelLifeCycle = original.PredictionModelLifeCycleDeleted
	PredictionModelLifeCycleDiscovering              PredictionModelLifeCycle = original.PredictionModelLifeCycleDiscovering
	PredictionModelLifeCycleEvaluating               PredictionModelLifeCycle = original.PredictionModelLifeCycleEvaluating
	PredictionModelLifeCycleEvaluatingFailed         PredictionModelLifeCycle = original.PredictionModelLifeCycleEvaluatingFailed
	PredictionModelLifeCycleFailed                   PredictionModelLifeCycle = original.PredictionModelLifeCycleFailed
	PredictionModelLifeCycleFeaturing                PredictionModelLifeCycle = original.PredictionModelLifeCycleFeaturing
	PredictionModelLifeCycleFeaturingFailed          PredictionModelLifeCycle = original.PredictionModelLifeCycleFeaturingFailed
	PredictionModelLifeCycleHumanIntervention        PredictionModelLifeCycle = original.PredictionModelLifeCycleHumanIntervention
	PredictionModelLifeCycleNew                      PredictionModelLifeCycle = original.PredictionModelLifeCycleNew
	PredictionModelLifeCyclePendingDiscovering       PredictionModelLifeCycle = original.PredictionModelLifeCyclePendingDiscovering
	PredictionModelLifeCyclePendingFeaturing         PredictionModelLifeCycle = original.PredictionModelLifeCyclePendingFeaturing
	PredictionModelLifeCyclePendingModelConfirmation PredictionModelLifeCycle = original.PredictionModelLifeCyclePendingModelConfirmation
	PredictionModelLifeCyclePendingTraining          PredictionModelLifeCycle = original.PredictionModelLifeCyclePendingTraining
	PredictionModelLifeCycleProvisioning             PredictionModelLifeCycle = original.PredictionModelLifeCycleProvisioning
	PredictionModelLifeCycleProvisioningFailed       PredictionModelLifeCycle = original.PredictionModelLifeCycleProvisioningFailed
	PredictionModelLifeCycleTraining                 PredictionModelLifeCycle = original.PredictionModelLifeCycleTraining
	PredictionModelLifeCycleTrainingFailed           PredictionModelLifeCycle = original.PredictionModelLifeCycleTrainingFailed
)

type ProvisioningStates = original.ProvisioningStates

const (
	ProvisioningStatesDeleting          ProvisioningStates = original.ProvisioningStatesDeleting
	ProvisioningStatesExpiring          ProvisioningStates = original.ProvisioningStatesExpiring
	ProvisioningStatesFailed            ProvisioningStates = original.ProvisioningStatesFailed
	ProvisioningStatesHumanIntervention ProvisioningStates = original.ProvisioningStatesHumanIntervention
	ProvisioningStatesProvisioning      ProvisioningStates = original.ProvisioningStatesProvisioning
	ProvisioningStatesSucceeded         ProvisioningStates = original.ProvisioningStatesSucceeded
)

type RoleTypes = original.RoleTypes

const (
	Admin        RoleTypes = original.Admin
	DataAdmin    RoleTypes = original.DataAdmin
	DataReader   RoleTypes = original.DataReader
	ManageAdmin  RoleTypes = original.ManageAdmin
	ManageReader RoleTypes = original.ManageReader
	Reader       RoleTypes = original.Reader
)

type Status = original.Status

const (
	StatusActive  Status = original.StatusActive
	StatusDeleted Status = original.StatusDeleted
	StatusNone    Status = original.StatusNone
)

type AssignmentPrincipal = original.AssignmentPrincipal
type AuthorizationPoliciesClient = original.AuthorizationPoliciesClient
type AuthorizationPolicy = original.AuthorizationPolicy
type AuthorizationPolicyListResult = original.AuthorizationPolicyListResult
type AuthorizationPolicyListResultIterator = original.AuthorizationPolicyListResultIterator
type AuthorizationPolicyListResultPage = original.AuthorizationPolicyListResultPage
type AuthorizationPolicyResourceFormat = original.AuthorizationPolicyResourceFormat
type AzureBlobConnectorProperties = original.AzureBlobConnectorProperties
type BaseClient = original.BaseClient
type CanonicalProfileDefinition = original.CanonicalProfileDefinition
type CanonicalProfileDefinitionPropertiesItem = original.CanonicalProfileDefinitionPropertiesItem
type Connector = original.Connector
type ConnectorListResult = original.ConnectorListResult
type ConnectorListResultIterator = original.ConnectorListResultIterator
type ConnectorListResultPage = original.ConnectorListResultPage
type ConnectorMapping = original.ConnectorMapping
type ConnectorMappingAvailability = original.ConnectorMappingAvailability
type ConnectorMappingCompleteOperation = original.ConnectorMappingCompleteOperation
type ConnectorMappingErrorManagement = original.ConnectorMappingErrorManagement
type ConnectorMappingFormat = original.ConnectorMappingFormat
type ConnectorMappingListResult = original.ConnectorMappingListResult
type ConnectorMappingListResultIterator = original.ConnectorMappingListResultIterator
type ConnectorMappingListResultPage = original.ConnectorMappingListResultPage
type ConnectorMappingProperties = original.ConnectorMappingProperties
type ConnectorMappingResourceFormat = original.ConnectorMappingResourceFormat
type ConnectorMappingStructure = original.ConnectorMappingStructure
type ConnectorMappingsClient = original.ConnectorMappingsClient
type ConnectorResourceFormat = original.ConnectorResourceFormat
type ConnectorsClient = original.ConnectorsClient
type ConnectorsCreateOrUpdateFuture = original.ConnectorsCreateOrUpdateFuture
type ConnectorsDeleteFuture = original.ConnectorsDeleteFuture
type CrmConnectorEntities = original.CrmConnectorEntities
type CrmConnectorProperties = original.CrmConnectorProperties
type DataSource = original.DataSource
type DataSourcePrecedence = original.DataSourcePrecedence
type EnrichingKpi = original.EnrichingKpi
type EntityTypeDefinition = original.EntityTypeDefinition
type GetImageUploadURLInput = original.GetImageUploadURLInput
type Hub = original.Hub
type HubBillingInfoFormat = original.HubBillingInfoFormat
type HubListResult = original.HubListResult
type HubListResultIterator = original.HubListResultIterator
type HubListResultPage = original.HubListResultPage
type HubPropertiesFormat = original.HubPropertiesFormat
type HubsClient = original.HubsClient
type HubsDeleteFuture = original.HubsDeleteFuture
type ImageDefinition = original.ImageDefinition
type ImagesClient = original.ImagesClient
type InteractionListResult = original.InteractionListResult
type InteractionListResultIterator = original.InteractionListResultIterator
type InteractionListResultPage = original.InteractionListResultPage
type InteractionResourceFormat = original.InteractionResourceFormat
type InteractionTypeDefinition = original.InteractionTypeDefinition
type InteractionsClient = original.InteractionsClient
type InteractionsCreateOrUpdateFuture = original.InteractionsCreateOrUpdateFuture
type KpiAlias = original.KpiAlias
type KpiClient = original.KpiClient
type KpiCreateOrUpdateFuture = original.KpiCreateOrUpdateFuture
type KpiDefinition = original.KpiDefinition
type KpiDeleteFuture = original.KpiDeleteFuture
type KpiExtract = original.KpiExtract
type KpiGroupByMetadata = original.KpiGroupByMetadata
type KpiListResult = original.KpiListResult
type KpiListResultIterator = original.KpiListResultIterator
type KpiListResultPage = original.KpiListResultPage
type KpiParticipantProfilesMetadata = original.KpiParticipantProfilesMetadata
type KpiResourceFormat = original.KpiResourceFormat
type KpiThresholds = original.KpiThresholds
type LinkDefinition = original.LinkDefinition
type LinkListResult = original.LinkListResult
type LinkListResultIterator = original.LinkListResultIterator
type LinkListResultPage = original.LinkListResultPage
type LinkResourceFormat = original.LinkResourceFormat
type LinksClient = original.LinksClient
type LinksCreateOrUpdateFuture = original.LinksCreateOrUpdateFuture
type ListKpiDefinition = original.ListKpiDefinition
type MetadataDefinitionBase = original.MetadataDefinitionBase
type Operation = original.Operation
type OperationDisplay = original.OperationDisplay
type OperationListResult = original.OperationListResult
type OperationListResultIterator = original.OperationListResultIterator
type OperationListResultPage = original.OperationListResultPage
type OperationsClient = original.OperationsClient
type Participant = original.Participant
type ParticipantProfilePropertyReference = original.ParticipantProfilePropertyReference
type ParticipantPropertyReference = original.ParticipantPropertyReference
type Prediction = original.Prediction
type PredictionDistributionDefinition = original.PredictionDistributionDefinition
type PredictionDistributionDefinitionDistributionsItem = original.PredictionDistributionDefinitionDistributionsItem
type PredictionGradesItem = original.PredictionGradesItem
type PredictionListResult = original.PredictionListResult
type PredictionListResultIterator = original.PredictionListResultIterator
type PredictionListResultPage = original.PredictionListResultPage
type PredictionMappings = original.PredictionMappings
type PredictionModelStatus = original.PredictionModelStatus
type PredictionResourceFormat = original.PredictionResourceFormat
type PredictionSystemGeneratedEntities = original.PredictionSystemGeneratedEntities
type PredictionTrainingResults = original.PredictionTrainingResults
type PredictionsClient = original.PredictionsClient
type PredictionsCreateOrUpdateFuture = original.PredictionsCreateOrUpdateFuture
type PredictionsDeleteFuture = original.PredictionsDeleteFuture
type ProfileEnumValidValuesFormat = original.ProfileEnumValidValuesFormat
type ProfileListResult = original.ProfileListResult
type ProfileListResultIterator = original.ProfileListResultIterator
type ProfileListResultPage = original.ProfileListResultPage
type ProfileResourceFormat = original.ProfileResourceFormat
type ProfileTypeDefinition = original.ProfileTypeDefinition
type ProfilesClient = original.ProfilesClient
type ProfilesCreateOrUpdateFuture = original.ProfilesCreateOrUpdateFuture
type ProfilesDeleteFuture = original.ProfilesDeleteFuture
type PropertyDefinition = original.PropertyDefinition
type ProxyResource = original.ProxyResource
type RelationshipDefinition = original.RelationshipDefinition
type RelationshipLinkDefinition = original.RelationshipLinkDefinition
type RelationshipLinkFieldMapping = original.RelationshipLinkFieldMapping
type RelationshipLinkListResult = original.RelationshipLinkListResult
type RelationshipLinkListResultIterator = original.RelationshipLinkListResultIterator
type RelationshipLinkListResultPage = original.RelationshipLinkListResultPage
type RelationshipLinkResourceFormat = original.RelationshipLinkResourceFormat
type RelationshipLinksClient = original.RelationshipLinksClient
type RelationshipLinksCreateOrUpdateFuture = original.RelationshipLinksCreateOrUpdateFuture
type RelationshipLinksDeleteFuture = original.RelationshipLinksDeleteFuture
type RelationshipListResult = original.RelationshipListResult
type RelationshipListResultIterator = original.RelationshipListResultIterator
type RelationshipListResultPage = original.RelationshipListResultPage
type RelationshipResourceFormat = original.RelationshipResourceFormat
type RelationshipTypeFieldMapping = original.RelationshipTypeFieldMapping
type RelationshipTypeMapping = original.RelationshipTypeMapping
type RelationshipsClient = original.RelationshipsClient
type RelationshipsCreateOrUpdateFuture = original.RelationshipsCreateOrUpdateFuture
type RelationshipsDeleteFuture = original.RelationshipsDeleteFuture
type RelationshipsLookup = original.RelationshipsLookup
type Resource = original.Resource
type ResourceSetDescription = original.ResourceSetDescription
type Role = original.Role
type RoleAssignment = original.RoleAssignment
type RoleAssignmentListResult = original.RoleAssignmentListResult
type RoleAssignmentListResultIterator = original.RoleAssignmentListResultIterator
type RoleAssignmentListResultPage = original.RoleAssignmentListResultPage
type RoleAssignmentResourceFormat = original.RoleAssignmentResourceFormat
type RoleAssignmentsClient = original.RoleAssignmentsClient
type RoleAssignmentsCreateOrUpdateFuture = original.RoleAssignmentsCreateOrUpdateFuture
type RoleListResult = original.RoleListResult
type RoleListResultIterator = original.RoleListResultIterator
type RoleListResultPage = original.RoleListResultPage
type RoleResourceFormat = original.RoleResourceFormat
type RolesClient = original.RolesClient
type SalesforceConnectorProperties = original.SalesforceConnectorProperties
type SalesforceDiscoverSetting = original.SalesforceDiscoverSetting
type SalesforceTable = original.SalesforceTable
type StrongID = original.StrongID
type SuggestRelationshipLinksResponse = original.SuggestRelationshipLinksResponse
type TypePropertiesMapping = original.TypePropertiesMapping
type View = original.View
type ViewListResult = original.ViewListResult
type ViewListResultIterator = original.ViewListResultIterator
type ViewListResultPage = original.ViewListResultPage
type ViewResourceFormat = original.ViewResourceFormat
type ViewsClient = original.ViewsClient
type WidgetType = original.WidgetType
type WidgetTypeListResult = original.WidgetTypeListResult
type WidgetTypeListResultIterator = original.WidgetTypeListResultIterator
type WidgetTypeListResultPage = original.WidgetTypeListResultPage
type WidgetTypeResourceFormat = original.WidgetTypeResourceFormat
type WidgetTypesClient = original.WidgetTypesClient

func New(subscriptionID string) BaseClient {
	return original.New(subscriptionID)
}
func NewAuthorizationPoliciesClient(subscriptionID string) AuthorizationPoliciesClient {
	return original.NewAuthorizationPoliciesClient(subscriptionID)
}
func NewAuthorizationPoliciesClientWithBaseURI(baseURI string, subscriptionID string) AuthorizationPoliciesClient {
	return original.NewAuthorizationPoliciesClientWithBaseURI(baseURI, subscriptionID)
}
func NewAuthorizationPolicyListResultIterator(page AuthorizationPolicyListResultPage) AuthorizationPolicyListResultIterator {
	return original.NewAuthorizationPolicyListResultIterator(page)
}
func NewAuthorizationPolicyListResultPage(getNextPage func(context.Context, AuthorizationPolicyListResult) (AuthorizationPolicyListResult, error)) AuthorizationPolicyListResultPage {
	return original.NewAuthorizationPolicyListResultPage(getNextPage)
}
func NewConnectorListResultIterator(page ConnectorListResultPage) ConnectorListResultIterator {
	return original.NewConnectorListResultIterator(page)
}
func NewConnectorListResultPage(getNextPage func(context.Context, ConnectorListResult) (ConnectorListResult, error)) ConnectorListResultPage {
	return original.NewConnectorListResultPage(getNextPage)
}
func NewConnectorMappingListResultIterator(page ConnectorMappingListResultPage) ConnectorMappingListResultIterator {
	return original.NewConnectorMappingListResultIterator(page)
}
func NewConnectorMappingListResultPage(getNextPage func(context.Context, ConnectorMappingListResult) (ConnectorMappingListResult, error)) ConnectorMappingListResultPage {
	return original.NewConnectorMappingListResultPage(getNextPage)
}
func NewConnectorMappingsClient(subscriptionID string) ConnectorMappingsClient {
	return original.NewConnectorMappingsClient(subscriptionID)
}
func NewConnectorMappingsClientWithBaseURI(baseURI string, subscriptionID string) ConnectorMappingsClient {
	return original.NewConnectorMappingsClientWithBaseURI(baseURI, subscriptionID)
}
func NewConnectorsClient(subscriptionID string) ConnectorsClient {
	return original.NewConnectorsClient(subscriptionID)
}
func NewConnectorsClientWithBaseURI(baseURI string, subscriptionID string) ConnectorsClient {
	return original.NewConnectorsClientWithBaseURI(baseURI, subscriptionID)
}
func NewHubListResultIterator(page HubListResultPage) HubListResultIterator {
	return original.NewHubListResultIterator(page)
}
func NewHubListResultPage(getNextPage func(context.Context, HubListResult) (HubListResult, error)) HubListResultPage {
	return original.NewHubListResultPage(getNextPage)
}
func NewHubsClient(subscriptionID string) HubsClient {
	return original.NewHubsClient(subscriptionID)
}
func NewHubsClientWithBaseURI(baseURI string, subscriptionID string) HubsClient {
	return original.NewHubsClientWithBaseURI(baseURI, subscriptionID)
}
func NewImagesClient(subscriptionID string) ImagesClient {
	return original.NewImagesClient(subscriptionID)
}
func NewImagesClientWithBaseURI(baseURI string, subscriptionID string) ImagesClient {
	return original.NewImagesClientWithBaseURI(baseURI, subscriptionID)
}
func NewInteractionListResultIterator(page InteractionListResultPage) InteractionListResultIterator {
	return original.NewInteractionListResultIterator(page)
}
func NewInteractionListResultPage(getNextPage func(context.Context, InteractionListResult) (InteractionListResult, error)) InteractionListResultPage {
	return original.NewInteractionListResultPage(getNextPage)
}
func NewInteractionsClient(subscriptionID string) InteractionsClient {
	return original.NewInteractionsClient(subscriptionID)
}
func NewInteractionsClientWithBaseURI(baseURI string, subscriptionID string) InteractionsClient {
	return original.NewInteractionsClientWithBaseURI(baseURI, subscriptionID)
}
func NewKpiClient(subscriptionID string) KpiClient {
	return original.NewKpiClient(subscriptionID)
}
func NewKpiClientWithBaseURI(baseURI string, subscriptionID string) KpiClient {
	return original.NewKpiClientWithBaseURI(baseURI, subscriptionID)
}
func NewKpiListResultIterator(page KpiListResultPage) KpiListResultIterator {
	return original.NewKpiListResultIterator(page)
}
func NewKpiListResultPage(getNextPage func(context.Context, KpiListResult) (KpiListResult, error)) KpiListResultPage {
	return original.NewKpiListResultPage(getNextPage)
}
func NewLinkListResultIterator(page LinkListResultPage) LinkListResultIterator {
	return original.NewLinkListResultIterator(page)
}
func NewLinkListResultPage(getNextPage func(context.Context, LinkListResult) (LinkListResult, error)) LinkListResultPage {
	return original.NewLinkListResultPage(getNextPage)
}
func NewLinksClient(subscriptionID string) LinksClient {
	return original.NewLinksClient(subscriptionID)
}
func NewLinksClientWithBaseURI(baseURI string, subscriptionID string) LinksClient {
	return original.NewLinksClientWithBaseURI(baseURI, subscriptionID)
}
func NewOperationListResultIterator(page OperationListResultPage) OperationListResultIterator {
	return original.NewOperationListResultIterator(page)
}
func NewOperationListResultPage(getNextPage func(context.Context, OperationListResult) (OperationListResult, error)) OperationListResultPage {
	return original.NewOperationListResultPage(getNextPage)
}
func NewOperationsClient(subscriptionID string) OperationsClient {
	return original.NewOperationsClient(subscriptionID)
}
func NewOperationsClientWithBaseURI(baseURI string, subscriptionID string) OperationsClient {
	return original.NewOperationsClientWithBaseURI(baseURI, subscriptionID)
}
func NewPredictionListResultIterator(page PredictionListResultPage) PredictionListResultIterator {
	return original.NewPredictionListResultIterator(page)
}
func NewPredictionListResultPage(getNextPage func(context.Context, PredictionListResult) (PredictionListResult, error)) PredictionListResultPage {
	return original.NewPredictionListResultPage(getNextPage)
}
func NewPredictionsClient(subscriptionID string) PredictionsClient {
	return original.NewPredictionsClient(subscriptionID)
}
func NewPredictionsClientWithBaseURI(baseURI string, subscriptionID string) PredictionsClient {
	return original.NewPredictionsClientWithBaseURI(baseURI, subscriptionID)
}
func NewProfileListResultIterator(page ProfileListResultPage) ProfileListResultIterator {
	return original.NewProfileListResultIterator(page)
}
func NewProfileListResultPage(getNextPage func(context.Context, ProfileListResult) (ProfileListResult, error)) ProfileListResultPage {
	return original.NewProfileListResultPage(getNextPage)
}
func NewProfilesClient(subscriptionID string) ProfilesClient {
	return original.NewProfilesClient(subscriptionID)
}
func NewProfilesClientWithBaseURI(baseURI string, subscriptionID string) ProfilesClient {
	return original.NewProfilesClientWithBaseURI(baseURI, subscriptionID)
}
func NewRelationshipLinkListResultIterator(page RelationshipLinkListResultPage) RelationshipLinkListResultIterator {
	return original.NewRelationshipLinkListResultIterator(page)
}
func NewRelationshipLinkListResultPage(getNextPage func(context.Context, RelationshipLinkListResult) (RelationshipLinkListResult, error)) RelationshipLinkListResultPage {
	return original.NewRelationshipLinkListResultPage(getNextPage)
}
func NewRelationshipLinksClient(subscriptionID string) RelationshipLinksClient {
	return original.NewRelationshipLinksClient(subscriptionID)
}
func NewRelationshipLinksClientWithBaseURI(baseURI string, subscriptionID string) RelationshipLinksClient {
	return original.NewRelationshipLinksClientWithBaseURI(baseURI, subscriptionID)
}
func NewRelationshipListResultIterator(page RelationshipListResultPage) RelationshipListResultIterator {
	return original.NewRelationshipListResultIterator(page)
}
func NewRelationshipListResultPage(getNextPage func(context.Context, RelationshipListResult) (RelationshipListResult, error)) RelationshipListResultPage {
	return original.NewRelationshipListResultPage(getNextPage)
}
func NewRelationshipsClient(subscriptionID string) RelationshipsClient {
	return original.NewRelationshipsClient(subscriptionID)
}
func NewRelationshipsClientWithBaseURI(baseURI string, subscriptionID string) RelationshipsClient {
	return original.NewRelationshipsClientWithBaseURI(baseURI, subscriptionID)
}
func NewRoleAssignmentListResultIterator(page RoleAssignmentListResultPage) RoleAssignmentListResultIterator {
	return original.NewRoleAssignmentListResultIterator(page)
}
func NewRoleAssignmentListResultPage(getNextPage func(context.Context, RoleAssignmentListResult) (RoleAssignmentListResult, error)) RoleAssignmentListResultPage {
	return original.NewRoleAssignmentListResultPage(getNextPage)
}
func NewRoleAssignmentsClient(subscriptionID string) RoleAssignmentsClient {
	return original.NewRoleAssignmentsClient(subscriptionID)
}
func NewRoleAssignmentsClientWithBaseURI(baseURI string, subscriptionID string) RoleAssignmentsClient {
	return original.NewRoleAssignmentsClientWithBaseURI(baseURI, subscriptionID)
}
func NewRoleListResultIterator(page RoleListResultPage) RoleListResultIterator {
	return original.NewRoleListResultIterator(page)
}
func NewRoleListResultPage(getNextPage func(context.Context, RoleListResult) (RoleListResult, error)) RoleListResultPage {
	return original.NewRoleListResultPage(getNextPage)
}
func NewRolesClient(subscriptionID string) RolesClient {
	return original.NewRolesClient(subscriptionID)
}
func NewRolesClientWithBaseURI(baseURI string, subscriptionID string) RolesClient {
	return original.NewRolesClientWithBaseURI(baseURI, subscriptionID)
}
func NewViewListResultIterator(page ViewListResultPage) ViewListResultIterator {
	return original.NewViewListResultIterator(page)
}
func NewViewListResultPage(getNextPage func(context.Context, ViewListResult) (ViewListResult, error)) ViewListResultPage {
	return original.NewViewListResultPage(getNextPage)
}
func NewViewsClient(subscriptionID string) ViewsClient {
	return original.NewViewsClient(subscriptionID)
}
func NewViewsClientWithBaseURI(baseURI string, subscriptionID string) ViewsClient {
	return original.NewViewsClientWithBaseURI(baseURI, subscriptionID)
}
func NewWidgetTypeListResultIterator(page WidgetTypeListResultPage) WidgetTypeListResultIterator {
	return original.NewWidgetTypeListResultIterator(page)
}
func NewWidgetTypeListResultPage(getNextPage func(context.Context, WidgetTypeListResult) (WidgetTypeListResult, error)) WidgetTypeListResultPage {
	return original.NewWidgetTypeListResultPage(getNextPage)
}
func NewWidgetTypesClient(subscriptionID string) WidgetTypesClient {
	return original.NewWidgetTypesClient(subscriptionID)
}
func NewWidgetTypesClientWithBaseURI(baseURI string, subscriptionID string) WidgetTypesClient {
	return original.NewWidgetTypesClientWithBaseURI(baseURI, subscriptionID)
}
func NewWithBaseURI(baseURI string, subscriptionID string) BaseClient {
	return original.NewWithBaseURI(baseURI, subscriptionID)
}
func PossibleCalculationWindowTypesValues() []CalculationWindowTypes {
	return original.PossibleCalculationWindowTypesValues()
}
func PossibleCanonicalPropertyValueTypeValues() []CanonicalPropertyValueType {
	return original.PossibleCanonicalPropertyValueTypeValues()
}
func PossibleCardinalityTypesValues() []CardinalityTypes {
	return original.PossibleCardinalityTypesValues()
}
func PossibleCompletionOperationTypesValues() []CompletionOperationTypes {
	return original.PossibleCompletionOperationTypesValues()
}
func PossibleConnectorMappingStatesValues() []ConnectorMappingStates {
	return original.PossibleConnectorMappingStatesValues()
}
func PossibleConnectorStatesValues() []ConnectorStates {
	return original.PossibleConnectorStatesValues()
}
func PossibleConnectorTypesValues() []ConnectorTypes {
	return original.PossibleConnectorTypesValues()
}
func PossibleDataSourceTypeValues() []DataSourceType {
	return original.PossibleDataSourceTypeValues()
}
func PossibleEntityTypeValues() []EntityType {
	return original.PossibleEntityTypeValues()
}
func PossibleEntityTypesValues() []EntityTypes {
	return original.PossibleEntityTypesValues()
}
func PossibleErrorManagementTypesValues() []ErrorManagementTypes {
	return original.PossibleErrorManagementTypesValues()
}
func PossibleFrequencyTypesValues() []FrequencyTypes {
	return original.PossibleFrequencyTypesValues()
}
func PossibleInstanceOperationTypeValues() []InstanceOperationType {
	return original.PossibleInstanceOperationTypeValues()
}
func PossibleKpiFunctionsValues() []KpiFunctions {
	return original.PossibleKpiFunctionsValues()
}
func PossibleLinkTypesValues() []LinkTypes {
	return original.PossibleLinkTypesValues()
}
func PossiblePermissionTypesValues() []PermissionTypes {
	return original.PossiblePermissionTypesValues()
}
func PossiblePredictionModelLifeCycleValues() []PredictionModelLifeCycle {
	return original.PossiblePredictionModelLifeCycleValues()
}
func PossibleProvisioningStatesValues() []ProvisioningStates {
	return original.PossibleProvisioningStatesValues()
}
func PossibleRoleTypesValues() []RoleTypes {
	return original.PossibleRoleTypesValues()
}
func PossibleStatusValues() []Status {
	return original.PossibleStatusValues()
}
func UserAgent() string {
	return original.UserAgent() + " profiles/preview"
}
func Version() string {
	return original.Version()
}
