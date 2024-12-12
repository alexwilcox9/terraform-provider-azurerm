// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package dns

import (
	"bytes"
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

var (
	_ sdk.Resource           = DnsTLSARecordResource{}
	_ sdk.ResourceWithUpdate = DnsTLSARecordResource{}
)

type DnsTLSARecordResource struct{}

func (DnsTLSARecordResource) ModelObject() interface{} {
	return &DnsTLSARecordResourceModel{}
}

func (DnsTLSARecordResource) IDValidationFunc() pluginsdk.SchemaValidateFunc {
	return validate.ValidateRecordTypeID(recordsets.RecordTypeTLSA)
}

func (DnsTLSARecordResource) ResourceType() string {
	return "azurerm_dns_tlsa_record"
}

type DnsTLSARecordResourceModel struct {
	Name              string                        `tfschema:"name"`
	ResourceGroupName string                        `tfschema:"resource_group_name"`
	ZoneName          string                        `tfschema:"zone_name"`
	Ttl               int64                         `tfschema:"ttl"`
	Record            []DnsTLSARecordResourceRecord `tfschema:"record"`
	Tags              map[string]string             `tfschema:"tags"`
	Fqdn              string                        `tfschema:"fqdn"`
}

type DnsTLSARecordResourceRecord struct {
	MatchingType        int64  `tfschema:"matching_type"`
	Selector            int64  `tfschema:"selector"`
	Usage               int64  `tfschema:"usage"`
	CertAssociationData string `tfschema:"cert_association_data"`
}

func (DnsTLSARecordResource) Arguments() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"name": {
			Type:     pluginsdk.TypeString,
			Required: true,
			ForceNew: true,
		},

		"resource_group_name": commonschema.ResourceGroupName(),

		"zone_name": {
			Type:     pluginsdk.TypeString,
			Required: true,
			ForceNew: true,
		},

		"record": {
			Type:     pluginsdk.TypeSet,
			Required: true,
			Elem: &pluginsdk.Resource{
				Schema: map[string]*pluginsdk.Schema{
					"matching_type": {
						Type:     pluginsdk.TypeInt,
						Required: true,
					},

					"selector": {
						Type:     pluginsdk.TypeInt,
						Required: true,
					},

					"usage": {
						Type:     pluginsdk.TypeInt,
						Required: true,
					},

					"cert_association_data": {
						Type:     pluginsdk.TypeString,
						Required: true,
					},
				},
			},
			Set: resourceDnsTLSARecordHash,
		},

		"ttl": {
			Type:     pluginsdk.TypeInt,
			Required: true,
		},

		"tags": commonschema.Tags(),
	}
}

func (DnsTLSARecordResource) Attributes() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"fqdn": {
			Type:     pluginsdk.TypeString,
			Computed: true,
		},
	}
}

func (r DnsTLSARecordResource) Create() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 30 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			var model DnsTLSARecordResourceModel
			if err := metadata.Decode(&model); err != nil {
				return err
			}

			client := metadata.Client.Dns.RecordSets
			subscriptionId := metadata.Client.Account.SubscriptionId

			id := recordsets.NewRecordTypeID(subscriptionId, model.ResourceGroupName, model.ZoneName, recordsets.RecordTypeTLSA, model.Name)

			existing, err := client.Get(ctx, id)
			if err != nil {
				if !response.WasNotFound(existing.HttpResponse) {
					return fmt.Errorf("checking for presence of existing %s: %+v", id, err)
				}
			}

			if !response.WasNotFound(existing.HttpResponse) {
				return metadata.ResourceRequiresImport(r.ResourceType(), id)
			}

			parameters := recordsets.RecordSet{
				Name: pointer.To(model.Name),
				Properties: &recordsets.RecordSetProperties{
					Metadata:    pointer.To(model.Tags),
					TTL:         pointer.To(model.Ttl),
					TLSARecords: expandAzureRmDnsTLSARecords(model.Record),
				},
			}

			if _, err := client.CreateOrUpdate(ctx, id, parameters, recordsets.DefaultCreateOrUpdateOperationOptions()); err != nil {
				return fmt.Errorf("creating %s: %+v", id, err)
			}

			metadata.SetID(id)

			return nil
		},
	}
}

func (DnsTLSARecordResource) Read() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 5 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.Dns.RecordSets

			state := DnsTLSARecordResourceModel{}

			id, err := recordsets.ParseRecordTypeID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}

			resp, err := client.Get(ctx, *id)
			if err != nil {
				if response.WasNotFound(resp.HttpResponse) {
					return metadata.MarkAsGone(id)
				}
				return fmt.Errorf("retrieving %s: %+v", *id, err)
			}

			state.Name = id.RelativeRecordSetName
			state.ResourceGroupName = id.ResourceGroupName
			state.ZoneName = id.DnsZoneName

			if model := resp.Model; model != nil {
				if props := model.Properties; props != nil {
					state.Ttl = pointer.From(props.TTL)
					state.Fqdn = pointer.From(props.Fqdn)

					state.Record = flattenAzureRmDnsTLSARecords(props.TLSARecords)

					state.Tags = pointer.From(props.Metadata)
				}
			}

			return metadata.Encode(&state)
		},
	}
}

func (DnsTLSARecordResource) Update() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 30 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.Dns.RecordSets

			var model DnsTLSARecordResourceModel

			if err := metadata.Decode(&model); err != nil {
				return err
			}

			id, err := recordsets.ParseRecordTypeID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}

			existing, err := client.Get(ctx, *id)
			if err != nil {
				return fmt.Errorf("retrieving %s: %+v", *id, err)
			}

			if existing.Model == nil {
				return fmt.Errorf("retrieving %s: `model` was nil", id)
			}

			if existing.Model.Properties == nil {
				return fmt.Errorf("retrieving %s: `properties` was nil", id)
			}

			payload := *existing.Model

			if metadata.ResourceData.HasChange("record") {
				payload.Properties.TLSARecords = expandAzureRmDnsTLSARecords(model.Record)
			}

			if metadata.ResourceData.HasChange("ttl") {
				payload.Properties.TTL = pointer.To(model.Ttl)
			}

			if metadata.ResourceData.HasChange("tags") {
				payload.Properties.Metadata = pointer.To(model.Tags)
			}

			if _, err := client.CreateOrUpdate(ctx, *id, payload, recordsets.DefaultCreateOrUpdateOperationOptions()); err != nil {
				return fmt.Errorf("updating %s: %+v", id, err)
			}

			return nil
		},
	}
}

func (DnsTLSARecordResource) Delete() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 30 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.Dns.RecordSets

			id, err := recordsets.ParseRecordTypeID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}

			if _, err := client.Delete(ctx, *id, recordsets.DefaultDeleteOperationOptions()); err != nil {
				return fmt.Errorf("deleting %s: %+v", *id, err)
			}

			return nil
		},
	}
}

func flattenAzureRmDnsTLSARecords(records *[]recordsets.TlsaRecord) []DnsTLSARecordResourceRecord {
	results := make([]DnsTLSARecordResourceRecord, 0)

	if records != nil {
		for _, record := range *records {
			results = append(results, DnsTLSARecordResourceRecord{
				MatchingType:        pointer.From(record.MatchingType),
				Selector:            pointer.From(record.Selector),
				Usage:               pointer.From(record.Usage),
				CertAssociationData: pointer.From(record.CertAssociationData),
			})
		}
	}

	return results
}

func expandAzureRmDnsTLSARecords(d []DnsTLSARecordResourceRecord) *[]recordsets.TlsaRecord {
	records := make([]recordsets.TlsaRecord, 0)

	for _, v := range d {
		records = append(records, recordsets.TlsaRecord{
			MatchingType:        pointer.To(v.MatchingType),
			Selector:            pointer.To(v.Selector),
			Usage:               pointer.To(v.Usage),
			CertAssociationData: pointer.To(v.CertAssociationData),
		})
	}

	return &records
}

func resourceDnsTLSARecordHash(v interface{}) int {
	var buf bytes.Buffer

	if m, ok := v.(map[string]interface{}); ok {
		buf.WriteString(fmt.Sprintf("%d-", m["matching_type"].(int)))
		buf.WriteString(fmt.Sprintf("%d-", m["selector"].(int)))
		buf.WriteString(fmt.Sprintf("%d-", m["usage"].(int)))
		buf.WriteString(fmt.Sprintf("%s-", m["cert_association_data"].(string)))
	}

	return pluginsdk.HashString(buf.String())
}
