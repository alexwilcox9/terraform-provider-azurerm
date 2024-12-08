// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package dns

import (
	"bytes"
	"fmt"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-helpers/lang/response"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/commonschema"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/tags"
	"github.com/hashicorp/go-azure-sdk/resource-manager/dns/2023-07-01-preview/recordsets"
	"github.com/hashicorp/terraform-provider-azurerm/helpers/tf"
	"github.com/hashicorp/terraform-provider-azurerm/internal/clients"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/timeouts"
)

func resourceDnsTLSARecord() *pluginsdk.Resource {
	return &pluginsdk.Resource{
		Create: resourceDnsTLSARecordCreate,
		Read:   resourceDnsTLSARecordRead,
		Update: resourceDnsTLSARecordUpdate,
		Delete: resourceDnsTLSARecordDelete,

		Timeouts: &pluginsdk.ResourceTimeout{
			Create: pluginsdk.DefaultTimeout(30 * time.Minute),
			Read:   pluginsdk.DefaultTimeout(5 * time.Minute),
			Update: pluginsdk.DefaultTimeout(30 * time.Minute),
			Delete: pluginsdk.DefaultTimeout(30 * time.Minute),
		},

		Importer: pluginsdk.ImporterValidatingResourceId(func(id string) error {
			parsed, err := recordsets.ParseRecordTypeID(id)
			if err != nil {
				return err
			}
			if parsed.RecordType != recordsets.RecordTypeTLSA {
				return fmt.Errorf("this resource only supports 'TLSA' records")
			}
			return nil
		}),

		Schema: map[string]*pluginsdk.Schema{
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

			"fqdn": {
				Type:     pluginsdk.TypeString,
				Computed: true,
			},

			"tags": commonschema.Tags(),
		},
	}
}

func resourceDnsTLSARecordCreate(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Dns.RecordSets
	ctx, cancel := timeouts.ForCreate(meta.(*clients.Client).StopContext, d)
	subscriptionId := meta.(*clients.Client).Account.SubscriptionId
	defer cancel()

	name := d.Get("name").(string)
	resGroup := d.Get("resource_group_name").(string)
	zoneName := d.Get("zone_name").(string)

	id := recordsets.NewRecordTypeID(subscriptionId, resGroup, zoneName, recordsets.RecordTypeTLSA, name)

	existing, err := client.Get(ctx, id)
	if err != nil {
		if !response.WasNotFound(existing.HttpResponse) {
			return fmt.Errorf("checking for presence of existing %s: %+v", id, err)
		}
	}

	if !response.WasNotFound(existing.HttpResponse) {
		return tf.ImportAsExistsError("azurerm_dns_ds_record", id.ID())
	}

	ttl := int64(d.Get("ttl").(int))
	t := d.Get("tags").(map[string]interface{})

	parameters := recordsets.RecordSet{
		Name: &name,
		Properties: &recordsets.RecordSetProperties{
			Metadata:    tags.Expand(t),
			TTL:         pointer.To(ttl),
			TLSARecords: expandAzureRmDnsTLSARecords(d),
		},
	}

	if _, err := client.CreateOrUpdate(ctx, id, parameters, recordsets.DefaultCreateOrUpdateOperationOptions()); err != nil {
		return fmt.Errorf("creating %s: %+v", id, err)
	}

	d.SetId(id.ID())

	return resourceDnsTLSARecordRead(d, meta)
}

func resourceDnsTLSARecordRead(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Dns.RecordSets
	ctx, cancel := timeouts.ForRead(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := recordsets.ParseRecordTypeID(d.Id())
	if err != nil {
		return err
	}

	resp, err := client.Get(ctx, *id)
	if err != nil {
		if response.WasNotFound(resp.HttpResponse) {
			d.SetId("")
			return nil
		}

		return fmt.Errorf("retrieving %s: %+v", *id, err)
	}

	d.Set("name", id.RelativeRecordSetName)
	d.Set("resource_group_name", id.ResourceGroupName)
	d.Set("zone_name", id.DnsZoneName)

	if model := resp.Model; model != nil {
		if props := model.Properties; props != nil {
			d.Set("ttl", props.TTL)
			d.Set("fqdn", props.Fqdn)

			if err := d.Set("record", flattenAzureRmDnsTLSARecords(props.TLSARecords)); err != nil {
				return err
			}
			if err := tags.FlattenAndSet(d, props.Metadata); err != nil {
				return err
			}
		}
	}

	return nil
}

func resourceDnsTLSARecordUpdate(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Dns.RecordSets
	ctx, cancel := timeouts.ForUpdate(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := recordsets.ParseRecordTypeID(d.Id())
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

	if d.HasChange("record") {
		payload.Properties.TLSARecords = expandAzureRmDnsTLSARecords(d)
	}

	if d.HasChange("ttl") {
		payload.Properties.TTL = pointer.To(int64(d.Get("ttl").(int)))
	}

	if d.HasChange("tags") {
		payload.Properties.Metadata = tags.Expand(d.Get("tags").(map[string]interface{}))
	}

	if _, err := client.CreateOrUpdate(ctx, *id, payload, recordsets.DefaultCreateOrUpdateOperationOptions()); err != nil {
		return fmt.Errorf("updating %s: %+v", id, err)
	}

	d.SetId(id.ID())

	return resourceDnsSrvRecordRead(d, meta)
}

func resourceDnsTLSARecordDelete(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Dns.RecordSets
	ctx, cancel := timeouts.ForDelete(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := recordsets.ParseRecordTypeID(d.Id())
	if err != nil {
		return err
	}

	if _, err := client.Delete(ctx, *id, recordsets.DefaultDeleteOperationOptions()); err != nil {
		return fmt.Errorf("deleting %s: %+v", *id, err)
	}

	return nil
}

func flattenAzureRmDnsTLSARecords(records *[]recordsets.TlsaRecord) []map[string]interface{} {
	results := make([]map[string]interface{}, 0)

	if records != nil {
		for _, record := range *records {
			matchingType := int64(0)
			if record.MatchingType != nil {
				matchingType = *record.MatchingType
			}

			selector := int64(0)
			if record.Selector != nil {
				selector = *record.Selector
			}

			usage := int64(0)
			if record.Usage != nil {
				usage = *record.Usage
			}

			certAssociationData := ""
			if record.CertAssociationData != nil {
				certAssociationData = *record.CertAssociationData
			}

			results = append(results, map[string]interface{}{
				"matching_type":         matchingType,
				"selector":              selector,
				"usage":                 usage,
				"cert_association_data": certAssociationData,
			})
		}
	}

	return results
}

func expandAzureRmDnsTLSARecords(d *pluginsdk.ResourceData) *[]recordsets.TlsaRecord {
	recordStrings := d.Get("record").(*pluginsdk.Set).List()
	records := make([]recordsets.TlsaRecord, 0)

	for _, v := range recordStrings {
		record := v.(map[string]interface{})
		matchingType := int64(record["matching_type"].(int))
		selector := int64(record["selector"].(int))
		usage := int64(record["usage"].(int))
		certAssociationData := record["cert_association_data"].(string)

		records = append(records, recordsets.TlsaRecord{
			MatchingType:        &matchingType,
			Selector:            &selector,
			Usage:               &usage,
			CertAssociationData: &certAssociationData,
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
