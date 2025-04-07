/*
Copyright 2024 The Scitix Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package metrics

import (
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

type GaugeVecMetricExporter struct {
	prefix     string
	labelKeys  []string
	metricsMap map[string]*prometheus.GaugeVec
	lock       sync.Mutex
}

func NewGaugeVecMetricExporter(prefix string, labelKeys []string) *GaugeVecMetricExporter {
	return &GaugeVecMetricExporter{
		prefix:     prefix,
		labelKeys:  labelKeys,
		metricsMap: make(map[string]*prometheus.GaugeVec),
	}
}

// ExportStruct This method receives a struct (or a nested struct) and a list of label values. It converts the struct into a map of metrics, then registers and sets values for each metric.
func (e *GaugeVecMetricExporter) ExportStruct(v interface{}, labelVals []string, tagPrefix string) {
	metricsValueMap := make(map[string]float64)
	StructToMetricsMap(reflect.ValueOf(v), "", tagPrefix, metricsValueMap)

	for name, val := range metricsValueMap {
		fullName := sanitizeMetricName(e.prefix + "_" + name)
		e.SetMetric(fullName, labelVals, val)
	}
}

// setMetric set metric value to a metric name with a list of label values.
func (e *GaugeVecMetricExporter) SetMetric(name string, labelVals []string, value float64) {
	e.lock.Lock()
	defer e.lock.Unlock()

	// Check if the metric already exists, if not create and register it
	gaugeVec, exists := e.metricsMap[name]
	if !exists {
		gaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: name,
		}, e.labelKeys)
		// Register the new metric with Prometheus
		prometheus.MustRegister(gaugeVec)
		// Store it in the metricsMap
		e.metricsMap[name] = gaugeVec
	}
	// Set the metric value for the metric
	gaugeVec.WithLabelValues(labelVals...).Set(value)
}

// StructToMetricsMap This recursively flattens the struct into a map where each field is represented by a string path and its corresponding value.
func StructToMetricsMap(v reflect.Value, path, tagPrefix string, metrics map[string]float64) {
	// Dereference pointers
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			fieldType := t.Field(i)
			if !field.CanInterface() {
				continue
			}
			// 指定从结构体字段的tagPrefix(通常使用"json")标签中提取标签的值作为字段名。如果没有标签，则使用字段的名称
			tag := fieldType.Tag.Get(tagPrefix)
			if tag == "-" {
				continue
			}
			fieldName := tag
			if fieldName == "" {
				fieldName = fieldType.Name
			}
			// Construct the new path for nested fields
			newPath := path
			if newPath != "" {
				newPath += "_"
			}
			newPath += fieldName
			// Recursively call StructToMetricsMap for nested fields
			StructToMetricsMap(field, newPath, tagPrefix, metrics)
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			strKey := fmt.Sprintf("%v", key.Interface())
			StructToMetricsMap(v.MapIndex(key), path+"_"+strKey, tagPrefix, metrics)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			StructToMetricsMap(v.Index(i), fmt.Sprintf("%s_%d", path, i), tagPrefix, metrics)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		metrics[path] = float64(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		metrics[path] = float64(v.Uint())
	case reflect.Float32, reflect.Float64:
		metrics[path] = v.Float()
	case reflect.Bool:
		if v.Bool() {
			metrics[path] = 1.0
		} else {
			metrics[path] = 0.0
		}
	case reflect.String:
		if v.String() == "enable" || v.String() == "true" {
			metrics[path] = 1.0
		} else {
			metrics[path] = 0.0
		}
	default:
		logrus.WithField("metrics", "common").Errorf("Unsupported type %v to reflect", v.Type())
	}
}

// sanitizeMetricName This function sanitizes metric names to ensure they are valid for Prometheus by replacing non-alphanumeric characters (except _).
func sanitizeMetricName(name string) string {
	name = strings.ToLower(name)
	// Replace non-alphanumeric characters with "_"
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, "+", "_")
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "[", "_")
	name = strings.ReplaceAll(name, "]", "")
	return name
}

func BoolToFloat64(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}
