package specmeta

var Databases = []string{
	"postgresql",
	"mysql",
	"sqlite",
	"mssql",
	"mongodb",
}

var APIStyles = []string{
	"rest",
	"graphql",
	"grpc",
}

var Features = []string{
	"audit",
	"audit_log",
	"soft_delete",
	"optimistic_lock",
}

var FieldTypes = []string{
	"string",
	"int",
	"bigint",
	"float",
	"decimal",
	"boolean",
	"date",
	"datetime",
	"uuid",
	"text",
	"json",
	"jsonb",
	"relation",
	"array",
	"enum",
}

var OnDeleteValues = []string{
	"cascade",
	"set_null",
	"restrict",
	"no_action",
}
