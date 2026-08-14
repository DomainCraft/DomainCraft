package ir

import "testing"

func TestEnumValuesWireValue(t *testing.T) {
	p := &IRProject{
		Enums: map[string][]string{
			"OrderStatus": {"PENDING", "IN_PROGRESS", "Shipped"},
			"Single":      {"Draft"},
		},
	}

	got := p.EnumValues("OrderStatus")
	if len(got) != 3 {
		t.Fatalf("got %d values, want 3", len(got))
	}
	cases := []struct {
		idx  int
		name string
		wire string
	}{
		{0, "PENDING", "pending"},
		{1, "IN_PROGRESS", "in_progress"},
		{2, "Shipped", "shipped"},
	}
	for _, c := range cases {
		if got[c.idx].Name != c.name {
			t.Errorf("value[%d].Name = %q, want %q", c.idx, got[c.idx].Name, c.name)
		}
		if got[c.idx].WireValue != c.wire {
			t.Errorf("value[%d].WireValue = %q, want %q", c.idx, got[c.idx].WireValue, c.wire)
		}
		if got[c.idx].Ordinal != c.idx {
			t.Errorf("value[%d].Ordinal = %d, want %d", c.idx, got[c.idx].Ordinal, c.idx)
		}
	}

	if got := p.EnumValues("Missing"); got != nil {
		t.Errorf("EnumValues(Missing) = %v, want nil", got)
	}
	if got := p.EnumValues("Single"); len(got) != 1 || got[0].WireValue != "draft" {
		t.Errorf("EnumValues(Single) = %+v, want one value with wire \"draft\"", got)
	}
}

func TestEnumValuesNilProject(t *testing.T) {
	var p *IRProject
	if got := p.EnumValues("X"); got != nil {
		t.Errorf("nil project EnumValues = %v, want nil", got)
	}
}

func TestIRIndexDatabaseName(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"idx_orders_sku", "IX_idx_orders_sku"},
		{"idx_order_item_sku_0", "IX_idx_order_item_sku_0"},
	}
	for _, c := range cases {
		if got := (IRIndex{Name: c.name}).DatabaseName(); got != c.want {
			t.Errorf("DatabaseName(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}
