package gdb

import "context"

// TenantOption provides configuration options for multi-tenancy support.
type TenantOption struct {
	Enable            bool                          // Enable controls whether the tenant feature is enabled.
	PropagateToJoins  bool                          // PropagateToJoins determines whether tenant conditions should be applied to joined tables.
	IgnoreTables      []string                      // IgnoreTables lists table names that should be excluded from tenant filtering.
	TenantIdValue     any                           // TenantIdValue specifies a fixed tenant ID value to be used for queries.
	TenantIdField     string                        // TenantIdField specifies the column name used for tenant identification.
	TenantIdValueFunc func(ctx context.Context) any // TenantIdValueFunc is a function that dynamically returns the tenant ID based on context.
}

func (m *Model) Tenant(options ...TenantOption) *Model {
	model := m.getModel()
	if len(options) > 0 {
		model.tenantOption = options[0]
		return model
	}
	model.tenantOption.Enable = true
	return model
}

func (m *Model) tenantMaintainer() *TenantMaintainer {
	return &TenantMaintainer{
		Model: m,
	}
}

type TenantMaintainer struct {
	*Model
}

func (tm *TenantMaintainer) GetTenantFieldName(ctx context.Context, schema string, table string) string {
	config := tm.db.GetConfig()
	if config.TenantIdField != "" {
		return config.TenantIdField
	}
	return "tenant_id"
}

type iTenantMaintainer interface {
	GetTenantFieldName(ctx context.Context, schema string, table string) string
	GetTenantCondition(ctx context.Context) string
}
