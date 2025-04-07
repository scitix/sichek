package metrics

import (
	"reflect"
	"testing"
)

func TestNewGaugeVecMetricExporter(t *testing.T) {
	prefix := "test_prefix"
	labelKeys := []string{"label1", "label2"}
	exporter := NewGaugeVecMetricExporter(prefix, labelKeys)

	if exporter.prefix != prefix {
		t.Errorf("expected prefix %s, got %s", prefix, exporter.prefix)
	}
	if !reflect.DeepEqual(exporter.labelKeys, labelKeys) {
		t.Errorf("expected labelKeys %v, got %v", labelKeys, exporter.labelKeys)
	}
	if len(exporter.metricsMap) != 0 {
		t.Errorf("expected empty metricsMap, got %v", exporter.metricsMap)
	}
}

func TestGaugeVecMetricExporter_SetMetric(t *testing.T) {
	exporter := NewGaugeVecMetricExporter("test", []string{"label1"})
	exporter.SetMetric("test_metric", []string{"value1"}, 42.0)

	if _, exists := exporter.metricsMap["test_metric"]; !exists {
		t.Fatalf("expected metric test_metric to exist")
	}
}

func TestGaugeVecMetricExporter_ExportStruct(t *testing.T) {
	type TestStruct struct {
		Field1 int     `json:"field1"`
		Field2 float64 `json:"field2"`
		Field3 bool    `json:"field3"`
	}

	exporter := NewGaugeVecMetricExporter("test", []string{"label1"})
	testStruct := TestStruct{Field1: 10, Field2: 20.5, Field3: true}
	exporter.ExportStruct(testStruct, []string{"value1"}, "json")

	expectedMetrics := map[string]float64{
		"test_field1": 10.0,
		"test_field2": 20.5,
		"test_field3": 1.0,
	}

	for name := range expectedMetrics {
		if _, exists := exporter.metricsMap[name]; !exists {
			t.Fatalf("expected metric %s to exist", name)
		}
	}
}

func TestSliceStructToMetricsMap(t *testing.T) {
	type NestedStruct struct {
		InnerField1 int `json:"inner_field0"`
		InnerField2 int `json:"inner_field1"`
	}
	type TestStruct struct {
		Field []NestedStruct `json:"field"`
	}

	exporter := NewGaugeVecMetricExporter("test", []string{"label1"})
	testStruct := TestStruct{
		Field: []NestedStruct{
			{InnerField1: 100, InnerField2: 200},
			{InnerField1: 300, InnerField2: 400},
		},
	}
	exporter.ExportStruct(testStruct, []string{"value1"}, "json")
	for name := range exporter.metricsMap {
		t.Logf("Metric name: %s", name)
	}
	expectedMetrics := map[string]float64{
		"test_field_0_inner_field0": 100,
		"test_field_0_inner_field1": 200,
		"test_field_1_inner_field0": 300,
		"test_field_1_inner_field1": 400,
	}

	for name := range expectedMetrics {
		if _, exists := exporter.metricsMap[name]; !exists {
			t.Fatalf("expected metric %s to exist", name)
		}
	}
}

func TestMapStructToMetricsMap(t *testing.T) {
	type NestedStruct struct {
		InnerField1 int `json:"inner_field0"`
		InnerField2 int `json:"inner_field1"`
	}
	type TestStruct struct {
		Field map[string]NestedStruct `json:"field"`
	}

	exporter := NewGaugeVecMetricExporter("test", []string{"label1"})
	testStruct := TestStruct{
		Field: map[string]NestedStruct{
			"field0": {InnerField1: 100, InnerField2: 200},
			"field1": {InnerField1: 300, InnerField2: 400},
		},
	}
	exporter.ExportStruct(testStruct, []string{"value1"}, "json")
	for name := range exporter.metricsMap {
		t.Logf("Metric name: %s", name)
	}
	expectedMetrics := map[string]float64{
		"test_field_field0_inner_field0": 100,
		"test_field_field0_inner_field1": 200,
		"test_field_field1_inner_field0": 300,
		"test_field_field1_inner_field1": 400,
	}

	for name := range expectedMetrics {
		if _, exists := exporter.metricsMap[name]; !exists {
			t.Fatalf("expected metric %s to exist", name)
		}
	}
}

func TestStructToMetricsMap(t *testing.T) {
	type NestedStruct struct {
		InnerField1 int `json:"inner_field1"`
		InnerField2 int `json:"inner_field2"`
	}
	type TestStruct struct {
		Field1 int          `json:"field1"`
		Field2 NestedStruct `json:"field2"`
	}

	testStruct := TestStruct{
		Field1: 42,
		Field2: NestedStruct{InnerField1: 100, InnerField2: 200},
	}

	metrics := make(map[string]float64)
	StructToMetricsMap(reflect.ValueOf(testStruct), "", "json", metrics)

	expectedMetrics := map[string]float64{
		"field1":              42.0,
		"field2_inner_field1": 100.0,
		"field2_inner_field2": 200.0,
	}

	for key, expectedValue := range expectedMetrics {
		if metrics[key] != expectedValue {
			t.Errorf("expected metric %s to have value %v, got %v", key, expectedValue, metrics[key])
		}
	}
}

func TestSanitizeMetricName(t *testing.T) {
	tests := map[string]string{
		"metric.name":     "metric_name",
		"metric-name":     "metric_name",
		"metric+name":     "metric_name",
		"metric[name]":    "metric_name",
		"metric name":     "metric_name",
		"MetricName":      "metricname",
		"metric[complex]": "metric_complex",
	}

	for input, expected := range tests {
		output := sanitizeMetricName(input)
		if output != expected {
			t.Errorf("expected sanitized name %s, got %s", expected, output)
		}
	}
}
