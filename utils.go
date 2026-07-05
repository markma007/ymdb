package ymdb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// PrettyPrintJSON formats a JSON string using indentSpaces spaces per nesting
// level. It returns an error for invalid JSON or a negative indentation width.
func PrettyPrintJSON(jsonString string, indentSpaces int) (string, error) {
	if indentSpaces < 0 {
		return "", fmt.Errorf("indentation width cannot be negative: %d", indentSpaces)
	}

	var formatted bytes.Buffer
	indent := strings.Repeat(" ", indentSpaces)
	if err := json.Indent(&formatted, []byte(jsonString), "", indent); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}
	return formatted.String(), nil
}

func CommaSeparatedNumberStringToSlice(s string) ([]int, error) {
	// 1. Split the string into a slice of strings using the comma delimiter
	strSlice := strings.Split(s, ",")
	// 2. Convert string slices to integer slices (with error handling)
	intSlice := make([]int, 0, len(strSlice))
	for _, numStr := range strSlice {
		// Trim any leading/trailing whitespace before conversion
		trimmedStr := strings.TrimSpace(numStr)
		if trimmedStr == "" {
			continue // Skip empty strings that might result from trailing commas, etc.
		}
		num, err := strconv.Atoi(trimmedStr)
		if err != nil {
			// Return an error if any part of the string isn't a valid number
			return nil, fmt.Errorf("invalid number in string: %s", numStr)
		}
		intSlice = append(intSlice, num)
	}
	return intSlice, nil
}

func SortCommaSeparatedNumbers(s string) (string, error) {
	// // 1. Split the string into a slice of strings using the comma delimiter
	// strSlice := strings.Split(s, ",")

	// // 2. Convert string slices to integer slices (with error handling)
	// intSlice := make([]int, 0, len(strSlice))
	// for _, numStr := range strSlice {
	// 	// Trim any leading/trailing whitespace before conversion
	// 	trimmedStr := strings.TrimSpace(numStr)
	// 	if trimmedStr == "" {
	// 		continue // Skip empty strings that might result from trailing commas, etc.
	// 	}

	// 	num, err := strconv.Atoi(trimmedStr)
	// 	if err != nil {
	// 		// Return an error if any part of the string isn't a valid number
	// 		return "", fmt.Errorf("invalid number in string: %s", numStr)
	// 	}
	// 	intSlice = append(intSlice, num)
	// }
	intSlice, err := CommaSeparatedNumberStringToSlice(s)
	if err != nil {
		return "", err
	}

	// 3. Sort the integer slice numerically in place
	sort.Ints(intSlice)

	// 4. Convert the sorted integers back to strings
	sortedStrSlice := make([]string, 0, len(intSlice))
	for _, num := range intSlice {
		strNum := strconv.Itoa(num)
		sortedStrSlice = append(sortedStrSlice, strNum)
	}

	// 5. Join the sorted string slice back into a single comma-separated string
	sortedString := strings.Join(sortedStrSlice, ",")

	return sortedString, nil
}

// walkData recursively traverses a nested data structure
func WalkData(data any, depth int) {
	indent := strings.Repeat("  ", depth) // For visual clarity

	switch v := data.(type) {
	case map[string]any:
		fmt.Printf("%sMap (length %d):\n", indent, len(v))
		for key, value := range v {
			fmt.Printf("%s  Key: %s\n", indent, key)
			WalkData(value, depth+1) // Recurse on map values
		}
	case []any:
		fmt.Printf("%sSlice (length %d):\n", indent, len(v))
		for i, value := range v {
			fmt.Printf("%s  Index: %d\n", indent, i)
			WalkData(value, depth+1) // Recurse on slice elements
		}
	case string:
		fmt.Printf("%sString: %s\n", indent, v)
	case int:
		fmt.Printf("%sInt: %d\n", indent, v)
	case float64:
		fmt.Printf("%sFloat64: %f\n", indent, v)
	case bool:
		fmt.Printf("%sBool: %v\n", indent, v)
	case nil:
		fmt.Printf("%sNil\n", indent)
	default:
		// Handle any other unexpected types
		fmt.Printf("%sUnknown type: %T\n", indent, v)
	}
}

// func main() {
// 	// Example of deeply nested data structure (often the result of json.Unmarshal)
// 	arbitraryData := map[string]any{
// 		"name": "Project A",
// 		"details": map[string]any{
// 			"status": "active",
// 			"priority": 2,
// 			"tags": []any{"go", "json", "arbitrary"},
// 		},
// 		"metadata": []any{
// 			map[string]any{"key": "v1", "value": 100},
// 			map[string]any{"key": "v2", "value": true},
// 			nil,
// 		},
// 	}
//
// 	WalkData(arbitraryData, 0)
// }
