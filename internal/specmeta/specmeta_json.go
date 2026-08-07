package specmeta

// SpecmetaJSON returns every specmeta constant as a JSON-ready map. This is the
// single source of the shape exposed to the GUI via the WASM bridge
// (cmd/wasm-validator -> goSpecmeta). The GUI loads it once at boot into its
// live SPECMETA object — no other copy is shipped.
//
// Keep the keys in sync with the Specmeta type in
// domaincraft-studio/src/lib/specmeta.ts.
func SpecmetaJSON() map[string]any {
	return map[string]any{
		"primitiveFieldTypes":        PrimitiveFieldTypes,
		"stringFieldTypes":           StringFieldTypes,
		"numericFieldTypes":          NumericFieldTypes,
		"booleanFieldTypes":          BooleanFieldTypes,
		"metaFieldTypes":             MetaFieldTypes,
		"fieldTypes":                 FieldTypes,
		"onDeleteValues":             OnDeleteValues,
		"features":                   Features,
		"addons":                     Addons,
		"indexTypes":                 IndexTypes,
		"databases":                  Databases,
		"apiStyles":                  APIStyles,
		"authTypes":                  AuthTypes,
		"cacheProviders":             CacheProviders,
		"multiTenancyModes":          MultiTenancyModes,
		"rateLimitPolicies":          RateLimitPolicies,
		"permissionKeys":             PermissionKeys,
		"sortDirections":             SortDirections,
		"stringValidationModifiers":  StringValidationModifiers,
		"numericValidationModifiers": NumericValidationModifiers,
		"infraQueues":                InfraQueues,
		"infraCacheStores":           InfraCacheStores,
		"infraSecretStores":          InfraSecretStores,
		"infraStores":                InfraStores,
		"featureFieldDefs":           FeatureFieldDefs,
	}
}
