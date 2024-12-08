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

func resourceDnsDSRecord() *pluginsdk.Resource {
	return &pluginsdk.Resource{
		Create: resourceDnsDSRecordCreate,
		Read:   resourceDnsDSRecordRead,
		Update: resourceDnsDSRecordUpdate,
		Delete: resourceDnsDSRecordDelete,

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
			if parsed.RecordType != recordsets.RecordTypeDS {
				return fmt.Errorf("this resource only supports 'DS' records")
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
						"algorithm": {
							Type:     pluginsdk.TypeInt,
							Required: true,
						},

						"key_tag": {
							Type:     pluginsdk.TypeInt,
							Required: true,
						},

						"digest_type": {
							Type:     pluginsdk.TypeInt,
							Required: true,
						},

						"digest_value": {
							Type:     pluginsdk.TypeString,
							Required: true,
						},
					},
				},
				Set: resourceDnsDSRecordHash,
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

func resourceDnsDSRecordCreate(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Dns.RecordSets
	ctx, cancel := timeouts.ForCreate(meta.(*clients.Client).StopContext, d)
	subscriptionId := meta.(*clients.Client).Account.SubscriptionId
	defer cancel()

	name := d.Get("name").(string)
	resGroup := d.Get("resource_group_name").(string)
	zoneName := d.Get("zone_name").(string)

	id := recordsets.NewRecordTypeID(subscriptionId, resGroup, zoneName, recordsets.RecordTypeDS, name)

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
			Metadata:  tags.Expand(t),
			TTL:       pointer.To(ttl),
			DSRecords: expandAzureRmDnsDSRecords(d),
		},
	}

	if _, err := client.CreateOrUpdate(ctx, id, parameters, recordsets.DefaultCreateOrUpdateOperationOptions()); err != nil {
		return fmt.Errorf("creating %s: %+v", id, err)
	}

	d.SetId(id.ID())

	return resourceDnsDSRecordRead(d, meta)
}

func resourceDnsDSRecordRead(d *pluginsdk.ResourceData, meta interface{}) error {
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

			if err := d.Set("record", flattenAzureRmDnsDSRecords(props.DSRecords)); err != nil {
				return err
			}
			if err := tags.FlattenAndSet(d, props.Metadata); err != nil {
				return err
			}
		}
	}

	return nil
}

func resourceDnsDSRecordUpdate(d *pluginsdk.ResourceData, meta interface{}) error {
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
		payload.Properties.DSRecords = expandAzureRmDnsDSRecords(d)
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

func resourceDnsDSRecordDelete(d *pluginsdk.ResourceData, meta interface{}) error {
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

func flattenAzureRmDnsDSRecords(records *[]recordsets.DsRecord) []map[string]interface{} {
	results := make([]map[string]interface{}, 0)

	if records != nil {
		for _, record := range *records {
			algorithm := int64(0)
			if record.Algorithm != nil {
				algorithm = *record.Algorithm
			}

			keyTag := int64(0)
			if record.KeyTag != nil {
				keyTag = *record.KeyTag
			}

			digestType := int64(0)
			digestValue := ""
			if record.Digest != nil {
				if record.Digest.AlgorithmType != nil {
					digestType = *record.Digest.AlgorithmType
				}

				if record.Digest.Value != nil {
					digestValue = *record.Digest.Value
				}
			}

			results = append(results, map[string]interface{}{
				"algorithm":    algorithm,
				"key_tag":      keyTag,
				"digest_type":  digestType,
				"digest_value": digestValue,
			})
		}
	}

	return results
}

func expandAzureRmDnsDSRecords(d *pluginsdk.ResourceData) *[]recordsets.DsRecord {
	recordStrings := d.Get("record").(*pluginsdk.Set).List()
	records := make([]recordsets.DsRecord, 0)

	for _, v := range recordStrings {
		record := v.(map[string]interface{})
		algorithm := int64(record["algorithm"].(int))
		keyTag := int64(record["key_tag"].(int))
		digestType := int64(record["digest_type"].(int))
		digestValue := record["digest_value"].(string)

		records = append(records, recordsets.DsRecord{
			Algorithm: &algorithm,
			KeyTag:    &keyTag,
			Digest: &recordsets.Digest{
				AlgorithmType: &digestType,
				Value:         &digestValue,
			},
		})
	}

	return &records
}

func resourceDnsDSRecordHash(v interface{}) int {
	var buf bytes.Buffer

	if m, ok := v.(map[string]interface{}); ok {
		buf.WriteString(fmt.Sprintf("%d-", m["algorithm"].(int)))
		buf.WriteString(fmt.Sprintf("%d-", m["key_tag"].(int)))
		buf.WriteString(fmt.Sprintf("%d-", m["digest_type"].(int)))
		buf.WriteString(fmt.Sprintf("%s-", m["digest_value"].(string)))
	}

	return pluginsdk.HashString(buf.String())
}
