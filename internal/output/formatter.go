package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"

	"github.com/olekukonko/tablewriter"
)

// Format represents the output format
type Format string

const (
	JSON  Format = "json"
	Table Format = "table"
)

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

		// Get JSON tag name if available
		fieldName := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
			fieldName = jsonTag
		}

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

// extractHeaders extracts field names from a struct type
func extractHeaders(typ reflect.Type) []string {
	var headers []string
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}

		// Use JSON tag if available
		fieldName := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
			// Remove omitempty and other options
			for idx := 0; idx < len(jsonTag); idx++ {
				if jsonTag[idx] == ',' {
					fieldName = jsonTag[:idx]
					break
				}
			}
			if fieldName == "" || fieldName == jsonTag {
				fieldName = jsonTag
			}
		}

		headers = append(headers, fieldName)
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
		if val.Len() == 0 {
			return "[]"
		}
		// Format as comma-separated list for simple types
		if val.Len() <= 5 {
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
		return fmt.Sprintf("[%d items]", val.Len())
	case reflect.Map:
		if val.Len() == 0 {
			return "{}"
		}
		return fmt.Sprintf("{%d entries}", val.Len())
	case reflect.Struct:
		// For time.Time and similar, use String() method
		if stringer, ok := val.Interface().(fmt.Stringer); ok {
			return stringer.String()
		}
		return fmt.Sprintf("%v", val.Interface())
	case reflect.Bool:
		if val.Bool() {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(val.Interface())
	}
}
