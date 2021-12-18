// Copyright 2021 The protobuf-tools Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"log"

	"cuelang.org/go/encoding/openapi"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/utils/pointer"
	crdutil "sigs.k8s.io/controller-tools/pkg/crd"
)

const (
	statusOutput = `
status:
  acceptedNames:
    kind: ""
    plural: ""
  conditions: null
  storedVersions: null`

	creationTimestampOutput = `
  creationTimestamp: null`
)

// Build CRDs based on the configuration and schema.
//nolint:staticcheck,interfacer,lll
func completeCRD(c *apiextv1.CustomResourceDefinition, versionSchemas map[string]*openapi.OrderedMap, statusSchema *openapi.OrderedMap, preserveUnknownFields map[string][]string) {
	for i, version := range c.Spec.Versions {

		b, err := versionSchemas[version.Name].MarshalJSON()
		if err != nil {
			log.Fatalf("Cannot marshal OpenAPI schema for %v: %v", c.Name, err)
		}

		j := &apiextv1.JSONSchemaProps{}
		if err = json.Unmarshal(b, j); err != nil {
			log.Fatalf("Cannot unmarshal raw OpenAPI schema to JSONSchemaProps for %v: %v", c.Name, err)
		}

		// mark fields as `x-kubernetes-preserve-unknown-fields: true` using the visitor
		if fs, ok := preserveUnknownFields[version.Name]; ok {
			for _, f := range fs {
				p := &preserveUnknownFieldVisitor{path: f}
				crdutil.EditSchema(j, p)
			}
		}

		version.Schema = &apiextv1.CustomResourceValidation{OpenAPIV3Schema: &apiextv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]apiextv1.JSONSchemaProps{
				"spec": *j,
			},
		}}

		// only add status schema validation when status subresource is enabled in the CRD.
		if version.Subresources != nil {
			status := &apiextv1.JSONSchemaProps{}
			if statusSchema == nil {
				status = &apiextv1.JSONSchemaProps{
					Type:                   "object",
					XPreserveUnknownFields: pointer.BoolPtr(true),
				}
			} else {
				o, err := statusSchema.MarshalJSON()
				if err != nil {
					log.Fatal("Cannot marshal OpenAPI schema for the status field")
				}

				if err = json.Unmarshal(o, status); err != nil {
					log.Fatal("Cannot unmarshal raw status schema to JSONSchemaProps")
				}
			}

			version.Schema.OpenAPIV3Schema.Properties["status"] = *status
		}

		fmt.Printf("Checking if the schema is structural for %v \n", c.Name)
		if err = validateStructural(version.Schema.OpenAPIV3Schema); err != nil {
			log.Fatal(err)
		}

		c.Spec.Versions[i] = version
	}

	c.APIVersion = apiextv1.SchemeGroupVersion.String()
	c.Kind = "CustomResourceDefinition"

	// marshal to an empty field in the output
	c.Status = apiextv1.CustomResourceDefinitionStatus{}
}

func validateStructural(s *apiextv1.JSONSchemaProps) error {
	out := &apiext.JSONSchemaProps{}
	if err := apiextv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(s, out, nil); err != nil {
		return fmt.Errorf("cannot convert v1 JSONSchemaProps to JSONSchemaProps: %v", err)
	}

	r, err := structuralschema.NewStructural(out)
	if err != nil {
		return fmt.Errorf("cannot convert to a structural schema: %v", err)
	}

	if errs := structuralschema.ValidateStructural(nil, r); len(errs) != 0 {
		return fmt.Errorf("schema is not structural: %v", errs.ToAggregate().Error())
	}

	return nil
}
