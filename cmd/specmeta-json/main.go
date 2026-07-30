package main

import (
	"encoding/json"
	"os"

	"github.com/DomainCraft/DomainCraft/internal/specmeta"
)

func main() {
	json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
		"primitiveFieldTypes":       specmeta.PrimitiveFieldTypes,
		"stringFieldTypes":          specmeta.StringFieldTypes,
		"numericFieldTypes":         specmeta.NumericFieldTypes,
		"booleanFieldTypes":         specmeta.BooleanFieldTypes,
		"metaFieldTypes":            specmeta.MetaFieldTypes,
		"onDeleteValues":            specmeta.OnDeleteValues,
		"features":                  specmeta.Features,
		"indexTypes":                specmeta.IndexTypes,
		"databases":                 specmeta.Databases,
		"apiStyles":                 specmeta.APIStyles,
		"authTypes":                 specmeta.AuthTypes,
		"cacheProviders":            specmeta.CacheProviders,
		"multiTenancyModes":         specmeta.MultiTenancyModes,
		"permissionKeys":            specmeta.PermissionKeys,
		"sortDirections":            specmeta.SortDirections,
		"stringValidationModifiers": specmeta.StringValidationModifiers,
		"numericValidationModifiers": specmeta.NumericValidationModifiers,
		"featureFieldDefs":          specmeta.FeatureFieldDefs,
	})
}
