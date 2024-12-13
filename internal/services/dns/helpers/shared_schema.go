// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package helpers

import (
	"github.com/hashicorp/go-azure-helpers/resourcemanager/commonschema"
	"github.com/hashicorp/go-azure-sdk/resource-manager/dns/2023-07-01-preview/zones"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
)

type DnsRecordModel struct {
	Name   string            `tfschema:"name"`
	ZoneId string            `tfschema:"dns_zone_id"`
	Ttl    int64             `tfschema:"ttl"`
	Tags   map[string]string `tfschema:"tags"`
	Fqdn   string            `tfschema:"fqdn"`
}

func DnsRecordResourceArgumentsSchema() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"name": {
			Type:     pluginsdk.TypeString,
			Required: true,
			ForceNew: true,
		},

		"dns_zone_id": {
			Type:         pluginsdk.TypeString,
			Required:     true,
			ValidateFunc: zones.ValidateDnsZoneID,
		},

		"ttl": {
			Type:     pluginsdk.TypeInt,
			Required: true,
		},

		"tags": commonschema.Tags(),
	}
}

func DnsRecordResourceAttributesSchema() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"fqdn": {
			Type:     pluginsdk.TypeString,
			Computed: true,
		},
	}
}

func DnsRecordDataSourceArgumentsSchema() map[string]*pluginsdk.Schema {
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

func DnsRecordDataSourceAttributesSchema() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
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
