// Copyright 2021 The protobuf-tools Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/utils/pointer"
	crdutil "sigs.k8s.io/controller-tools/pkg/crd"
)

var _ crdutil.SchemaVisitor = &preserveUnknownFieldVisitor{}

// a visitor to add x-kubernetes-preserve-unknown-field to a schema
type preserveUnknownFieldVisitor struct {
	// path is in the format of a.b.c to indicate a field path in the schema
	// a `[]` indicates an array, a string is a key to a map in the schema
	// e.g. a.[].b.c
	path string
}

func (v *preserveUnknownFieldVisitor) Visit(schema *apiextv1.JSONSchemaProps) crdutil.SchemaVisitor {
	if schema == nil {
		return v
	}
	p := strings.Split(v.path, ".")
	if len(p) == 0 {
		return nil
	}
	if len(p) == 1 {
		if s, ok := schema.Properties[p[0]]; ok {
			s.XPreserveUnknownFields = pointer.BoolPtr(true)
			schema.Properties[p[0]] = s
		}
		return nil
	}
	if len(p) > 1 {
		if p[0] == "[]" && schema.Items == nil {
			return nil
		}
		if p[0] != "[]" && schema.Items != nil {
			return nil
		}
		if _, ok := schema.Properties[p[0]]; p[0] != "[]" && !ok {
			return nil
		}
		return &preserveUnknownFieldVisitor{path: strings.Join(p[1:], ".")}
	}
	return nil
}
