package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
)

// Format represents the output format
type Format string

const (
	JSON  Format = "json"
	Table Format = "table"
)

// TableFormatter is an interface for types that want custom table formatting
type TableFormatter interface {
	FormatForTable() string
}

// Print outputs data in the specified format
func Print(data interface{}, format Format) error {
	return PrintTo(os.Stdout, data, format)
}

// PrintTo outputs data to a specific writer in the specified format
func PrintTo(w io.Writer, data interface{}, format Format) error {
	switch format {
	case JSON:
		return printJSON(w, data)
	case Table:
		return printTable(w, data)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// printJSON outputs data as pretty-printed JSON
func printJSON(w io.Writer, data interface{}) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}
	return nil
}

// printTable outputs data as a table
func printTable(w io.Writer, data interface{}) error {
	// Handle nil data
	if data == nil {
		return nil
	}

	// Use reflection to handle different data types
	val := reflect.ValueOf(data)
	typ := val.Type()

	// Dereference pointers
	if typ.Kind() == reflect.Ptr {
		val = val.Elem()
		typ = val.Type()
	}

	// Handle different data structures
	switch typ.Kind() {
	case reflect.Slice, reflect.Array:
		return printSliceTable(w, val)
	case reflect.Struct:
		return printStructTable(w, val)
	case reflect.Map:
		return printMapTable(w, val)
	default:
		// For simple types, just print them
		fmt.Fprintln(w, val.Interface())
		return nil
	}
}

// printSliceTable prints a slice as a table
func printSliceTable(w io.Writer, val reflect.Value) error {
	if val.Len() == 0 {
		fmt.Fprintln(w, "No results found")
		return nil
	}

	// Get the first element to determine structure
	firstElem := val.Index(0)
	if firstElem.Kind() == reflect.Ptr {
		firstElem = firstElem.Elem()
	}

	table := tablewriter.NewWriter(w)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("\t")
	table.SetNoWhiteSpace(true)

	// Handle structs
	if firstElem.Kind() == reflect.Struct {
		// Extract headers from struct fields
		headers := extractHeaders(firstElem.Type())
		table.SetHeader(headers)

		// Add rows
		for i := 0; i < val.Len(); i++ {
			elem := val.Index(i)
			if elem.Kind() == reflect.Ptr {
				elem = elem.Elem()
			}
			row := extractRow(elem, headers)
			table.Append(row)
		}
	} else {
		// For simple types, create a single column table
		table.SetHeader([]string{"Value"})
		for i := 0; i < val.Len(); i++ {
			elem := val.Index(i)
			table.Append([]string{fmt.Sprint(elem.Interface())})
		}
	}

	table.Render()
	return nil
}

// printStructTable prints a single struct as a key-value table
func printStructTable(w io.Writer, val reflect.Value) error {
	table := tablewriter.NewWriter(w)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("\t")
	table.SetNoWhiteSpace(true)

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get field name (properly parsed from JSON tag)
		fieldName := getFieldName(field)

		// Format field value
		fieldValue := formatValue(fieldVal)
		table.Append([]string{fieldName, fieldValue})
	}

	table.Render()
	return nil
}

// printMapTable prints a map as a key-value table
func printMapTable(w io.Writer, val reflect.Value) error {
	table := tablewriter.NewWriter(w)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("\t")
	table.SetNoWhiteSpace(true)

	iter := val.MapRange()
	for iter.Next() {
		key := iter.Key()
		value := iter.Value()
		table.Append([]string{fmt.Sprint(key.Interface()), formatValue(value)})
	}

	table.Render()
	return nil
}

// parseJSONTag extracts the field name from a JSON tag, removing options like omitempty
func parseJSONTag(jsonTag string) string {
	// Remove omitempty and other options
	for idx := 0; idx < len(jsonTag); idx++ {
		if jsonTag[idx] == ',' {
			return jsonTag[:idx]
		}
	}
	return jsonTag
}

// getFieldName returns the field name to use, preferring JSON tag over struct field name
func getFieldName(field reflect.StructField) string {
	jsonTag := field.Tag.Get("json")
	if jsonTag != "" && jsonTag != "-" {
		return parseJSONTag(jsonTag)
	}
	return field.Name
}

// extractHeaders extracts field names from a struct type
func extractHeaders(typ reflect.Type) []string {
	var headers []string
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		headers = append(headers, getFieldName(field))
	}
	return headers
}

// extractRow extracts field values from a struct
func extractRow(val reflect.Value, headers []string) []string {
	row := make([]string, 0, len(headers))
	typ := val.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}

		fieldVal := val.Field(i)
		row = append(row, formatValue(fieldVal))
	}

	return row
}

// formatSliceValue formats a slice or array value
func formatSliceValue(val reflect.Value) string {
	if val.Len() == 0 {
		return "[]"
	}

	// Check the element type
	elemType := val.Type().Elem()
	if elemType.Kind() == reflect.Ptr {
		elemType = elemType.Elem()
	}

	// For struct slices, format each item with key fields
	if elemType.Kind() == reflect.Struct {
		return formatStructSlice(val)
	}

	// Format as comma-separated list for simple types (≤3 items)
	if val.Len() <= 3 {
		return formatShortSlice(val)
	}

	return fmt.Sprintf("[%d items]", val.Len())
}

// formatStructSlice formats a slice of structs in a compact, single-line way
func formatStructSlice(val reflect.Value) string {
	// Show count only for lists > 3 items
	if val.Len() > 3 {
		return fmt.Sprintf("[%d items]", val.Len())
	}

	var items []string

	for i := 0; i < val.Len(); i++ {
		elem := val.Index(i)
		if elem.Kind() == reflect.Ptr {
			if elem.IsNil() {
				continue
			}
			elem = elem.Elem()
		}

		if elem.Kind() != reflect.Struct {
			continue
		}

		// Check if the element implements TableFormatter interface
		if elem.CanInterface() {
			if formatter, ok := elem.Interface().(TableFormatter); ok {
				items = append(items, formatter.FormatForTable())
				continue
			}
		}

		// Fallback: extract key identifying fields (name, enabled, etc.)
		typ := elem.Type()
		fields := make(map[string]string)
		var enabled string

		for j := 0; j < elem.NumField(); j++ {
			field := typ.Field(j)
			if !field.IsExported() {
				continue
			}

			fieldName := getFieldName(field)
			fieldVal := elem.Field(j)

			// Capture key fields for compact display
			switch fieldName {
			case "name":
				val := formatValue(fieldVal)
				if val != "" {
					fields[fieldName] = val
				}
			case "enabled":
				// Track enabled status with color
				if fieldVal.Kind() == reflect.Bool {
					if fieldVal.Bool() {
						enabled = color.GreenString("enabled")
					} else {
						enabled = color.RedString("disabled")
					}
				}
			}
		}

		// Build compact representation
		var itemStr string
		if name, ok := fields["name"]; ok && name != "" {
			itemStr = name
			// Add enabled/disabled status if present
			if enabled != "" {
				itemStr += " (" + enabled + ")"
			}
		}

		if itemStr != "" {
			items = append(items, itemStr)
		}
	}

	if len(items) == 0 {
		return "[]"
	}

	// Format as comma-separated list (single line)
	return strings.Join(items, ", ")
}

// formatShortSlice formats a small slice as a comma-separated list
func formatShortSlice(val reflect.Value) string {
	result := "["
	for i := 0; i < val.Len(); i++ {
		if i > 0 {
			result += ", "
		}
		result += formatValue(val.Index(i))
	}
	result += "]"
	return result
}

// formatStructValue formats a struct value
func formatStructValue(val reflect.Value) string {
	// For time.Time and similar, use String() method
	if stringer, ok := val.Interface().(fmt.Stringer); ok {
		return stringer.String()
	}
	return fmt.Sprintf("%v", val.Interface())
}

// formatBoolValue formats a boolean value with color
func formatBoolValue(val reflect.Value) string {
	if val.Bool() {
		return color.GreenString("true")
	}
	return color.RedString("false")
}

// formatValue formats a reflect.Value as a string
func formatValue(val reflect.Value) string {
	if !val.IsValid() {
		return ""
	}

	switch val.Kind() {
	case reflect.Ptr:
		if val.IsNil() {
			return ""
		}
		return formatValue(val.Elem())
	case reflect.Slice, reflect.Array:
		return formatSliceValue(val)
	case reflect.Map:
		if val.Len() == 0 {
			return "{}"
		}
		return fmt.Sprintf("{%d entries}", val.Len())
	case reflect.Struct:
		return formatStructValue(val)
	case reflect.Bool:
		return formatBoolValue(val)
	default:
		return fmt.Sprint(val.Interface())
	}
}
