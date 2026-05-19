package ir

// IRProject represents the intermediate project model.
type IRProject struct {
	Name     string
	Database string
	Auth     string
	APIStyle string
	Enums    map[string][]string
	Entities []IREntity
}

// IREntity represents an entity in IR.
type IREntity struct {
	Name              string
	NamePlural        string
	HasAudit          bool
	HasSoftDelete     bool
	HasOptimisticLock bool
	Fields            []IRField
	RelationsOut      []IRRelation
	RelationsIn       []IRRelation
	Indexes           []IRIndex
	Seed              []map[string]interface{}
	Permissions       *IRPermissions
}

// IRField represents a field in IR.
type IRField struct {
	Name           string
	DatabaseType   string
	IsPrimary      bool
	IsNullable     bool
	IsUnique       bool
	IsHidden       bool
	IsRelation     bool
	IsMany         bool
	RelationTarget string
	DefaultValue   string
	DefaultIsFunc  bool
	Validations    []IRValidation
}

// IRValidation represents one validation rule.
type IRValidation struct {
	Name  string
	Value string
}

// IRRelation represents a relation between entities.
type IRRelation struct {
	FieldName        string
	TargetEntity     *IREntity
	NavigationName   string
	InverseNavName   string
	OnDeleteBehavior string
	IsNullable       bool
	IsMany           bool
	RelationType     string
}

// IRIndex represents an index.
type IRIndex struct {
	Name   string
	Fields []string
	Type   string
	Sort   []string
	Unique bool
}

// IRPermissions represents entity permissions in IR.
type IRPermissions struct {
	Read       []string
	Create     []string
	Update     []string
	Delete     []string
	ReadPublic string
}
