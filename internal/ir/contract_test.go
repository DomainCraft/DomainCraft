package ir

import "testing"

func TestValidationRules(t *testing.T) {
	f := IRField{
		Name:         "email",
		DatabaseType: "string",
		Validations: []IRValidation{
			{Name: "email", Value: "true"},
			{Name: "min", Value: "3"},
			{Name: "max", Value: "50"},
		},
	}
	rules := f.ValidationRules()

	kinds := map[string]string{}
	for _, r := range rules {
		kinds[r.Kind] = r.Value
	}
	if _, ok := kinds["required"]; !ok {
		t.Error("non-nullable string should be required")
	}
	if _, ok := kinds["email"]; !ok {
		t.Error("email validator missing")
	}
	if kinds["min_length"] != "3" {
		t.Errorf("min_length = %q, want 3", kinds["min_length"])
	}
	if kinds["max_length"] != "50" {
		t.Errorf("max_length = %q, want 50", kinds["max_length"])
	}
}

func TestValidationRulesNumericBounds(t *testing.T) {
	f := IRField{
		Name:         "qty",
		DatabaseType: "int",
		Validations: []IRValidation{
			{Name: "gte", Value: "10"},
			{Name: "lte", Value: "20"},
		},
	}
	kinds := map[string]string{}
	for _, r := range f.ValidationRules() {
		kinds[r.Kind] = r.Value
	}
	if kinds["min_value"] != "10" || kinds["max_value"] != "20" {
		t.Errorf("got %v, want min_value=10 max_value=20", kinds)
	}
}

func TestValidationRulesCodesAndMessages(t *testing.T) {
	cases := []struct {
		name     string
		field    IRField
		wantCode string
		wantMsg  string
	}{
		{"required", IRField{Name: "title", DatabaseType: "string"}, "REQUIRED", "is required"},
		{"email", IRField{Name: "email", DatabaseType: "string", IsNullable: true, Validations: []IRValidation{{Name: "email"}}}, "EMAIL", "must be a valid email address"},
		{"min_length", IRField{Name: "name", DatabaseType: "string", Validations: []IRValidation{{Name: "min", Value: "3"}}}, "MIN_LENGTH", "must be at least 3 characters"},
		{"min_value", IRField{Name: "qty", DatabaseType: "int", Validations: []IRValidation{{Name: "gte", Value: "10"}}}, "MIN_VALUE", "must be greater than or equal to 10"},
		{"exclusive min_value", IRField{Name: "qty", DatabaseType: "int", Validations: []IRValidation{{Name: "gt", Value: "0"}}}, "MIN_VALUE", "must be greater than 0"},
		{"exclusive max_value", IRField{Name: "rating", DatabaseType: "int", Validations: []IRValidation{{Name: "lt", Value: "6"}}}, "MAX_VALUE", "must be less than 6"},
	}
	for _, c := range cases {
		rules := c.field.ValidationRules()
		found := false
		for _, r := range rules {
			if r.Code == c.wantCode {
				found = true
				if r.Message != c.wantMsg {
					t.Errorf("%s: message = %q, want %q", c.name, r.Message, c.wantMsg)
				}
			}
		}
		if !found {
			t.Errorf("%s: rule with code %q not found in %+v", c.name, c.wantCode, rules)
		}
	}
}

func TestValidationRulesNullableNotRequired(t *testing.T) {
	f := IRField{Name: "nick", DatabaseType: "string", IsNullable: true}
	for _, r := range f.ValidationRules() {
		if r.Kind == "required" {
			t.Error("nullable field should not be required")
		}
	}
}

func TestAllIndexesMergesUniqueFields(t *testing.T) {
	e := IREntity{
		Name:       "Order",
		NamePlural: "Orders",
		Fields: []IRField{
			{Name: "id", DatabaseType: "uuid", IsPrimary: true},
			{Name: "sku", DatabaseType: "string", IsUnique: true},
			{Name: "ref", DatabaseType: "string"},
		},
		Indexes: []IRIndex{
			{Name: "idx_orders_ref", Fields: []string{"ref"}, Sort: []string{"asc"}, Unique: false},
		},
	}
	got := e.AllIndexes()
	if len(got) != 2 {
		t.Fatalf("got %d indexes, want 2 (declared ref + implicit sku): %+v", len(got), got)
	}
	var skuIdx, refIdx bool
	for _, idx := range got {
		switch idx.Name {
		case "idx_orders_sku":
			skuIdx = true
			if !idx.Unique || len(idx.Fields) != 1 || idx.Fields[0] != "sku" {
				t.Errorf("sku index = %+v, want unique [sku]", idx)
			}
		case "idx_orders_ref":
			refIdx = true
		}
	}
	if !skuIdx || !refIdx {
		t.Errorf("missing indexes, got %+v", got)
	}
}

func TestAllIndexesDedupesDeclaredUnique(t *testing.T) {
	e := IREntity{
		Name:       "Order",
		NamePlural: "Orders",
		Fields:     []IRField{{Name: "sku", DatabaseType: "string", IsUnique: true}},
		Indexes:    []IRIndex{{Name: "ix_custom", Fields: []string{"sku"}, Unique: true}},
	}
	got := e.AllIndexes()
	if len(got) != 1 {
		t.Fatalf("got %d indexes, want 1 (no duplicate implicit): %+v", len(got), got)
	}
	if got[0].Name != "ix_custom" {
		t.Errorf("declared index should win, got %q", got[0].Name)
	}
}

func TestPermissionPlan(t *testing.T) {
	p := &IRPermissions{
		Read:   []string{"*"},
		Create: []string{"Admin", "@Owner"},
		Update: []string{},
		Delete: []string{"Admin"},
	}

	read := p.Plan("read")
	if !read.IsPublic || read.HasOwner || len(read.Roles) != 0 {
		t.Errorf("read plan = %+v, want public only", read)
	}

	create := p.Plan("create")
	if create.IsPublic || !create.HasOwner || len(create.Roles) != 1 || create.Roles[0] != "Admin" {
		t.Errorf("create plan = %+v, want Admin + @Owner", create)
	}

	update := p.Plan("update")
	if update == nil || update.IsPublic || update.HasOwner || len(update.Roles) != 0 {
		t.Errorf("update plan = %+v, want empty plan (no roles)", update)
	}

	if p.Plan("bogus") != nil {
		t.Error("unknown operation should return nil")
	}
}

func TestPermissionPlanNil(t *testing.T) {
	var p *IRPermissions
	if p.Plan("read") != nil {
		t.Error("nil permissions should yield nil plan")
	}
}

func TestEndpointsContract(t *testing.T) {
	e := IREntity{Name: "Order", NamePlural: "Orders", Features: map[string]bool{"optimistic_lock": true}}
	eps := e.Endpoints()
	if len(eps) != 6 {
		t.Fatalf("got %d endpoints, want 6", len(eps))
	}

	byOp := map[string]IREndpoint{}
	for _, ep := range eps {
		byOp[ep.Operation] = ep
	}

	list := byOp["list"]
	if list.Method != "GET" || list.Path != "api/orders" || list.SuccessStatus != 200 || !list.Paginated {
		t.Errorf("list = %+v", list)
	}

	create := byOp["create"]
	if create.Method != "POST" || create.SuccessStatus != 201 || !create.HasBody {
		t.Errorf("create = %+v", create)
	}

	update := byOp["update"]
	if !update.Concurrency || !update.HasStatus(409) {
		t.Errorf("update should allow 409 with optimistic lock: %+v", update)
	}

	del := byOp["delete"]
	if del.SuccessStatus != 204 || del.HasBody {
		t.Errorf("delete = %+v", del)
	}
}

func TestIREntity_PrimaryKey(t *testing.T) {
	e := IREntity{Fields: []IRField{{Name: "name"}, {Name: "id", IsPrimary: true}}}
	pk := e.PrimaryKey()
	if pk == nil || pk.Name != "id" {
		t.Errorf("PrimaryKey() = %v, want id", pk)
	}
	if (IREntity{Fields: []IRField{{Name: "name"}}}).PrimaryKey() != nil {
		t.Error("no primary key should yield nil")
	}
}

func TestIREntity_FieldByName(t *testing.T) {
	e := IREntity{Fields: []IRField{{Name: "firstName"}, {Name: "ID"}}}
	if f := e.FieldByName("FirstName"); f == nil || f.Name != "firstName" {
		t.Errorf("FieldByName(FirstName) = %v, want firstName (case-insensitive)", f)
	}
	if f := e.FieldByName("id"); f == nil || f.Name != "ID" {
		t.Errorf("FieldByName(id) = %v, want ID (case-insensitive)", f)
	}
	if e.FieldByName("missing") != nil {
		t.Error("FieldByName(missing) should be nil")
	}
}

func TestIRProject_AuthEntity(t *testing.T) {
	p := &IRProject{
		Auth:     &IRAuthConfig{Type: "jwt", Entity: "User"},
		Entities: []IREntity{{Name: "Order"}, {Name: "User"}},
	}
	if e := p.AuthEntity(); e == nil || e.Name != "User" {
		t.Errorf("AuthEntity() = %v, want User", e)
	}
	if (&IRProject{}).AuthEntity() != nil {
		t.Error("no-auth project should yield nil AuthEntity")
	}
	// AuthEntity resolves by the configured auth entity NAME.
	p.Auth.Entity = "Missing"
	if p.AuthEntity() != nil {
		t.Error("AuthEntity should be nil when the configured entity does not exist")
	}
}

func TestIRPermissions_HasOwnerToken(t *testing.T) {
	if (&IRPermissions{Read: []string{"Admin"}, Create: []string{"*"}}).HasOwnerToken() {
		t.Error("no @Owner token should yield false")
	}
	if !(&IRPermissions{Update: []string{"@Owner"}}).HasOwnerToken() {
		t.Error("any op with @Owner should yield true")
	}
	var nilP *IRPermissions
	if nilP.HasOwnerToken() {
		t.Error("nil permissions should yield false")
	}
}

func TestAuthEndpoints(t *testing.T) {
	p := &IRProject{Auth: &IRAuthConfig{
		Type:   "jwt",
		Entity: "User",
		Endpoints: IRAuthEndpoints{
			HasLogin: true, HasRegister: true, HasMe: true, HasSetup: true,
		},
	}}
	decls := p.AuthEndpoints()
	if len(decls) != 4 {
		t.Fatalf("got %d auth endpoints, want 4", len(decls))
	}
	for _, d := range decls {
		if d.Operation == "me" && d.Anonymous {
			t.Error("/me must not be anonymous")
		}
		if d.Operation == "login" && !d.Anonymous {
			t.Error("/login must be anonymous")
		}
	}
	if got := (&IRProject{}).AuthEndpoints(); got != nil {
		t.Error("no-auth project should yield nil auth endpoints")
	}
}
