// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package notificationhub

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-helpers/lang/response"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/commonschema"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/location"
	"github.com/hashicorp/go-azure-sdk/resource-manager/notificationhubs/2023-09-01/hubs"
	"github.com/hashicorp/terraform-provider-azurerm/helpers/tf"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/notificationhub/migration"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/validation"
)

type NotificationHubResource struct{}

type NotificationHubModel struct {
	Name              string                                  `tfschema:"name"`
	NamespaceName     string                                  `tfschema:"namespace_name"`
	ResourceGroupName string                                  `tfschema:"resource_group_name"`
	Location          string                                  `tfschema:"location"`
	AdmCredential     []NotificationHubAdmCredentialModel     `tfschema:"adm_credential"`
	ApnsCredential    []NotificationHubApnsCredentialModel    `tfschema:"apns_credential"`
	BrowserCredential []NotificationHubBrowserCredentialModel `tfschema:"browser_credential"`
	GcmCredential     []NotificationHubGcmCredentialModel     `tfschema:"gcm_credential"`
	Tags              map[string]string                       `tfschema:"tags"`
}

type NotificationHubAdmCredentialModel struct {
	AuthTokenUrl string `tfschema:"auth_token_url"`
	ClientId     string `tfschema:"client_id"`
	ClientSecret string `tfschema:"client_secret"`
}

type NotificationHubApnsCredentialModel struct {
	ApplicationMode string `tfschema:"application_mode"`
	BundleId        string `tfschema:"bundle_id"`
	KeyId           string `tfschema:"key_id"`
	TeamId          string `tfschema:"team_id"`
	Token           string `tfschema:"token"`
}

type NotificationHubBrowserCredentialModel struct {
	Subject         string `tfschema:"subject"`
	VapidPrivateKey string `tfschema:"vapid_private_key"`
	VapidPublicKey  string `tfschema:"vapid_public_key"`
}

type NotificationHubGcmCredentialModel struct {
	ApiKey string `tfschema:"api_key"`
}

var (
	_ sdk.ResourceWithUpdate         = NotificationHubResource{}
	_ sdk.ResourceWithStateMigration = NotificationHubResource{}
	_ sdk.ResourceWithCustomizeDiff  = NotificationHubResource{}
)

func (r NotificationHubResource) ResourceType() string {
	return "azurerm_notification_hub"
}

func (r NotificationHubResource) ModelObject() interface{} {
	return &NotificationHubModel{}
}

func (r NotificationHubResource) StateUpgraders() sdk.StateUpgradeData {
	return sdk.StateUpgradeData{
		SchemaVersion: 1,
		Upgraders: map[int]pluginsdk.StateUpgrade{
			0: migration.NotificationHubResourceV0ToV1{},
		},
	}
}

func (r NotificationHubResource) IDValidationFunc() pluginsdk.SchemaValidateFunc {
	return hubs.ValidateNotificationHubID
}

func (r NotificationHubResource) Arguments() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"name": {
			Type:     pluginsdk.TypeString,
			Required: true,
			ForceNew: true,
		},

		"namespace_name": {
			Type:     pluginsdk.TypeString,
			Required: true,
			ForceNew: true,
		},

		"resource_group_name": commonschema.ResourceGroupName(),

		"location": commonschema.Location(),

		"adm_credential": {
			Type:     pluginsdk.TypeList,
			Optional: true,
			ForceNew: true,
			MaxItems: 1,
			Elem: &pluginsdk.Resource{
				Schema: map[string]*pluginsdk.Schema{
					"auth_token_url": {
						Type:         pluginsdk.TypeString,
						Required:     true,
						ValidateFunc: validation.StringIsNotEmpty,
					},
					"client_id": {
						Type:         pluginsdk.TypeString,
						Required:     true,
						ValidateFunc: validation.StringIsNotEmpty,
					},
					"client_secret": {
						Type:         pluginsdk.TypeString,
						Required:     true,
						ValidateFunc: validation.StringIsNotEmpty,
						Sensitive:    true,
					},
				},
			},
		},

		"apns_credential": {
			Type:     pluginsdk.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &pluginsdk.Resource{
				Schema: map[string]*pluginsdk.Schema{
					// NOTE: APNS supports two modes, certificate auth (v1) and token auth (v2)
					// certificate authentication/v1 is marked for deprecation; as such we're not
					// supporting it at this time.
					"application_mode": {
						Type:     pluginsdk.TypeString,
						Required: true,
						ValidateFunc: validation.StringInSlice([]string{
							apnsProductionName,
							apnsSandboxName,
						}, false),
					},
					"bundle_id": {
						Type:     pluginsdk.TypeString,
						Required: true,
					},
					"key_id": {
						Type:     pluginsdk.TypeString,
						Required: true,
					},
					// Team ID (within Apple & the Portal) == "AppID" (within the API)
					"team_id": {
						Type:     pluginsdk.TypeString,
						Required: true,
					},
					"token": {
						Type:      pluginsdk.TypeString,
						Required:  true,
						Sensitive: true,
					},
				},
			},
		},

		"browser_credential": {
			Type:     pluginsdk.TypeList,
			Optional: true,
			ForceNew: true,
			MaxItems: 1,
			Elem: &pluginsdk.Resource{
				Schema: map[string]*pluginsdk.Schema{
					"subject": {
						Type:         pluginsdk.TypeString,
						Required:     true,
						ValidateFunc: validation.StringIsNotEmpty,
					},
					"vapid_private_key": {
						Type:         pluginsdk.TypeString,
						Required:     true,
						ValidateFunc: validation.StringIsNotEmpty,
						Sensitive:    true,
					},
					"vapid_public_key": {
						Type:         pluginsdk.TypeString,
						Required:     true,
						ValidateFunc: validation.StringIsNotEmpty,
					},
				},
			},
		},

		"gcm_credential": {
			Type:     pluginsdk.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &pluginsdk.Resource{
				Schema: map[string]*pluginsdk.Schema{
					"api_key": {
						Type:      pluginsdk.TypeString,
						Required:  true,
						Sensitive: true,
					},
				},
			},
		},

		"tags": commonschema.Tags(),
	}
}

func (r NotificationHubResource) Attributes() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{}
}

func (r NotificationHubResource) CustomizeDiff() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 5 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			// NOTE: the ForceNew is to workaround a bug in the Azure SDK where nil-values aren't sent to the API.
			// Bug: https://github.com/Azure/azure-sdk-for-go/issues/2246

			diff := metadata.ResourceDiff
			oADM, nADM := diff.GetChange("adm_credential.#")
			oADMi := oADM.(int)
			nADMi := nADM.(int)
			if nADMi < oADMi {
				diff.ForceNew("adm_credential")
			}

			oAPNS, nAPNS := diff.GetChange("apns_credential.#")
			oAPNSi := oAPNS.(int)
			nAPNSi := nAPNS.(int)
			if nAPNSi < oAPNSi {
				diff.ForceNew("apns_credential")
			}

			oGCM, nGCM := diff.GetChange("gcm_credential.#")
			oGCMi := oGCM.(int)
			nGCMi := nGCM.(int)
			if nGCMi < oGCMi {
				diff.ForceNew("gcm_credential")
			}

			return nil
		},
	}
}

const (
	apnsProductionName     = "Production"
	apnsProductionEndpoint = "https://api.push.apple.com:443/3/device"
	apnsSandboxName        = "Sandbox"
	apnsSandboxEndpoint    = "https://api.development.push.apple.com:443/3/device"
)

func resourceNotificationHubCreateUpdate(ctx context.Context, metadata sdk.ResourceMetaData) error {
	client := metadata.Client.NotificationHubs.HubsClient
	subscriptionId := metadata.Client.Account.SubscriptionId

	var model NotificationHubModel
	if err := metadata.Decode(&model); err != nil {
		return fmt.Errorf("decoding: %+v", err)
	}

	id := hubs.NewNotificationHubID(subscriptionId, model.ResourceGroupName, model.NamespaceName, model.Name)

	if metadata.ResourceData.IsNewResource() {
		if !metadata.Client.Features.SkipImportCheckOnCreateAndAllowOverwritingExistingResources {
			existing, err := client.NotificationHubsGet(ctx, id)
			if err != nil {
				if !response.WasNotFound(existing.HttpResponse) {
					return fmt.Errorf("checking for presence of existing %s: %+v", id, err)
				}
			}

			if !response.WasNotFound(existing.HttpResponse) {
				return tf.ImportAsExistsError("azurerm_notification_hub", id.ID())
			}
		}
	}

	parameters := hubs.NotificationHubResource{
		Location: location.Normalize(model.Location),
		Properties: &hubs.NotificationHubProperties{
			AdmCredential:     expandNotificationHubsADMCredentials(model.AdmCredential),
			ApnsCredential:    expandNotificationHubsAPNSCredentials(model.ApnsCredential),
			BrowserCredential: expandNotificationHubsBrowserCredentials(model.BrowserCredential),
			GcmCredential:     expandNotificationHubsGCMCredentials(model.GcmCredential),
		},
		Tags: &model.Tags,
	}

	if _, err := client.NotificationHubsCreateOrUpdate(ctx, id, parameters); err != nil {
		return fmt.Errorf("creating %s: %+v", id, err)
	}

	// Notification Hubs are eventually consistent
	log.Printf("[DEBUG] Waiting for %s to become available..", id)
	deadline, ok := ctx.Deadline()
	if !ok {
		return fmt.Errorf("internal-error: context had no deadline")
	}
	stateConf := &pluginsdk.StateChangeConf{
		Pending:                   []string{"404"},
		Target:                    []string{"200"},
		Refresh:                   notificationHubStateRefreshFunc(ctx, client, id),
		MinTimeout:                15 * time.Second,
		ContinuousTargetOccurence: 10,
		Timeout:                   time.Until(deadline),
	}
	if _, err := stateConf.WaitForStateContext(ctx); err != nil {
		return fmt.Errorf("waiting for %s to become available: %+v", id, err)
	}

	metadata.SetID(id)

	return nil
}

func (r NotificationHubResource) Create() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 30 * time.Minute,
		Func:    resourceNotificationHubCreateUpdate,
	}
}

func (r NotificationHubResource) Update() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 30 * time.Minute,
		Func:    resourceNotificationHubCreateUpdate,
	}
}

func notificationHubStateRefreshFunc(ctx context.Context, client *hubs.HubsClient, id hubs.NotificationHubId) pluginsdk.StateRefreshFunc {
	return func() (interface{}, string, error) {
		res, err := client.NotificationHubsGet(ctx, id)
		statusCode := "dropped connection"
		if res.HttpResponse != nil {
			statusCode = strconv.Itoa(res.HttpResponse.StatusCode)
		}

		if err != nil {
			if response.WasNotFound(res.HttpResponse) {
				return nil, statusCode, nil
			}

			return nil, "", fmt.Errorf("retrieving %s: %+v", id, err)
		}

		return res, statusCode, nil
	}
}

func (r NotificationHubResource) Read() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 5 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.NotificationHubs.HubsClient

			id, err := hubs.ParseNotificationHubID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}

			resp, err := client.NotificationHubsGet(ctx, *id)
			if err != nil {
				if response.WasNotFound(resp.HttpResponse) {
					log.Printf("[DEBUG] %s was not found - removing from state", *id)
					return metadata.MarkAsGone(id)
				}

				return fmt.Errorf("retrieving %s: %+v", *id, err)
			}

			credentials, err := client.NotificationHubsGetPnsCredentials(ctx, *id)
			if err != nil {
				return fmt.Errorf("retrieving credentials for %s: %+v", *id, err)
			}
			state := NotificationHubModel{
				Name:              id.NotificationHubName,
				NamespaceName:     id.NamespaceName,
				ResourceGroupName: id.ResourceGroupName,
			}

			if credentialsModel := credentials.Model; credentialsModel != nil {
				if props := credentialsModel.Properties; props != nil {
					state.AdmCredential = flattenNotificationHubsADMCredentials(props.AdmCredential)
					state.ApnsCredential = flattenNotificationHubsAPNSCredentials(props.ApnsCredential)
					state.BrowserCredential = flattenNotificationHubsBrowserCredentials(props.BrowserCredential)
					state.GcmCredential = flattenNotificationHubsGCMCredentials(props.GcmCredential)
				}
			}

			if model := resp.Model; model != nil {
				state.Location = location.NormalizeNilable(&model.Location)
				state.Tags = *model.Tags
			}

			return metadata.Encode(&state)
		},
	}
}

func (r NotificationHubResource) Delete() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 30 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.NotificationHubs.HubsClient

			id, err := hubs.ParseNotificationHubID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}

			resp, err := client.NotificationHubsDelete(ctx, *id)
			if err != nil {
				if !response.WasNotFound(resp.HttpResponse) {
					return fmt.Errorf("deleting %s: %+v", *id, err)
				}
			}

			return nil
		},
	}
}

func expandNotificationHubsADMCredentials(inputs []NotificationHubAdmCredentialModel) *hubs.AdmCredential {
	if len(inputs) == 0 {
		return nil
	}

	input := inputs[0]
	authTokenUrl := input.AuthTokenUrl
	clientId := input.ClientId
	clientSecret := input.ClientSecret

	credentials := hubs.AdmCredential{
		Properties: hubs.AdmCredentialProperties{
			AuthTokenURL: authTokenUrl,
			ClientId:     clientId,
			ClientSecret: clientSecret,
		},
	}
	return &credentials
}

func expandNotificationHubsAPNSCredentials(inputs []NotificationHubApnsCredentialModel) *hubs.ApnsCredential {
	if len(inputs) == 0 {
		return nil
	}

	input := inputs[0]
	applicationMode := input.ApplicationMode
	bundleId := input.BundleId
	keyId := input.KeyId
	teamId := input.TeamId
	token := input.Token

	applicationEndpoints := map[string]string{
		apnsProductionName: apnsProductionEndpoint,
		apnsSandboxName:    apnsSandboxEndpoint,
	}
	endpoint := applicationEndpoints[applicationMode]

	credentials := hubs.ApnsCredential{
		Properties: hubs.ApnsCredentialProperties{
			AppId:    pointer.To(teamId),
			AppName:  pointer.To(bundleId),
			Endpoint: endpoint,
			KeyId:    pointer.To(keyId),
			Token:    pointer.To(token),
		},
	}
	return &credentials
}

func expandNotificationHubsBrowserCredentials(inputs []NotificationHubBrowserCredentialModel) *hubs.BrowserCredential {
	if len(inputs) == 0 {
		return nil
	}

	input := inputs[0]
	credentials := hubs.BrowserCredential{
		Properties: hubs.BrowserCredentialProperties{
			Subject:         input.Subject,
			VapidPrivateKey: input.VapidPrivateKey,
			VapidPublicKey:  input.VapidPublicKey,
		},
	}
	return &credentials
}

func flattenNotificationHubsADMCredentials(input *hubs.AdmCredential) []NotificationHubAdmCredentialModel {
	if input == nil {
		return make([]NotificationHubAdmCredentialModel, 0)
	}

	output := NotificationHubAdmCredentialModel{}

	output.AuthTokenUrl = input.Properties.AuthTokenURL
	output.ClientId = input.Properties.ClientId
	output.ClientSecret = input.Properties.ClientSecret

	return []NotificationHubAdmCredentialModel{output}
}

func flattenNotificationHubsAPNSCredentials(input *hubs.ApnsCredential) []NotificationHubApnsCredentialModel {
	if input == nil {
		return make([]NotificationHubApnsCredentialModel, 0)
	}

	output := NotificationHubApnsCredentialModel{}

	if bundleId := input.Properties.AppName; bundleId != nil {
		output.BundleId = *bundleId
	}

	applicationEndpoints := map[string]string{
		apnsProductionEndpoint: apnsProductionName,
		apnsSandboxEndpoint:    apnsSandboxName,
	}
	applicationMode := applicationEndpoints[input.Properties.Endpoint]
	output.ApplicationMode = applicationMode

	if keyId := input.Properties.KeyId; keyId != nil {
		output.KeyId = *keyId
	}

	if teamId := input.Properties.AppId; teamId != nil {
		output.TeamId = *teamId
	}

	if token := input.Properties.Token; token != nil {
		output.Token = *token
	}

	return []NotificationHubApnsCredentialModel{output}
}

func flattenNotificationHubsBrowserCredentials(input *hubs.BrowserCredential) []NotificationHubBrowserCredentialModel {
	if input == nil {
		return make([]NotificationHubBrowserCredentialModel, 0)
	}

	output := NotificationHubBrowserCredentialModel{
		Subject:         input.Properties.Subject,
		VapidPrivateKey: input.Properties.VapidPrivateKey,
		VapidPublicKey:  input.Properties.VapidPublicKey,
	}

	return []NotificationHubBrowserCredentialModel{output}
}

func expandNotificationHubsGCMCredentials(inputs []NotificationHubGcmCredentialModel) *hubs.GcmCredential {
	if len(inputs) == 0 {
		return nil
	}

	input := inputs[0]
	credentials := hubs.GcmCredential{
		Properties: hubs.GcmCredentialProperties{
			GoogleApiKey: input.ApiKey,
		},
	}
	return &credentials
}

func flattenNotificationHubsGCMCredentials(input *hubs.GcmCredential) []NotificationHubGcmCredentialModel {
	if input == nil {
		return []NotificationHubGcmCredentialModel{}
	}

	output := NotificationHubGcmCredentialModel{
		ApiKey: input.Properties.GoogleApiKey,
	}

	return []NotificationHubGcmCredentialModel{output}
}
