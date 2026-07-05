package ymdb

import (
	"reflect"
	"testing"
)

func TestPostContentAndDataFormats(t *testing.T) {
	setupTestDB(t)
	post, err := NewPostE("dataset")
	if err != nil {
		t.Fatal(err)
	}
	if err := post.SetContent("# Heading", PostMimeMarkdown); err != nil {
		t.Fatal(err)
	}
	if err := post.SetContent("<book><title>Blue Sky</title></book>", PostMimeXML); err != nil {
		t.Fatal(err)
	}
	if post.Mime != PostMimeXML {
		t.Fatalf("XML content MIME=%q", post.Mime)
	}
	input := map[string]any{"name": "YMDB", "count": float64(2)}
	if err := post.SetJSONData(input); err != nil {
		t.Fatal(err)
	}
	if err := post.SaveE(); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := post.DecodeJSONData(&decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, input) {
		t.Fatalf("decoded JSON=%#v", decoded)
	}
	records := [][]string{{"name", "score"}, {"alex", "10"}}
	if err := post.SetCSVData(records); err != nil {
		t.Fatal(err)
	}
	decodedCSV, err := post.DecodeCSVData()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decodedCSV, records) {
		t.Fatalf("decoded CSV=%#v", decodedCSV)
	}
	tabularRecords := [][]string{{"a1", "a2", "a3"}, {"b1", "b2", "b3"}, {"c1", "c2", "c3"}}
	post.SetTabularData(tabularRecords)
	if post.Format != PostFormatTabular || post.Data != "a1,a2,a3;b1,b2,b3;c1,c2,c3" {
		t.Fatalf("tabular data=%#v", post)
	}
	decodedTabular, err := post.DecodeTabularData()
	if err != nil || !reflect.DeepEqual(decodedTabular, tabularRecords) {
		t.Fatalf("decoded tabular=%#v err=%v", decodedTabular, err)
	}
	escapedRecords := [][]string{{"comma,value", "semicolon;value", `quoted"value`}}
	post.SetTabularData(escapedRecords)
	decodedTabular, err = post.DecodeTabularData()
	if err != nil || !reflect.DeepEqual(decodedTabular, escapedRecords) {
		t.Fatalf("decoded escaped tabular=%#v data=%q err=%v", decodedTabular, post.Data, err)
	}
	post.SetTextData("plain")
	if post.Format != PostFormatText || post.Data != "plain" {
		t.Fatalf("text data=%#v", post)
	}
}

func TestPostRejectsInvalidDataFormat(t *testing.T) {
	setupTestDB(t)
	post, err := NewPostE("dataset")
	if err != nil {
		t.Fatal(err)
	}
	post.Format = "xml"
	if err := post.SaveE(); err == nil {
		t.Fatal("expected unsupported format error")
	}
	post.Format = PostFormatJSON
	post.Data = "not-json"
	if err := post.SaveE(); err == nil {
		t.Fatal("expected invalid JSON error")
	}
	post.Format = PostFormatTabular
	post.Data = `"unclosed`
	if err := post.SaveE(); err == nil {
		t.Fatal("expected invalid tabular data error")
	}
}
