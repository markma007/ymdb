package ymdb

import (
	"reflect"
	"strings"
	"testing"
)

func TestPrettyPrintJSON(t *testing.T) {
	got, err := PrettyPrintJSON(`{"name":"YMDB","settings":{"enabled":true}}`, 4)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n    \"name\": \"YMDB\",\n    \"settings\": {\n        \"enabled\": true\n    }\n}"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPrettyPrintJSONRejectsInvalidInput(t *testing.T) {
	if _, err := PrettyPrintJSON(`{"broken":}`, 2); err == nil {
		t.Fatal("expected invalid JSON error")
	}
	if _, err := PrettyPrintJSON(`{}`, -1); err == nil || !strings.Contains(err.Error(), "negative") {
		t.Fatalf("expected negative indentation error, got %v", err)
	}
}

func TestCSNStringToNumbers(t *testing.T) {
	want := []int{10, 7, 20, 16, 1}
	got, err := CommaSeparatedNumberStringToSlice("10,7,20,16,1")
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v (err=%v)", got, want, err)
	}
}

func TestSort(t *testing.T) {
	got, err := SortCommaSeparatedNumbers("10,7,20,16,1")
	if err != nil || got != "1,7,10,16,20" {
		t.Fatalf("got %q, want %q (err=%v)", got, "1,7,10,16,20", err)
	}
}
