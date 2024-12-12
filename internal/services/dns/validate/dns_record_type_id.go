// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package validate

import (
	"fmt"

	"github.com/hashicorp/go-azure-sdk/resource-manager/dns/2023-07-01-preview/recordsets"
)

func ValidateRecordTypeID(recordType recordsets.RecordType) func(interface{}, string) ([]string, []error) {
	return func(input interface{}, key string) (warnings []string, errors []error) {
		warnings, errors = recordsets.ValidateRecordTypeID(input, key)
		if len(warnings) > 0 || len(errors) > 0 {
			return
		}

		// recordsets.ValidateRecordTypeID checks that input is a string and that it parses successfully so we don't need to recheck
		parsed, _ := recordsets.ParseRecordTypeID(input.(string))
		if parsed.RecordType != recordType {
			errors = append(errors, fmt.Errorf("this resource only supports '%q' records", recordType))
			return
		}

		return
	}
}
