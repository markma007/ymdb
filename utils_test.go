package ymdb

import (
	"reflect"
	"testing"
)

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
