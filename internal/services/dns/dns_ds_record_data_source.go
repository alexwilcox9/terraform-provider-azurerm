// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package dns

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-helpers/lang/response"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/commonschema"
	"github.com/hashicorp/go-azure-sdk/resource-manager/dns/2023-07-01-preview/recordsets"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/dns/validate"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
)

var _ sdk.DataSource = DnsDSRecordDataResource{}

type DnsDSRecordDataResource struct{}

func (DnsDSRecordDataResource) ModelObject() interface{} {
	return &DnsDSRecordDataSourceModel{}
}

func (d DnsDSRecordDataResource) IDValidationFunc() pluginsdk.SchemaValidateFunc {
	return validate.ValidateRecordTypeID(recordsets.RecordTypeDS)
}

func (DnsDSRecordDataResource) ResourceType() string {
	return "azurerm_dns_ds_record"
}

type DnsDSRecordDataSourceModel struct {
	Name              string                      `tfschema:"name"`
	ResourceGroupName string                      `tfschema:"resource_group_name"`
	ZoneName          string                      `tfschema:"zone_name"`
	Ttl               int64                       `tfschema:"ttl"`
	Record            []DnsDSRecordResourceRecord `tfschema:"record"`
	Tags              map[string]string           `tfschema:"tags"`
	Fqdn              string                      `tfschema:"fqdn"`
}

func (DnsDSRecordDataResource) Arguments() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"name": {
			Type:     pluginsdk.TypeString,
			Required: true,
		},

		"resource_group_name": commonschema.ResourceGroupNameForDataSource(),

		"zone_name": {
			Type:     pluginsdk.TypeString,
			Required: true,
		},
	}
}

func (DnsDSRecordDataResource) Attributes() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"record": {
			Type:     pluginsdk.TypeSet,
			Computed: true,
			Elem: &pluginsdk.Resource{
				Schema: map[string]*pluginsdk.Schema{
					"algorithm": {
						Type:     pluginsdk.TypeInt,
						Computed: true,
					},

					"key_tag": {
						Type:     pluginsdk.TypeInt,
						Computed: true,
					},

					"digest_type": {
						Type:     pluginsdk.TypeInt,
						Computed: true,
					},

					"digest_value": {
						Type:     pluginsdk.TypeString,
						Computed: true,
					},
				},
			},
			Set: resourceDnsDSRecordHash,
		},

		"ttl": {
			Type:     pluginsdk.TypeInt,
			Computed: true,
		},

		"fqdn": {
			Type:     pluginsdk.TypeString,
			Computed: true,
		},

		"tags": commonschema.TagsDataSource(),
	}
}

func (DnsDSRecordDataResource) Read() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 5 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			var state DnsDSRecordDataSourceModel
			if err := metadata.Decode(&state); err != nil {
				return err
			}

			client := metadata.Client.Dns.RecordSets
			subscriptionId := metadata.Client.Account.SubscriptionId

			id := recordsets.NewRecordTypeID(subscriptionId, state.ResourceGroupName, state.ZoneName, recordsets.RecordTypeDS, state.Name)

			resp, err := client.Get(ctx, id)
			if err != nil {
				if response.WasNotFound(resp.HttpResponse) {
					return fmt.Errorf("record %s not found", id)
				}
				return fmt.Errorf("retrieving %s: %+v", id, err)
			}

			if model := resp.Model; model != nil {
				if props := model.Properties; props != nil {
					state.Ttl = pointer.From(props.TTL)
					state.Fqdn = pointer.From(props.Fqdn)

					state.Record = flattenAzureRmDnsDSRecords(props.DSRecords)

					state.Tags = pointer.From(props.Metadata)
				}
			}
			metadata.SetID(id)

			return metadata.Encode(&state)
		},
	}
}
