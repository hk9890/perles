package bql

import (
	"fmt"
	"strings"
)

// ValidFields defines the set of valid field names in BQL.
var ValidFields = map[string]FieldType{
	"type":            FieldEnum,
	"status_category": FieldEnum,
	"priority":        FieldPriority,
	"status":          FieldEnum,
	"blocked":         FieldBool,
	"ready":           FieldBool,
	"pinned":          FieldBool,
	"is_template":     FieldBool,
	"label":           FieldString,
	"title":           FieldString,
	"id":              FieldString,
	"assignee":        FieldString,
	"sender":          FieldString,
	"description":     FieldString,
	"design":          FieldString,
	"notes":           FieldString,
	"created_by":      FieldString,
	"hook_bead":       FieldString,
	"role_bead":       FieldString,
	"agent_state":     FieldString,
	"last_activity":   FieldDate,
	"role_type":       FieldString,
	"rig":             FieldString,
	"mol_type":        FieldString,
	"created":         FieldDate,
	"updated":         FieldDate,
}

// FieldType categorizes fields for validation.
type FieldType int

const (
	FieldString FieldType = iota
	FieldEnum
	FieldPriority
	FieldBool
	FieldDate
)

// KnownTypeValues are built-in type values in beads v1.
//
// Validation intentionally allows custom type strings for compatibility with
// project-specific custom_types.
var KnownTypeValues = map[string]bool{
	"bug":       true,
	"feature":   true,
	"task":      true,
	"epic":      true,
	"chore":     true,
	"decision":  true,
	"spike":     true,
	"story":     true,
	"milestone": true,
}

// KnownStatusValues are built-in status values used by current beads versions.
//
// Validation intentionally allows custom status strings for compatibility, but this
// list is kept for discoverability and error messaging where a status-like value is
// required.
var KnownStatusValues = map[string]bool{
	"open":        true,
	"in_progress": true,
	"closed":      true,
	"blocked":     true,
	"deferred":    true,
	"pinned":      true,
	"hooked":      true,
}

// KnownStatusCategoryValues are the canonical status categories in beads v1.
var KnownStatusCategoryValues = map[string]bool{
	"active": true,
	"wip":    true,
	"done":   true,
	"frozen": true,
}

// ValidPriorityValues are the valid values for the priority field.
var ValidPriorityValues = map[string]bool{
	"P0": true, "p0": true,
	"P1": true, "p1": true,
	"P2": true, "p2": true,
	"P3": true, "p3": true,
	"P4": true, "p4": true,
}

// Validate validates a BQL query and returns an error if invalid.
func Validate(query *Query) error {
	if query.Filter != nil {
		if err := validateExpr(query.Filter); err != nil {
			return err
		}
	}

	for _, term := range query.OrderBy {
		if err := validateOrderField(term.Field); err != nil {
			return err
		}
	}

	return nil
}

// validateExpr validates an expression recursively.
func validateExpr(expr Expr) error {
	switch e := expr.(type) {
	case *BinaryExpr:
		if err := validateExpr(e.Left); err != nil {
			return err
		}
		return validateExpr(e.Right)

	case *NotExpr:
		return validateExpr(e.Expr)

	case *CompareExpr:
		return validateCompare(e)

	case *InExpr:
		return validateIn(e)
	}

	return nil
}

// validateCompare validates a comparison expression.
func validateCompare(e *CompareExpr) error {
	// Check field exists
	fieldType, ok := ValidFields[e.Field]
	if !ok {
		return fmt.Errorf("unknown field: %q (valid: %s)", e.Field, validFieldNames())
	}

	// Check operator is valid for field type
	if err := validateOperator(e.Field, fieldType, e.Op); err != nil {
		return err
	}

	// Check value is valid for field type
	return validateValue(e.Field, fieldType, e.Value)
}

// validateIn validates an IN expression.
func validateIn(e *InExpr) error {
	// Check field exists
	fieldType, ok := ValidFields[e.Field]
	if !ok {
		return fmt.Errorf("unknown field: %q (valid: %s)", e.Field, validFieldNames())
	}

	// IN is only valid for enum, string, and priority fields
	if fieldType == FieldBool || fieldType == FieldDate {
		return fmt.Errorf("operator IN is not valid for field %q", e.Field)
	}

	// Validate each value
	for _, v := range e.Values {
		if err := validateValue(e.Field, fieldType, v); err != nil {
			return err
		}
	}

	return nil
}

// validateOperator checks if an operator is valid for a field type.
func validateOperator(field string, fieldType FieldType, op TokenType) error {
	switch fieldType {
	case FieldBool:
		// Boolean fields only support = and !=
		if op != TokenEq && op != TokenNeq {
			return fmt.Errorf("operator %q is not valid for boolean field %q (use = or !=)", op, field)
		}

	case FieldEnum:
		// Enum fields support = and !=
		if op != TokenEq && op != TokenNeq {
			return fmt.Errorf("operator %q is not valid for field %q (use = or !=)", op, field)
		}

	case FieldString:
		// String fields support =, !=, ~, !~
		if op != TokenEq && op != TokenNeq && op != TokenContains && op != TokenNotContains {
			return fmt.Errorf("operator %q is not valid for string field %q (use =, !=, ~, or !~)", op, field)
		}

	case FieldPriority:
		// Priority supports all comparison operators
		// (already validated by parser)

	case FieldDate:
		// Date supports comparison operators, but not ~
		if op == TokenContains || op == TokenNotContains {
			return fmt.Errorf("operator %q is not valid for date field %q", op, field)
		}
	}

	return nil
}

// validateValue checks if a value is valid for a field type.
func validateValue(field string, fieldType FieldType, value Value) error {
	switch fieldType {
	case FieldBool:
		if value.Type != ValueBool {
			return fmt.Errorf("field %q requires a boolean value (true or false)", field)
		}

	case FieldPriority:
		// Accept both P0-P4 format and plain integers 0-4
		switch value.Type {
		case ValuePriority:
			// Already validated by parser
		case ValueInt:
			if value.Int < 0 || value.Int > 4 {
				return fmt.Errorf("field %q requires priority 0-4, got %d", field, value.Int)
			}
		default:
			return fmt.Errorf("field %q requires a priority value (P0-P4 or 0-4), got %q", field, value.Raw)
		}

	case FieldDate:
		if value.Type != ValueDate {
			return fmt.Errorf("field %q requires a date value (today, yesterday, -Nd, or ISO date), got %q", field, value.Raw)
		}

	case FieldEnum:
		// Validate enum values
		switch field {
		case "type":
			if value.Type != ValueString {
				return fmt.Errorf("field %q requires a type string value (known built-ins: %s), got %q", field, knownTypeNames(), value.Raw)
			}
			// Intentionally accept any string type for compatibility with custom
			// types from project custom_types configuration.
		case "status_category":
			if value.Type != ValueString {
				return fmt.Errorf("field %q requires a status category string value (valid: %s), got %q", field, knownStatusCategoryNames(), value.Raw)
			}
			if !KnownStatusCategoryValues[value.String] {
				return fmt.Errorf("invalid value %q for field %q (valid: %s)", value.String, field, knownStatusCategoryNames())
			}
		case "status":
			if value.Type != ValueString {
				return fmt.Errorf("field %q requires a status string value (known built-ins: %s), got %q", field, knownStatusNames(), value.Raw)
			}
			// Intentionally accept any string status for compatibility with custom
			// statuses from newer beads versions and project-specific workflows.
		}

	case FieldString:
		// Any string value is valid
	}

	return nil
}

// knownTypeNames returns a comma-separated list of known built-in type names.
func knownTypeNames() string {
	names := make([]string, 0, len(KnownTypeValues))
	for name := range KnownTypeValues {
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}

// knownStatusNames returns a comma-separated list of known built-in status names.
func knownStatusNames() string {
	names := make([]string, 0, len(KnownStatusValues))
	for name := range KnownStatusValues {
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}

func knownStatusCategoryNames() string {
	names := make([]string, 0, len(KnownStatusCategoryValues))
	for name := range KnownStatusCategoryValues {
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}

// validateOrderField checks if a field can be used in ORDER BY.
func validateOrderField(field string) error {
	// Check field exists
	_, ok := ValidFields[field]
	if !ok {
		return fmt.Errorf("unknown field in ORDER BY: %q (valid: %s)", field, validFieldNames())
	}
	return nil
}

// validFieldNames returns a comma-separated list of valid field names.
func validFieldNames() string {
	names := make([]string, 0, len(ValidFields))
	for name := range ValidFields {
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}
