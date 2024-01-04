// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

// Package compliance fetch information needed to render the 'compliance' section of the status page.
package compliance

import (
	"encoding/json"
	"expvar"
)

// PopulateStatus populates the status stats
func PopulateStatus(stats map[string]interface{}) {
	complianceVar := expvar.Get("compliance")
	if complianceVar != nil {
		complianceStatusJSON := []byte(complianceVar.String())
		complianceStatus := make(map[string]interface{})
		json.Unmarshal(complianceStatusJSON, &complianceStatus) //nolint:errcheck
		stats["complianceChecks"] = complianceStatus["Checks"]
	} else {
		stats["complianceChecks"] = map[string]interface{}{}
	}
}